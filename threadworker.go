package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"
	"unsafe"

	"github.com/dunglas/frankenphp/internal/state"
)

// representation of a thread assigned to a worker script
// executes the PHP worker script in a loop
// implements the threadHandler interface
type workerThread struct {
	state                   *state.ThreadState
	thread                  *phpThread
	worker                  *worker
	dummyFrankenPHPContext  *frankenPHPContext
	dummyContext            context.Context
	workerFrankenPHPContext *frankenPHPContext
	workerContext           context.Context
	isBootingScript         bool // true if the worker has not reached frankenphp_handle_request yet
	failureCount            int  // number of consecutive startup failures
}

func convertToWorkerThread(thread *phpThread, worker *worker) {
	thread.setHandler(&workerThread{
		state:  thread.state,
		thread: thread,
		worker: worker,
	})
	worker.attachThread(thread)
}

// beforeScriptExecution returns the name of the script or an empty string on shutdown
func (handler *workerThread) beforeScriptExecution() string {
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

		setupWorkerScript(handler, handler.worker)

		return handler.worker.fileName
	case state.ShuttingDown:
		if handler.worker.onThreadShutdown != nil {
			handler.worker.onThreadShutdown(handler.thread.threadIndex)
		}
		handler.worker.detachThread(handler.thread)

		// signal to stop
		return ""
	}

	panic("unexpected state: " + handler.state.Name())
}

func (handler *workerThread) afterScriptExecution(exitStatus int) {
	tearDownWorkerScript(handler, exitStatus)
}

func (handler *workerThread) frankenPHPContext() *frankenPHPContext {
	if handler.workerFrankenPHPContext != nil {
		return handler.workerFrankenPHPContext
	}

	return handler.dummyFrankenPHPContext
}
func (handler *workerThread) context() context.Context {
	if handler.workerContext != nil {
		return handler.workerContext
	}

	return handler.dummyContext
}

func (handler *workerThread) name() string {
	return "Worker PHP Thread - " + handler.worker.fileName
}

func setupWorkerScript(handler *workerThread, worker *worker) {
	metrics.StartWorker(worker.name)

	// Create a dummy request to set up the worker
	fc, err := newDummyContext(
		filepath.Base(worker.fileName),
		worker.requestOptions...,
	)
	if err != nil {
		panic(err)
	}

	ctx := context.WithValue(globalCtx, contextKey, fc)

	fc.worker = worker
	handler.dummyFrankenPHPContext = fc
	handler.dummyContext = ctx
	handler.isBootingScript = true
	clearSandboxedEnv(handler.thread)

	if globalLogger.Enabled(ctx, slog.LevelDebug) {
		globalLogger.LogAttrs(ctx, slog.LevelDebug, "starting", slog.String("worker", worker.name), slog.Int("thread", handler.thread.threadIndex))
	}
}

func tearDownWorkerScript(handler *workerThread, exitStatus int) {
	worker := handler.worker
	handler.dummyFrankenPHPContext = nil
	handler.dummyContext = nil

	// if the worker request is not nil, the script might have crashed
	// make sure to close the worker request context
	if handler.workerFrankenPHPContext != nil {
		handler.workerFrankenPHPContext.closeContext()
		handler.workerFrankenPHPContext = nil
		handler.workerContext = nil
	}

	// on exit status 0 we just run the worker script again
	if exitStatus == 0 && !handler.isBootingScript {
		metrics.StopWorker(worker.name, StopReasonRestart)

		if globalLogger.Enabled(globalCtx, slog.LevelDebug) {
			globalLogger.LogAttrs(globalCtx, slog.LevelDebug, "restarting", slog.String("worker", worker.name), slog.Int("thread", handler.thread.threadIndex), slog.Int("exit_status", exitStatus))
		}

		return
	}

	// worker has thrown a fatal error or has not reached frankenphp_handle_request
	if handler.isBootingScript {
		metrics.StopWorker(worker.name, StopReasonBootFailure)
	} else {
		metrics.StopWorker(worker.name, StopReasonCrash)
	}

	if !handler.isBootingScript {
		// fatal error (could be due to exit(1), timeouts, etc.)
		if globalLogger.Enabled(globalCtx, slog.LevelDebug) {
			globalLogger.LogAttrs(globalCtx, slog.LevelDebug, "restarting", slog.String("worker", worker.name), slog.Int("thread", handler.thread.threadIndex), slog.Int("exit_status", exitStatus))
		}

		return
	}

	if worker.maxConsecutiveFailures >= 0 && startupFailChan != nil && !watcherIsEnabled && handler.failureCount >= worker.maxConsecutiveFailures {
		startupFailChan <- fmt.Errorf("too many consecutive failures: worker %s has not reached frankenphp_handle_request()", worker.fileName)
		handler.thread.state.Set(state.ShuttingDown)
		return
	}

	if watcherIsEnabled {
		// worker script has probably failed due to script changes while watcher is enabled
		if globalLogger.Enabled(globalCtx, slog.LevelError) {
			globalLogger.LogAttrs(globalCtx, slog.LevelWarn, "(watcher enabled) worker script has not reached frankenphp_handle_request()", slog.String("worker", worker.name), slog.Int("thread", handler.thread.threadIndex))
		}
	} else {
		// rare case where worker script has failed on a restart during normal operation
		// this can happen if startup success depends on external resources
		if globalLogger.Enabled(globalCtx, slog.LevelWarn) {
			globalLogger.LogAttrs(globalCtx, slog.LevelWarn, "worker script has failed on restart", slog.String("worker", worker.name), slog.Int("thread", handler.thread.threadIndex), slog.Int("failures", handler.failureCount))
		}
	}

	// wait a bit and try again (exponential backoff)
	backoffDuration := time.Duration(handler.failureCount*handler.failureCount*100) * time.Millisecond
	if backoffDuration > time.Second {
		backoffDuration = time.Second
	}
	handler.failureCount++
	time.Sleep(backoffDuration)
}

