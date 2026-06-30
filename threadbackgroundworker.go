package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/dunglas/frankenphp/internal/state"
)

// backgroundWorkerThread handles background worker scripts. Owns its own
// lifecycle: boot, loop, optionally crash-restart with exponential backoff.
// Background workers share the PHP runtime with HTTP threads but don't
// serve HTTP requests. They expose a stop pipe (via
// frankenphp_get_worker_handle) so PHP scripts can park on stream_select
// and exit gracefully when FrankenPHP drains them.
type backgroundWorkerThread struct {
	state                  *state.ThreadState
	thread                 *phpThread
	worker                 *worker
	dummyFrankenPHPContext *frankenPHPContext
	dummyContext           context.Context
	failureCount           int

	// runtimeName is the worker identity (kept m#-prefixed for metric
	// label consistency); setupScript trims it at the PHP boundary.
	runtimeName string
	// backgroundReady is this thread's readiness slot: worker.backgroundReady
	// for named workers, worker.catchAllNames[runtimeName] for catch-all
	// threads (so boot-failure / abort / markReady stay per-name).
	backgroundReady *backgroundWorkerState

	// stopFdWrite is this thread's stop-pipe write end (per-thread so pool
	// workers drain independently); read end is exposed to PHP via
	// frankenphp_get_worker_handle().
	stopFdWrite atomic.Int32
}

func convertToBackgroundWorkerThread(thread *phpThread, worker *worker, runtimeName string, ready *backgroundWorkerState) {
	handler := &backgroundWorkerThread{
		state:           thread.state,
		thread:          thread,
		worker:          worker,
		runtimeName:     runtimeName,
		backgroundReady: ready,
	}
	handler.stopFdWrite.Store(-1)
	thread.setHandler(handler)
	worker.attachThread(thread)
}

func (handler *backgroundWorkerThread) scopedWorker() *worker { return handler.worker }

func (handler *backgroundWorkerThread) name() string {
	if handler.runtimeName != "" && handler.runtimeName != handler.worker.name {
		return "Background Worker PHP Thread - " + handler.worker.fileName + " (" + handler.runtimeName + ")"
	}
	return "Background Worker PHP Thread - " + handler.worker.fileName
}

// isPostBoot samples the readiness channel non-blockingly so
// afterScriptExecution can tell a boot crash (record bootFailureInfo) from
// a post-boot crash (ensure() callers already saw success).
func (handler *backgroundWorkerThread) isPostBoot() bool {
	if handler.backgroundReady == nil {
		return false
	}
	select {
	case <-handler.backgroundReady.ready:
		return true
	default:
		return false
	}
}

func (handler *backgroundWorkerThread) frankenPHPContext() *frankenPHPContext {
	return handler.dummyFrankenPHPContext
}

func (handler *backgroundWorkerThread) context() context.Context {
	if handler.dummyContext != nil {
		return handler.dummyContext
	}
	return globalCtx
}

// drain is called by drainWorkerThreads (and thread.shutdown) right before
// drainChan is closed. We close the stop-pipe's write end so the PHP worker
// script, which is typically parked in stream_select on the read end, wakes
// up and can finish its loop gracefully. Per-thread fd so pool workers
// drain their threads independently.
func (handler *backgroundWorkerThread) drain() {
	if fd := handler.stopFdWrite.Swap(-1); fd >= 0 {
		C.frankenphp_worker_close_fd(C.int(fd))
	}
}

func (handler *backgroundWorkerThread) beforeScriptExecution() string {
	switch handler.state.Get() {
	case state.TransitionRequested:
		if handler.worker.onThreadShutdown != nil {
			handler.worker.onThreadShutdown(handler.thread.threadIndex)
		}
		handler.worker.detachThread(handler.thread)
		return handler.thread.transitionToNewHandler()
	case state.Restarting:
		if handler.worker.onThreadShutdown != nil {
			handler.worker.onThreadShutdown(handler.thread.threadIndex)
		}
		handler.state.Set(state.Yielding)
		handler.state.WaitFor(state.Ready, state.ShuttingDown)
		return handler.beforeScriptExecution()
	case state.Ready, state.TransitionComplete:
		handler.thread.updateContext(true)
		if handler.worker.onThreadReady != nil {
			handler.worker.onThreadReady(handler.thread.threadIndex)
		}

		handler.setupScript()

		return handler.worker.fileName
	case state.ShuttingDown:
		if handler.worker.onThreadShutdown != nil {
			handler.worker.onThreadShutdown(handler.thread.threadIndex)
		}
		handler.worker.detachThread(handler.thread)
		return ""
	}

	panic("unexpected state: " + handler.state.Name())
}

