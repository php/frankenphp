package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/dunglas/frankenphp/internal/state"
)

// backgroundWorkerState carries a bg-worker instance's shared vars and
// combined readiness signal. ready closes only when BOTH markHandle (from
// frankenphp_get_worker_handle) and markVars (from frankenphp_set_vars)
// have fired. mu serialises writer/reader access to varsPtr so the C side
// can pfree the old pointer safely after set_vars returns.
type backgroundWorkerState struct {
	varsPtr     unsafe.Pointer // *C.HashTable in persistent memory
	mu          sync.RWMutex
	varsVersion atomic.Uint64

	ready      chan struct{}
	closeReady sync.Once
	hasHandle  atomic.Bool
	hasVars    atomic.Bool

	// aborted unblocks ensure() waiters when the start sequence is
	// abandoned before reaching ready.
	aborted   chan struct{}
	abortOnce sync.Once
	abortErr  error

	// bootFailure carries a pre-readiness crash so a timing-out ensure()
	// can surface it.
	bootFailure atomic.Pointer[bootFailureInfo]
}

func newBackgroundWorkerState() *backgroundWorkerState {
	return &backgroundWorkerState{
		ready:   make(chan struct{}),
		aborted: make(chan struct{}),
	}
}

// markHandle / markVars fire on the first frankenphp_get_worker_handle()
// and frankenphp_set_vars() calls respectively. ready closes once both
// have fired. Both are idempotent.
func (sk *backgroundWorkerState) markHandle() {
	sk.hasHandle.Store(true)
	sk.tryCloseReady()
}

func (sk *backgroundWorkerState) markVars() {
	sk.hasVars.Store(true)
	sk.tryCloseReady()
}

func (sk *backgroundWorkerState) tryCloseReady() {
	if sk.hasHandle.Load() && sk.hasVars.Load() {
		sk.closeReady.Do(func() { close(sk.ready) })
	}
}

// abort wakes ensure() waiters when the start sequence is abandoned.
// Idempotent.
func (sk *backgroundWorkerState) abort(err error) {
	sk.abortOnce.Do(func() {
		sk.abortErr = err
		close(sk.aborted)
	})
}

// backgroundWorkerThread handles background worker scripts. Owns its own
// lifecycle: boot, publish vars, loop, optionally crash-restart with
// exponential backoff.
type backgroundWorkerThread struct {
	state                  *state.ThreadState
	thread                 *phpThread
	worker                 *worker
	dummyFrankenPHPContext *frankenPHPContext
	dummyContext           context.Context
	isBootingScript        bool
	failureCount           int

	// runtimeName is the worker identity (kept m#-prefixed for metric
	// label consistency); setupScript trims it at the PHP boundary.
	runtimeName string
	// backgroundWorker is this thread's state slot (combined readiness +
	// shared vars): worker.bg.ready for named workers,
	// worker.bg.catchAllNames[runtimeName] for catch-all threads (so
	// boot-failure / abort / markHandle / markVars stay per-name).
	backgroundWorker *backgroundWorkerState

	// stopFdWrite is this thread's stop-pipe write end (per-thread so pool
	// workers drain independently); read end is exposed to PHP via
	// frankenphp_get_worker_handle().
	stopFdWrite atomic.Int32
}