// waitForWorkerRequest is called during frankenphp_handle_request in the php worker script.
func (handler *workerThread) waitForWorkerRequest() (bool, any) {
	// unpin any memory left over from previous requests
	handler.thread.Unpin()

	if globalLogger.Enabled(globalCtx, slog.LevelDebug) {
		globalLogger.LogAttrs(globalCtx, slog.LevelDebug, "waiting for request", slog.String("worker", handler.worker.name), slog.Int("thread", handler.thread.threadIndex))
	}

	// Clear the first dummy request created to initialize the worker
	if handler.isBootingScript {
		handler.isBootingScript = false
		handler.failureCount = 0
		if !C.frankenphp_shutdown_dummy_request() {
			panic("Not in CGI context")
		}

		// worker is truly ready only after reaching frankenphp_handle_request()
		metrics.ReadyWorker(handler.worker.name)
	}

	if handler.state.Is(state.TransitionComplete) {
		handler.state.Set(state.Ready)
	}

	handler.state.MarkAsWaiting(true)

	var requestCH contextHolder
	select {
	case <-handler.thread.drainChan:
		if globalLogger.Enabled(globalCtx, slog.LevelDebug) {
			globalLogger.LogAttrs(globalCtx, slog.LevelDebug, "shutting down", slog.String("worker", handler.worker.name), slog.Int("thread", handler.thread.threadIndex))
		}

		// flush the opcache when restarting due to watcher or admin api
		// note: this is done right before frankenphp_handle_request() returns 'false'
		if handler.state.Is(state.Restarting) {
			C.frankenphp_reset_opcache()
		}

		return false, nil
	case requestCH = <-handler.thread.requestChan:
	case requestCH = <-handler.worker.requestChan:
	}

	handler.workerContext = requestCH.ctx
	handler.workerFrankenPHPContext = requestCH.frankenPHPContext
	handler.state.MarkAsWaiting(false)

	if globalLogger.Enabled(requestCH.ctx, slog.LevelDebug) {
		if handler.workerFrankenPHPContext.request == nil {
			globalLogger.LogAttrs(requestCH.ctx, slog.LevelDebug, "request handling started", slog.String("worker", handler.worker.name), slog.Int("thread", handler.thread.threadIndex))
		} else {
			globalLogger.LogAttrs(requestCH.ctx, slog.LevelDebug, "request handling started", slog.String("worker", handler.worker.name), slog.Int("thread", handler.thread.threadIndex), slog.String("url", handler.workerFrankenPHPContext.request.RequestURI))
		}
	}

	return true, handler.workerFrankenPHPContext.handlerParameters
}

// go_frankenphp_worker_handle_request_start is called at the start of every php request served.
//
//export go_frankenphp_worker_handle_request_start
func go_frankenphp_worker_handle_request_start(threadIndex C.uintptr_t) (C.bool, unsafe.Pointer) {
	handler := phpThreads[threadIndex].handler.(*workerThread)
	hasRequest, parameters := handler.waitForWorkerRequest()

	if parameters != nil {
		var ptr unsafe.Pointer

		switch p := parameters.(type) {
		case unsafe.Pointer:
			ptr = p

		default:
			ptr = PHPValue(p)
		}
		handler.thread.Pin(ptr)

		return C.bool(hasRequest), ptr
	}

	return C.bool(hasRequest), nil
}

// go_frankenphp_finish_worker_request is called at the end of every php request served.
//
//export go_frankenphp_finish_worker_request
func go_frankenphp_finish_worker_request(threadIndex C.uintptr_t, retval *C.zval) {
	thread := phpThreads[threadIndex]
	ctx := thread.context()
	fc := ctx.Value(contextKey).(*frankenPHPContext)

	if retval != nil {
		r, err := GoValue[any](unsafe.Pointer(retval))
		if err != nil && globalLogger.Enabled(ctx, slog.LevelError) {
			globalLogger.LogAttrs(ctx, slog.LevelError, "cannot convert return value", slog.Any("error", err), slog.Int("thread", thread.threadIndex))
		}

		fc.handlerReturn = r
	}

	fc.closeContext()
	thread.handler.(*workerThread).workerFrankenPHPContext = nil
	thread.handler.(*workerThread).workerContext = nil

	if globalLogger.Enabled(ctx, slog.LevelDebug) {
		if fc.request == nil {
			fc.logger.LogAttrs(ctx, slog.LevelDebug, "request handling finished", slog.String("worker", fc.worker.name), slog.Int("thread", thread.threadIndex))
		} else {
			fc.logger.LogAttrs(ctx, slog.LevelDebug, "request handling finished", slog.String("worker", fc.worker.name), slog.Int("thread", thread.threadIndex), slog.String("url", fc.request.RequestURI))
		}
	}
}

// when frankenphp_finish_request() is directly called from PHP
//
//export go_frankenphp_finish_php_request
func go_frankenphp_finish_php_request(threadIndex C.uintptr_t) {
	thread := phpThreads[threadIndex]
	fc := thread.frankenPHPContext()

	fc.closeContext()

	ctx := thread.context()
	if fc.logger.Enabled(ctx, slog.LevelDebug) {
		fc.logger.LogAttrs(ctx, slog.LevelDebug, "request handling finished", slog.Int("thread", thread.threadIndex), slog.String("url", fc.request.RequestURI))
	}
}