func (handler *backgroundWorkerThread) setupScript() {
	if handler.runtimeName == "" {
		handler.runtimeName = handler.worker.name
	}
	metrics.StartWorker(bgWorkerMetricName(handler.worker.scope, handler.runtimeName))

	opts := append([]RequestOption(nil), handler.worker.requestOptions...)
	handler.stopFdWrite.Store(int32(C.frankenphp_set_background_worker_and_get_stop_fd_write()))

	fc, err := newDummyContext(
		filepath.Base(handler.worker.fileName),
		opts...,
	)
	if err != nil {
		panic(err)
	}

	ctx := context.WithValue(globalCtx, contextKey, fc)

	fc.worker = handler.worker
	handler.dummyFrankenPHPContext = fc
	handler.dummyContext = ctx

	if globalLogger.Enabled(ctx, slog.LevelDebug) {
		globalLogger.LogAttrs(ctx, slog.LevelDebug, "starting background worker", slog.String("worker", handler.runtimeName), slog.Int("thread", handler.thread.threadIndex))
	}

	handler.thread.state.Set(state.Ready)
	fc.scriptFilename = handler.worker.fileName
}

func (handler *backgroundWorkerThread) afterScriptExecution(exitStatus int) {
	// frankenphp_worker_get_stop_fd_write transferred fd ownership to Go,
	// so we must close on every exit path to avoid a leak.
	if fd := handler.stopFdWrite.Swap(-1); fd >= 0 {
		C.frankenphp_worker_close_fd(C.int(fd))
	}
	worker := handler.worker
	runtimeName := handler.runtimeName
	if runtimeName == "" {
		runtimeName = worker.name
	}
	handler.dummyFrankenPHPContext = nil
	handler.dummyContext = nil

	// During Shutdown the drain's force-kill is armed against one slot; a
	// freshly spawned pthread would re-enter the script and never exit.
	if mainThread != nil && mainThread.state.Is(state.ShuttingDown) {
		handler.thread.state.Set(state.ShuttingDown)
		return
	}

	metricName := bgWorkerMetricName(worker.scope, runtimeName)

	// Cooperative exit: re-run, reset backoff.
	if exitStatus == 0 {
		metrics.StopWorker(metricName, StopReasonRestart)

		if globalLogger.Enabled(globalCtx, slog.LevelDebug) {
			globalLogger.LogAttrs(globalCtx, slog.LevelDebug, "restarting background worker", slog.String("worker", runtimeName), slog.Int("thread", handler.thread.threadIndex), slog.Int("exit_status", exitStatus))
		}

		handler.failureCount = 0
		return
	}

	metrics.StopWorker(metricName, StopReasonCrash)

	// Pre-readiness crash: stash metadata for a timing-out ensure(). Post-
	// readiness crashes don't update this (the worker already signalled OK).
	if !handler.isPostBoot() && handler.backgroundReady != nil {
		handler.backgroundReady.bootFailure.Store(&bootFailureInfo{
			entrypoint:   worker.fileName,
			exitStatus:   exitStatus,
			failureCount: handler.failureCount + 1,
			// TODO(sidekicks): capture PG(last_error_message) here once
			// the C-side helper lands in the set_vars/get_vars step.
		})
	}

	// max_consecutive_failures cap. For single-instance bg workers,
	// abort and release the slot so a future ensure() can lazy-spawn a
	// fresh thread. Eager pools skip this: other pool threads may still
	// be alive.
	if worker.maxConsecutiveFailures >= 0 && handler.failureCount >= worker.maxConsecutiveFailures {
		isSingleInstance := worker.bg.catchAllNames != nil || worker.num == 0
		if isSingleInstance && handler.backgroundReady != nil {
			handler.backgroundReady.abort(fmt.Errorf("background worker %s exceeded max_consecutive_failures (%d, last exit status %d)", worker.fileName, worker.maxConsecutiveFailures, exitStatus))
			worker.invalidateBackgroundEntry(runtimeName)
		}
		if startupFailChan != nil && !watcherIsEnabled {
			startupFailChan <- fmt.Errorf("too many consecutive failures: background worker %s keeps crashing", worker.fileName)
			handler.thread.state.Set(state.ShuttingDown)
			return
		}
		if globalLogger.Enabled(globalCtx, slog.LevelWarn) {
			globalLogger.LogAttrs(globalCtx, slog.LevelWarn, "background worker exceeded max_consecutive_failures, stopping respawn", slog.String("worker", runtimeName), slog.Int("thread", handler.thread.threadIndex), slog.Int("failures", handler.failureCount))
		}
		handler.thread.state.Set(state.ShuttingDown)
		return
	}

	if globalLogger.Enabled(globalCtx, slog.LevelWarn) {
		globalLogger.LogAttrs(globalCtx, slog.LevelWarn, "background worker crashed, restarting", slog.String("worker", runtimeName), slog.Int("thread", handler.thread.threadIndex), slog.Int("failures", handler.failureCount), slog.Int("exit_status", exitStatus))
	}

	backoffDuration := time.Duration(handler.failureCount*handler.failureCount*100) * time.Millisecond
	if backoffDuration > time.Second {
		backoffDuration = time.Second
	}
	handler.failureCount++
	time.Sleep(backoffDuration)
}
