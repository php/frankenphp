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
)

// representation of a thread assigned to a worker script
// executes the PHP worker script in a loop
// implements the threadHandler interface
type workerThread struct {
	state           *threadState
	thread          *phpThread
	worker          *worker
	dummyContext    *frankenPHPContext
	workerContext   *frankenPHPContext
	backoff         *exponentialBackoff
	isBootingScript bool // true if the worker has not reached frankenphp_handle_request yet
}

func convertToWorkerThread(thread *phpThread, worker *worker) {
	thread.setHandler(&workerThread{
		state:  thread.state,
		thread: thread,
		worker: worker,
		backoff: &exponentialBackoff{
			maxBackoff:             1 * time.Second,
			minBackoff:             100 * time.Millisecond,
			maxConsecutiveFailures: worker.maxConsecutiveFailures,
		},
	})
	worker.attachThread(thread)
}

// beforeScriptExecution returns the name of the script or an empty string on shutdown
func (handler *workerThread) beforeScriptExecution() string {
	switch handler.state.get() {
	case stateTransitionRequested:
		if handler.worker.onThreadShutdown != nil {
			handler.worker.onThreadShutdown(handler.thread.threadIndex)
		}
		handler.worker.detachThread(handler.thread)
		return handler.thread.transitionToNewHandler()
	case stateRestarting:
		if handler.worker.onThreadShutdown != nil {
			handler.worker.onThreadShutdown(handler.thread.threadIndex)
		}
		handler.state.set(stateYielding)
		handler.state.waitFor(stateReady, stateShuttingDown)
		return handler.beforeScriptExecution()
	case stateReady, stateTransitionComplete:
		if handler.worker.onThreadReady != nil {
			handler.worker.onThreadReady(handler.thread.threadIndex)
		}
		setupWorkerScript(handler, handler.worker)
		return handler.worker.fileName
	case stateShuttingDown:
		if handler.worker.onThreadShutdown != nil {
			handler.worker.onThreadShutdown(handler.thread.threadIndex)
		}
		handler.worker.detachThread(handler.thread)
		// signal to stop
		return ""
	}
	panic("unexpected state: " + handler.state.name())
}

func (handler *workerThread) afterScriptExecution(exitStatus int) {
	tearDownWorkerScript(handler, exitStatus)
}

func (handler *workerThread) getRequestContext() *frankenPHPContext {
	if handler.workerContext != nil {
		return handler.workerContext
	}

	return handler.dummyContext
}

func (handler *workerThread) name() string {
	return "Worker PHP Thread - " + handler.worker.fileName
}

func setupWorkerScript(handler *workerThread, worker *worker) {
	handler.backoff.wait()
	metrics.StartWorker(worker.name)

	if handler.state.is(stateReady) {
		metrics.ReadyWorker(handler.worker.name)
	}

	// Create a dummy request to set up the worker
	fc, err := newDummyContext(
		filepath.Base(worker.fileName),
		WithRequestDocumentRoot(filepath.Dir(worker.fileName), false),
		WithRequestPreparedEnv(worker.env),
	)
	if err != nil {
		panic(err)
	}

	fc.worker = worker
	handler.dummyContext = fc
	handler.isBootingScript = true
	clearSandboxedEnv(handler.thread)
	logger.LogAttrs(context.Background(), slog.LevelDebug, "starting", slog.String("worker", worker.name), slog.Int("thread", handler.thread.threadIndex))
}

func tearDownWorkerScript(handler *workerThread, exitStatus int) {
	worker := handler.worker
	handler.dummyContext = nil

	ctx := context.Background()

	// if the worker request is not nil, the script might have crashed
	// make sure to close the worker request context
	if handler.workerContext != nil {
		handler.workerContext.closeContext()
		handler.workerContext = nil
	}

	// on exit status 0 we just run the worker script again
	if exitStatus == 0 && !handler.isBootingScript {
		metrics.StopWorker(worker.name, StopReasonRestart)
		handler.backoff.recordSuccess()
		logger.LogAttrs(ctx, slog.LevelDebug, "restarting", slog.String("worker", worker.name), slog.Int("thread", handler.thread.threadIndex), slog.Int("exit_status", exitStatus))

		return
	}

	// worker has thrown a fatal error or has not reached frankenphp_handle_request
	metrics.StopWorker(worker.name, StopReasonCrash)

	if !handler.isBootingScript {
		// fatal error (could be due to exit(1), timeouts, etc.)
		logger.LogAttrs(ctx, slog.LevelDebug, "restarting", slog.String("worker", worker.name), slog.Int("thread", handler.thread.threadIndex), slog.Int("exit_status", exitStatus))

		return
	}

	logger.LogAttrs(ctx, slog.LevelError, "worker script has not reached frankenphp_handle_request()", slog.String("worker", worker.name), slog.Int("thread", handler.thread.threadIndex))

	// panic after exponential backoff if the worker has never reached frankenphp_handle_request
	if handler.backoff.recordFailure() {
		if !watcherIsEnabled && !handler.state.is(stateReady) {
			logger.LogAttrs(ctx, slog.LevelError, "too many consecutive worker failures", slog.String("worker", worker.name), slog.Int("thread", handler.thread.threadIndex), slog.Int("failures", handler.backoff.failureCount))
			panic("too many consecutive worker failures")
		}
		logger.LogAttrs(ctx, slog.LevelWarn, "many consecutive worker failures", slog.String("worker", worker.name), slog.Int("thread", handler.thread.threadIndex), slog.Int("failures", handler.backoff.failureCount))
	}
}