func convertToBackgroundWorkerThread(thread *phpThread, worker *worker, runtimeName string, sk *backgroundWorkerState) {
	handler := &backgroundWorkerThread{
		state:            thread.state,
		thread:           thread,
		worker:           worker,
		runtimeName:      runtimeName,
		backgroundWorker: sk,
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

// isPostBoot samples the combined readiness channel (handle + set_vars)
// non-blockingly so afterScriptExecution can tell a boot crash from a
// post-boot crash.
func (handler *backgroundWorkerThread) isPostBoot() bool {
	if handler.backgroundWorker == nil {
		return false
	}
	select {
	case <-handler.backgroundWorker.ready:
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
	if handler.backgroundWorker == nil {
		handler.backgroundWorker = handler.worker.bg.ready
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
	handler.isBootingScript = true

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

	// Cooperative exit: re-run.
	if exitStatus == 0 && !handler.isBootingScript {
		metrics.StopWorker(metricName, StopReasonRestart)

		if globalLogger.Enabled(globalCtx, slog.LevelDebug) {
			globalLogger.LogAttrs(globalCtx, slog.LevelDebug, "restarting background worker", slog.String("worker", runtimeName), slog.Int("thread", handler.thread.threadIndex), slog.Int("exit_status", exitStatus))
		}

		return
	}

	if handler.isBootingScript {
		metrics.StopWorker(metricName, StopReasonBootFailure)
	} else {
		metrics.StopWorker(metricName, StopReasonCrash)
	}

	// Pre-readiness crash: capture metadata (including PG(last_error_*)
	// grabbed C-side before php_request_shutdown clears it) for a timing-
	// out ensure(). Post-readiness crashes don't update this (the worker
	// already signalled OK).
	if handler.isBootingScript && !handler.isPostBoot() && handler.backgroundWorker != nil {
		var phpError string
		if cErr := C.frankenphp_get_last_php_error(); cErr != nil {
			phpError = C.GoString(cErr)
			C.free(unsafe.Pointer(cErr))
		}
		handler.backgroundWorker.bootFailure.Store(&bootFailureInfo{
			entrypoint:   worker.fileName,
			exitStatus:   exitStatus,
			failureCount: handler.failureCount + 1,
			phpError:     phpError,
		})
	}

	// max_consecutive_failures cap. For single-instance bg workers,
	// abort and release the slot so a future ensure() can lazy-spawn a
	// fresh thread. Eager pools skip this: other pool threads may still
	// be alive.
	if worker.maxConsecutiveFailures >= 0 && handler.failureCount >= worker.maxConsecutiveFailures {
		isSingleInstance := worker.bg.catchAllNames != nil || worker.num == 0
		if isSingleInstance && handler.backgroundWorker != nil {
			handler.backgroundWorker.abort(fmt.Errorf("background worker %s exceeded max_consecutive_failures (%d, last exit status %d)", worker.fileName, worker.maxConsecutiveFailures, exitStatus))
			worker.invalidateBackgroundEntry(runtimeName)
		}
		if startupFailChan != nil && !watcherIsEnabled {
			startupFailChan <- fmt.Errorf("too many consecutive failures: background worker %s has not reached frankenphp_set_vars()", worker.fileName)
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
		msg := "background worker crashed, restarting"
		if handler.isBootingScript {
			msg = "background worker boot failed, restarting"
		}
		globalLogger.LogAttrs(globalCtx, slog.LevelWarn, msg, slog.String("worker", runtimeName), slog.Int("thread", handler.thread.threadIndex), slog.Int("failures", handler.failureCount), slog.Int("exit_status", exitStatus))
	}

	backoffDuration := time.Duration(handler.failureCount*handler.failureCount*100) * time.Millisecond
	if backoffDuration > time.Second {
		backoffDuration = time.Second
	}
	handler.failureCount++
	time.Sleep(backoffDuration)
}

// markBackgroundReady fires on the first set_vars after each (re)boot:
// clears the boot-failure record and marks the vars half of the combined
// readiness signal. failureCount is intentionally NOT reset here — a
// worker that reaches readiness and then crashes must still trip the
// max_consecutive_failures cap; only cooperative exit (exit 0) zeroes
// the counter. Idempotent within a boot.
func (handler *backgroundWorkerThread) markBackgroundReady() {
	if !handler.isBootingScript {
		return
	}

	handler.isBootingScript = false
	if handler.backgroundWorker != nil {
		handler.backgroundWorker.bootFailure.Store(nil)
		handler.backgroundWorker.markVars()
	}

	metrics.ReadyWorker(bgWorkerMetricName(handler.worker.scope, handler.runtimeName))
}
