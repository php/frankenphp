package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"fmt"
	"log/slog"
	"time"
	"unsafe"

	"github.com/dunglas/frankenphp/internal/state"
)

// representation of a thread assigned to a worker script
// executes the PHP worker script in a loop
// implements the threadHandler interface
type workerThread struct {
	workerLifecycle

	dummyFrankenPHPContext  *frankenPHPContext
	workerFrankenPHPContext *frankenPHPContext
	isBootingScript         bool // true if the worker has not reached frankenphp_handle_request yet
	failureCount            int  // number of consecutive startup failures
	requestCount            int  // number of requests handled since last restart
}

func convertToWorkerThread(thread *phpThread, worker *worker) {
	thread.setHandler(&workerThread{workerLifecycle: newWorkerLifecycle(thread, worker)})
	worker.attachThread(thread)
}

func (handler *workerThread) beforeScriptExecution() string {
	return handler.workerLifecycle.beforeScriptExecution(handler)
}

// startScript runs the worker script; it always has one to run
func (handler *workerThread) startScript() string {
	setupWorkerScript(handler, handler.worker)

	return handler.worker.fileName
}

func (handler *workerThread) resetForReboot() {
	handler.requestCount = 0
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

func (handler *workerThread) name() string {
	return "Worker PHP Thread - " + handler.worker.fileName
}

func (handler *workerThread) drain() {}

func setupWorkerScript(handler *workerThread, worker *worker) {
	metrics.StartWorker(worker.qualifiedName)

	// Create a dummy request to set up the worker
	fc, err := newWorkerDummyContext(worker)
	if err != nil {
		panic(err)
	}

	handler.dummyFrankenPHPContext = fc
	handler.isBootingScript = true
	handler.requestCount = 0

	if fc.logger.Enabled(fc.ctx, slog.LevelDebug) {
		fc.logger.LogAttrs(fc.ctx, slog.LevelDebug, "starting", slog.String("worker", worker.qualifiedName), slog.Int("thread", handler.thread.threadIndex))
	}
}

func tearDownWorkerScript(handler *workerThread, exitStatus int) {
	worker := handler.worker
	handler.dummyFrankenPHPContext = nil

	// if the worker request is not nil, the script might have crashed
	// make sure to close the worker request context
	if handler.workerFrankenPHPContext != nil {
		handler.workerFrankenPHPContext.closeContext()
		handler.thread.contextMu.Lock()
		handler.workerFrankenPHPContext = nil
		handler.thread.contextMu.Unlock()
	}

	// on exit status 0 we just run the worker script again
	if exitStatus == 0 && !handler.isBootingScript {
		metrics.StopWorker(worker.qualifiedName, StopReasonRestart)

		if globalLogger.Enabled(globalCtx, slog.LevelDebug) {
			globalLogger.LogAttrs(globalCtx, slog.LevelDebug, "restarting", slog.String("worker", worker.qualifiedName), slog.Int("thread", handler.thread.threadIndex), slog.Int("exit_status", exitStatus))
		}

		return
	}

	// worker has thrown a fatal error or has not reached frankenphp_handle_request
	if handler.isBootingScript {
		metrics.StopWorker(worker.qualifiedName, StopReasonBootFailure)
	} else {
		metrics.StopWorker(worker.qualifiedName, StopReasonCrash)
	}

	if !handler.isBootingScript {
		// fatal error (could be due to exit(1), timeouts, etc.)
		// unlike a clean restart, this took down any in-flight request, so
		// surface it above debug level, with the exit status needed to triage it
		if globalLogger.Enabled(globalCtx, slog.LevelWarn) {
			globalLogger.LogAttrs(globalCtx, slog.LevelWarn, "unexpected termination, restarting", slog.String("worker", worker.qualifiedName), slog.Int("thread", handler.thread.threadIndex), slog.Int("exit_status", exitStatus))
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
			globalLogger.LogAttrs(globalCtx, slog.LevelError, "(watcher enabled) worker script has not reached frankenphp_handle_request()", slog.String("worker", worker.qualifiedName), slog.Int("thread", handler.thread.threadIndex))
		}
	} else {
		// rare case where worker script has failed on a restart during normal operation
		// this can happen if startup success depends on external resources
		if globalLogger.Enabled(globalCtx, slog.LevelWarn) {
			globalLogger.LogAttrs(globalCtx, slog.LevelWarn, "worker script has failed on restart", slog.String("worker", worker.qualifiedName), slog.Int("thread", handler.thread.threadIndex), slog.Int("failures", handler.failureCount))
		}
	}

	// wait a bit and try again
	time.Sleep(restartBackoff(handler.failureCount))
	handler.failureCount++
}

// restartBackoff is the wait before a worker script is re-run after a
// failure: quadratic in the number of consecutive failures, capped at one
// second; shared by HTTP and background workers. The cap comes before the
// multiplication, which overflows a duration past some 300k failures, a
// count a crash loop reaches on its own in a few days
func restartBackoff(failures int) time.Duration {
	if failures >= 4 {
		return time.Second
	}

	return time.Duration(failures*failures*100) * time.Millisecond
}

// waitForWorkerRequest is called during frankenphp_handle_request in the php worker script.
func (handler *workerThread) waitForWorkerRequest() (bool, any) {
	// unpin any memory left over from previous requests
	handler.thread.Unpin()

	if globalLogger.Enabled(globalCtx, slog.LevelDebug) {
		globalLogger.LogAttrs(globalCtx, slog.LevelDebug, "waiting for request", slog.String("worker", handler.worker.qualifiedName), slog.Int("thread", handler.thread.threadIndex))
	}

	// Clear the first dummy request created to initialize the worker
	if handler.isBootingScript {
		handler.isBootingScript = false
		handler.failureCount = 0
		if !C.frankenphp_shutdown_dummy_request() {
			panic("Not in CGI context")
		}

		// worker is truly ready only after reaching frankenphp_handle_request()
		metrics.ReadyWorker(handler.worker.qualifiedName)
	}

	// max_requests reached: signal reboot for full ZTS cleanup
	if maxRequestsPerThread > 0 && handler.requestCount >= maxRequestsPerThread {
		if globalLogger.Enabled(globalCtx, slog.LevelDebug) {
			globalLogger.LogAttrs(globalCtx, slog.LevelDebug, "max requests reached, restarting",
				slog.String("worker", handler.worker.qualifiedName),
				slog.Int("thread", handler.thread.threadIndex),
				slog.Int("max_requests", maxRequestsPerThread),
			)
		}

		if handler.thread.reboot() {
			return false, nil
		}
	}

	if handler.state.Is(state.TransitionComplete) {
		handler.state.Set(state.Ready)
	}

	handler.state.MarkAsWaiting(true)

	var fc *frankenPHPContext
	select {
	case <-handler.thread.drainChan:
		if globalLogger.Enabled(globalCtx, slog.LevelDebug) {
			globalLogger.LogAttrs(globalCtx, slog.LevelDebug, "shutting down", slog.String("worker", handler.worker.qualifiedName), slog.Int("thread", handler.thread.threadIndex))
		}

		return false, nil
	case fc = <-handler.thread.requestChan:
	case fc = <-handler.worker.requestChan:
	}

	handler.requestCount++
	handler.thread.contextMu.Lock()
	handler.workerFrankenPHPContext = fc
	handler.thread.contextMu.Unlock()
	handler.state.MarkAsWaiting(false)

	if fc.logger.Enabled(fc.ctx, slog.LevelDebug) {
		if handler.workerFrankenPHPContext.request == nil {
			fc.logger.LogAttrs(fc.ctx, slog.LevelDebug, "request handling started", slog.String("worker", handler.worker.qualifiedName), slog.Int("thread", handler.thread.threadIndex))
		} else {
			fc.logger.LogAttrs(fc.ctx, slog.LevelDebug, "request handling started", slog.String("worker", handler.worker.qualifiedName), slog.Int("thread", handler.thread.threadIndex), slog.String("url", handler.workerFrankenPHPContext.request.RequestURI))
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
	fc := thread.handler.frankenPHPContext()

	if retval != nil {
		r, err := GoValue[any](unsafe.Pointer(retval))
		if err != nil && fc.logger.Enabled(fc.ctx, slog.LevelError) {
			fc.logger.LogAttrs(fc.ctx, slog.LevelError, "cannot convert return value", slog.Any("error", err), slog.Int("thread", thread.threadIndex))
		}

		fc.handlerReturn = r
	}

	thread.requestCount.Add(1)

	fc.closeContext()
	thread.contextMu.Lock()
	thread.handler.(*workerThread).workerFrankenPHPContext = nil
	thread.contextMu.Unlock()

	if fc.logger.Enabled(fc.ctx, slog.LevelDebug) {
		if fc.request == nil {
			fc.logger.LogAttrs(fc.ctx, slog.LevelDebug, "request handling finished", slog.String("worker", fc.worker.qualifiedName), slog.Int("thread", thread.threadIndex))
		} else {
			fc.logger.LogAttrs(fc.ctx, slog.LevelDebug, "request handling finished", slog.String("worker", fc.worker.qualifiedName), slog.Int("thread", thread.threadIndex), slog.String("url", fc.request.RequestURI))
		}
	}
}

// when frankenphp_finish_request() is directly called from PHP
//
//export go_frankenphp_finish_php_request
func go_frankenphp_finish_php_request(threadIndex C.uintptr_t) {
	thread := phpThreads[threadIndex]
	fc := thread.handler.frankenPHPContext()

	fc.closeContext()

	if fc.logger.Enabled(fc.ctx, slog.LevelDebug) {
		fc.logger.LogAttrs(fc.ctx, slog.LevelDebug, "request handling finished", slog.Int("thread", thread.threadIndex), slog.String("url", fc.request.RequestURI))
	}
}