// waitForWorkerRequest is called during frankenphp_handle_request in the php worker script.
func (handler *workerThread) waitForWorkerRequest() (bool, any) {
	// unpin any memory left over from previous requests
	handler.thread.Unpin()

	ctx := context.Background()
	logger.LogAttrs(ctx, slog.LevelDebug, "waiting for request", slog.String("worker", handler.worker.name), slog.Int("thread", handler.thread.threadIndex))

	// Clear the first dummy request created to initialize the worker
	if handler.isBootingScript {
		handler.isBootingScript = false
		if !C.frankenphp_shutdown_dummy_request() {
			panic("Not in CGI context")
		}
	}

	// worker threads are 'ready' after they first reach frankenphp_handle_request()
	// 'stateTransitionComplete' is only true on the first boot of the worker script,
	// while 'isBootingScript' is true on every boot of the worker script
	if handler.state.is(stateTransitionComplete) {
		metrics.ReadyWorker(handler.worker.name)
		handler.state.set(stateReady)
	}

	handler.state.markAsWaiting(true)

	var fc *frankenPHPContext
	select {
	case <-handler.thread.drainChan:
		logger.LogAttrs(ctx, slog.LevelDebug, "shutting down", slog.String("worker", handler.worker.name), slog.Int("thread", handler.thread.threadIndex))

		// flush the opcache when restarting due to watcher or admin api
		// note: this is done right before frankenphp_handle_request() returns 'false'
		if handler.state.is(stateRestarting) {
			C.frankenphp_reset_opcache()
		}

		return false, nil
	case fc = <-handler.thread.requestChan:
	case fc = <-handler.worker.requestChan:
	}

	handler.workerContext = fc
	handler.state.markAsWaiting(false)

	if fc.request == nil {
		logger.LogAttrs(ctx, slog.LevelDebug, "request handling started", slog.String("worker", handler.worker.name), slog.Int("thread", handler.thread.threadIndex))
	} else {
		logger.LogAttrs(ctx, slog.LevelDebug, "request handling started", slog.String("worker", handler.worker.name), slog.Int("thread", handler.thread.threadIndex), slog.String("url", fc.request.RequestURI))
	}

	return true, fc.handlerParameters
}

// go_frankenphp_worker_handle_request_start is called at the start of every php request served.
//
//export go_frankenphp_worker_handle_request_start
func go_frankenphp_worker_handle_request_start(threadIndex C.uintptr_t) (C.bool, *C.zval) {
	handler := phpThreads[threadIndex].handler.(*workerThread)
	hasRequest, parameters := handler.waitForWorkerRequest()

	if parameters != nil {
		switch p := parameters.(type) {
		case unsafe.Pointer:
			return C.bool(hasRequest), (*C.zval)(p)

		default:
			var zval C.zval
			phpValue(&zval, p)
			handler.thread.Pin(unsafe.Pointer(&zval))
			return C.bool(hasRequest), &zval
		}
	}

	return C.bool(hasRequest), nil
}

// go_frankenphp_finish_worker_request is called at the end of every php request served.
//
//export go_frankenphp_finish_worker_request
func go_frankenphp_finish_worker_request(threadIndex C.uintptr_t, retval *C.zval) {
	thread := phpThreads[threadIndex]
	fc := thread.getRequestContext()
	if retval != nil {
		r, err := GoValue[any](unsafe.Pointer(retval))
		if err != nil {
			logger.Error(fmt.Sprintf("cannot convert return value: %s", err))
		}

		fc.handlerReturn = r
	}

	fc.closeContext()
	thread.handler.(*workerThread).workerContext = nil

	if fc.request == nil {
		fc.logger.LogAttrs(context.Background(), slog.LevelDebug, "request handling finished", slog.String("worker", fc.worker.name), slog.Int("thread", thread.threadIndex))
	} else {
		fc.logger.LogAttrs(context.Background(), slog.LevelDebug, "request handling finished", slog.String("worker", fc.worker.name), slog.Int("thread", thread.threadIndex), slog.String("url", fc.request.RequestURI))
	}
}

// when frankenphp_finish_request() is directly called from PHP
//
//export go_frankenphp_finish_php_request
func go_frankenphp_finish_php_request(threadIndex C.uintptr_t) {
	thread := phpThreads[threadIndex]
	fc := thread.getRequestContext()

	fc.closeContext()

	fc.logger.LogAttrs(context.Background(), slog.LevelDebug, "request handling finished", slog.Int("thread", thread.threadIndex), slog.String("url", fc.request.RequestURI))
}
