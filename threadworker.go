package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"
	"unsafe"

	"github.com/dunglas/frankenphp/internal/blockio"
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
	requestCount            int  // number of requests handled since last restart

	// per-request timeout watchdog (worker.workerTimeout > 0)
	timeoutMu    sync.Mutex
	requestTimer *time.Timer
	requestEpoch uint64 // bumped on every request finish to invalidate a stale watchdog
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
	case state.Rebooting:
		return ""
	case state.RebootReady:
		handler.requestCount = 0
		handler.state.Set(state.Ready)
		return handler.beforeScriptExecution()
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

func (handler *workerThread) drain() {}

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
	handler.requestCount = 0

	if globalLogger.Enabled(ctx, slog.LevelDebug) {
		globalLogger.LogAttrs(ctx, slog.LevelDebug, "starting", slog.String("worker", worker.name), slog.Int("thread", handler.thread.threadIndex))
	}
}

func tearDownWorkerScript(handler *workerThread, exitStatus int) {
	worker := handler.worker

	// Stop any pending request-timeout watchdog. On a fatal error (including a
	// timeout-induced bailout) go_frankenphp_finish_worker_request is skipped,
	// so this is the crash-path counterpart to the cancel done there.
	handler.cancelRequestTimeout()

	handler.dummyFrankenPHPContext = nil
	handler.dummyContext = nil

	// if the worker request is not nil, the script might have crashed
	// make sure to close the worker request context
	if handler.workerFrankenPHPContext != nil {
		handler.workerFrankenPHPContext.closeContext()
		handler.thread.contextMu.Lock()
		handler.workerFrankenPHPContext = nil
		handler.workerContext = nil
		handler.thread.contextMu.Unlock()
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

	// max_requests reached: signal reboot for full ZTS cleanup
	if maxRequestsPerThread > 0 && handler.requestCount >= maxRequestsPerThread {
		if globalLogger.Enabled(globalCtx, slog.LevelDebug) {
			globalLogger.LogAttrs(globalCtx, slog.LevelDebug, "max requests reached, restarting",
				slog.String("worker", handler.worker.name),
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

	handler.requestCount++
	handler.thread.contextMu.Lock()
	handler.workerContext = requestCH.ctx
	handler.workerFrankenPHPContext = requestCH.frankenPHPContext
	handler.thread.contextMu.Unlock()
	handler.state.MarkAsWaiting(false)

	if globalLogger.Enabled(requestCH.ctx, slog.LevelDebug) {
		if handler.workerFrankenPHPContext.request == nil {
			globalLogger.LogAttrs(requestCH.ctx, slog.LevelDebug, "request handling started", slog.String("worker", handler.worker.name), slog.Int("thread", handler.thread.threadIndex))
		} else {
			globalLogger.LogAttrs(requestCH.ctx, slog.LevelDebug, "request handling started", slog.String("worker", handler.worker.name), slog.Int("thread", handler.thread.threadIndex), slog.String("url", handler.workerFrankenPHPContext.request.RequestURI))
		}
	}

	handler.armRequestTimeout()

	return true, handler.workerFrankenPHPContext.handlerParameters
}

// armRequestTimeout starts a watchdog for the request that is about to execute.
// If the request runs longer than worker.workerTimeout, the watchdog arms the
// per-thread timeout flag + VM interrupt (so a "Worker request timeout" fatal
// is raised at the next opcode boundary), on Linux shuts down the socket fd(s)
// the thread is blocked on (so a blocked DB/HTTP read returns instead of
// retrying EINTR) and wakes EINTR-abortable waits (sleep) via the realtime kill
// signal (Linux/FreeBSD). The worker script then restarts.
//
// A timeout of 0 disables the watchdog. On platforms without a realtime kill
// signal (macOS, Windows non-alertable waits) a blocking syscall already in
// progress cannot be unblocked; only the VM-interrupt flag is set, which is
// honored at the next opcode boundary (so CPU-bound overruns are still caught).
func (handler *workerThread) armRequestTimeout() {
	timeout := handler.worker.workerTimeout
	if timeout <= 0 {
		return
	}

	thread := handler.thread

	// Reset any stale pending flag from a previous request whose watchdog raced
	// completion, so it can't abort this one. Runs on the PHP thread, and always
	// after such a stale watchdog has finished: cancelRequestTimeout (called on
	// the same PHP thread when the previous request ended) blocks on timeoutMu
	// until a mid-flight watchdog body has run to completion.
	C.frankenphp_clear_worker_timeout(C.uintptr_t(thread.threadIndex))

	handler.timeoutMu.Lock()
	epoch := handler.requestEpoch
	handler.requestTimer = time.AfterFunc(timeout, func() {
		// timeoutMu is held for the entire interrupt sequence so the watchdog
		// cannot interleave with its request finishing: cancelRequestTimeout
		// (and therefore the next request's arm + pending-flag clear) waits
		// until this body is done. Checking the epoch under the same mutex
		// makes a watchdog whose request already finished a strict no-op - it
		// can never arm the timeout for (or shut down the sockets of) a request
		// it wasn't armed for, and it can never touch the per-thread C state
		// after a teardown's cancelRequestTimeout returned.
		handler.timeoutMu.Lock()
		defer handler.timeoutMu.Unlock()

		if handler.requestEpoch != epoch {
			return
		}

		// Only interrupt a thread that is actively handling a request. Any
		// other state means the thread is yielding, restarting, rebooting or
		// shutting down on its own and its force-kill slot may already be
		// cleared (frankenphp_force_kill_thread is still safe on a zeroed slot).
		if !handler.state.Is(state.Ready) {
			return
		}

		if globalLogger.Enabled(globalCtx, slog.LevelWarn) {
			globalLogger.LogAttrs(globalCtx, slog.LevelWarn, "worker request timeout, interrupting thread",
				slog.String("worker", handler.worker.name),
				slog.Int("thread", thread.threadIndex),
				slog.Duration("timeout", timeout),
			)
		}

		thread.forceKillMu.RLock()
		// 1. Arm: set the pending flag + VM interrupt so the interrupt hook
		//    raises our fatal as soon as the thread runs PHP again. Done before
		//    any wakeup so the driver's own I/O error can't pre-empt the message.
		C.frankenphp_arm_worker_timeout(
			C.uintptr_t(thread.threadIndex),
			thread.forceKill,
			C.double(timeout.Seconds()),
		)
		// 2. Abort the fd the thread is blocked on (e.g. a mysqlnd socket that
		//    isn't reachable via the resource list) so a retried blocking read
		//    fails terminally instead of resuming.
		blockio.Abort(int(thread.kernelTID.Load()))
		// 3. Wake EINTR-abortable waits (sleep) that have no fd to shut down.
		C.frankenphp_wake_worker_thread(thread.forceKill)
		thread.forceKillMu.RUnlock()
	})
	handler.timeoutMu.Unlock()
}

// cancelRequestTimeout stops the watchdog armed by armRequestTimeout and bumps
// the request epoch so a watchdog whose timer already fired but has not yet taken
// timeoutMu becomes a no-op. Because the watchdog body holds timeoutMu for its
// whole run, this call also blocks until a mid-flight watchdog has finished -
// after it returns, the watchdog can no longer interrupt the thread or touch
// the per-thread C state. Safe to call when no watchdog is armed.
func (handler *workerThread) cancelRequestTimeout() {
	handler.timeoutMu.Lock()
	handler.requestEpoch++
	if handler.requestTimer != nil {
		handler.requestTimer.Stop()
		handler.requestTimer = nil
	}
	handler.timeoutMu.Unlock()
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

	// the request completed normally: disarm the timeout watchdog
	thread.handler.(*workerThread).cancelRequestTimeout()

	ctx := thread.context()
	fc := ctx.Value(contextKey).(*frankenPHPContext)

	if retval != nil {
		r, err := GoValue[any](unsafe.Pointer(retval))
		if err != nil && globalLogger.Enabled(ctx, slog.LevelError) {
			globalLogger.LogAttrs(ctx, slog.LevelError, "cannot convert return value", slog.Any("error", err), slog.Int("thread", thread.threadIndex))
		}

		fc.handlerReturn = r
	}

	thread.requestCount.Add(1)

	fc.closeContext()
	thread.contextMu.Lock()
	thread.handler.(*workerThread).workerFrankenPHPContext = nil
	thread.handler.(*workerThread).workerContext = nil
	thread.contextMu.Unlock()

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
