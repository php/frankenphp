package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/dunglas/frankenphp/internal/state"
)

// backgroundWorkerState holds the shared state for a single background worker.
// Accessed by the background worker thread (writes) and HTTP worker threads (reads).
type backgroundWorkerState struct {
	varsPtr     unsafe.Pointer // *C.HashTable, persistent, managed by C
	mu          sync.RWMutex
	varsVersion atomic.Uint64 // incremented on each set_vars call
	ready       chan struct{}
	readyOnce   sync.Once
}

// backgroundWorkerThread handles background worker scripts.
// Decoupled from workerThread; owns its own lifecycle state.
type backgroundWorkerThread struct {
	state                 *state.ThreadState
	thread                *phpThread
	worker                *worker
	dummyFrankenPHPContext *frankenPHPContext
	dummyContext          context.Context
	isBootingScript       bool
	failureCount          int
}

func convertToBackgroundWorkerThread(thread *phpThread, worker *worker) {
	handler := &backgroundWorkerThread{
		state:  thread.state,
		thread: thread,
		worker: worker,
	}
	thread.setHandler(handler)
	worker.attachThread(thread)
}

func (handler *backgroundWorkerThread) name() string {
	return "Background Worker PHP Thread - " + handler.worker.fileName
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

func (handler *backgroundWorkerThread) drain() {
	if fd := handler.worker.backgroundStopFdWrite.Load(); fd >= 0 {
		C.frankenphp_worker_write_stop_fd(C.int(fd))
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
	// Ensure backgroundWorker state is reserved (for auto-started workers).
	// Uses sync.Once to handle pool workers (num > 1) safely.
	handler.worker.backgroundReserveOnce.Do(func() {
		if handler.worker.backgroundWorker == nil && handler.worker.backgroundRegistry != nil {
			bgw, _, err := handler.worker.backgroundRegistry.reserve(strings.TrimPrefix(handler.worker.name, "m#"))
			if err == nil {
				handler.worker.backgroundWorker = bgw
			}
		}
	})

	metrics.StartWorker(handler.worker.name)

	opts := append([]RequestOption(nil), handler.worker.requestOptions...)
	C.frankenphp_set_worker_name(handler.thread.pinCString(strings.TrimPrefix(handler.worker.name, "m#")), C._Bool(true))
	handler.worker.backgroundStopFdWrite.Store(int32(C.frankenphp_worker_get_stop_fd_write()))

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
		globalLogger.LogAttrs(ctx, slog.LevelDebug, "starting", slog.String("worker", handler.worker.name), slog.Int("thread", handler.thread.threadIndex))
	}

	handler.thread.state.Set(state.Ready)
	fc.scriptFilename = handler.worker.fileName
}

func (handler *backgroundWorkerThread) afterScriptExecution(exitStatus int) {
	handler.worker.backgroundStopFdWrite.Store(-1)
	worker := handler.worker
	handler.dummyFrankenPHPContext = nil
	handler.dummyContext = nil

	// on exit status 0 we just run the worker script again
	if exitStatus == 0 && !handler.isBootingScript {
		metrics.StopWorker(worker.name, StopReasonRestart)

		if globalLogger.Enabled(globalCtx, slog.LevelDebug) {
			globalLogger.LogAttrs(globalCtx, slog.LevelDebug, "restarting", slog.String("worker", worker.name), slog.Int("thread", handler.thread.threadIndex), slog.Int("exit_status", exitStatus))
		}

		return
	}

	// worker has thrown a fatal error or has not reached set_vars
	if handler.isBootingScript {
		metrics.StopWorker(worker.name, StopReasonBootFailure)
	} else {
		metrics.StopWorker(worker.name, StopReasonCrash)
	}

	if !handler.isBootingScript {
		// Background worker crashed after it was running - operationally significant
		if globalLogger.Enabled(globalCtx, slog.LevelWarn) {
			globalLogger.LogAttrs(globalCtx, slog.LevelWarn, "background worker exited, restarting", slog.String("worker", worker.name), slog.Int("thread", handler.thread.threadIndex), slog.Int("exit_status", exitStatus))
		}

		return
	}

	if worker.maxConsecutiveFailures >= 0 && startupFailChan != nil && !watcherIsEnabled && handler.failureCount >= worker.maxConsecutiveFailures {
		startupFailChan <- fmt.Errorf("too many consecutive failures: worker %s has not reached frankenphp_handle_request()", worker.fileName)
		handler.thread.state.Set(state.ShuttingDown)
		return
	}

	if watcherIsEnabled {
		if globalLogger.Enabled(globalCtx, slog.LevelWarn) {
			globalLogger.LogAttrs(globalCtx, slog.LevelWarn, "(watcher enabled) worker script has not reached frankenphp_handle_request()", slog.String("worker", worker.name), slog.Int("thread", handler.thread.threadIndex))
		}
	} else {
		if globalLogger.Enabled(globalCtx, slog.LevelWarn) {
			globalLogger.LogAttrs(globalCtx, slog.LevelWarn, "worker script has failed on restart", slog.String("worker", worker.name), slog.Int("thread", handler.thread.threadIndex), slog.Int("failures", handler.failureCount))
		}
	}

	backoffDuration := time.Duration(handler.failureCount*handler.failureCount*100) * time.Millisecond
	if backoffDuration > time.Second {
		backoffDuration = time.Second
	}
	handler.failureCount++
	time.Sleep(backoffDuration)
}

// markBackgroundReady resets failure state when the background worker
// successfully calls set_vars for the first time.
func (handler *backgroundWorkerThread) markBackgroundReady() {
	if !handler.isBootingScript {
		return
	}

	handler.failureCount = 0
	handler.isBootingScript = false
	metrics.ReadyWorker(handler.worker.name)
}
