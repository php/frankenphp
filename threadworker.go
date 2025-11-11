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
	state           *state.ThreadState
	thread          *phpThread
	worker          *worker
	dummyContext    *frankenPHPContext
	workerContext   *frankenPHPContext
	isBootingScript bool // true if the worker has not reached frankenphp_handle_request yet
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
	metrics.StartWorker(worker.name)

	if handler.state.Is(state.Ready) {
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

	if !watcherIsEnabled && !handler.state.Is(state.Ready) {
		select {
		case startupFailChan <- fmt.Errorf("worker failure: script %s has not reached frankenphp_handle_request()", worker.fileName):
			handler.thread.state.Set(state.ShuttingDown)
			return
		}
	}

	if !watcherIsEnabled {
		// rare case where worker script has failed on a restart during normal operation
		// this can happen if startup success depends on external resources
		logger.LogAttrs(ctx, slog.LevelError, "worker script has failed on restart", slog.String("worker", worker.name), slog.Int("thread", handler.thread.threadIndex))
	} else {
		// worker script has probably failed due to script changes while watcher is enabled
		logger.LogAttrs(ctx, slog.LevelWarn, "(watcher enabled) worker script has not reached frankenphp_handle_request()", slog.String("worker", worker.name), slog.Int("thread", handler.thread.threadIndex))
	}

	// wait a bit and try again
	time.Sleep(time.Millisecond * 250)
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
	// 'state.TransitionComplete' is only true on the first boot of the worker script,
	// while 'isBootingScript' is true on every boot of the worker script
	if handler.state.Is(state.TransitionComplete) {
		metrics.ReadyWorker(handler.worker.name)
		handler.state.Set(state.Ready)
	}

	handler.state.MarkAsWaiting(true)

	var fc *frankenPHPContext
	select {
	case <-handler.thread.drainChan:
		logger.LogAttrs(ctx, slog.LevelDebug, "shutting down", slog.String("worker", handler.worker.name), slog.Int("thread", handler.thread.threadIndex))

		// flush the opcache when restarting due to watcher or admin api
		// note: this is done right before frankenphp_handle_request() returns 'false'
		if handler.state.Is(state.Restarting) {
			C.frankenphp_reset_opcache()
		}

		return false, nil
	case fc = <-handler.thread.requestChan:
	case fc = <-handler.worker.requestChan:
	}

	handler.workerContext = fc
	handler.state.MarkAsWaiting(false)

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
