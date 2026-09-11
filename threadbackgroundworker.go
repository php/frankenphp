package frankenphp

// /* closesocket() lives in ws2_32: the PHP dll links it, a cgo build does not by itself */
// #cgo windows LDFLAGS: -lws2_32
// #include "frankenphp.h"
import "C"
import (
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/dunglas/frankenphp/internal/state"
)

// backgroundWorkerThread is the threadHandler of background worker scripts.
// It owns their lifecycle: boot the script, re-run it when it exits, restart
// it with a quadratic backoff when it crashes. Background workers share the
// PHP runtime with HTTP threads but never receive HTTP requests. The script
// can park on the stream returned by frankenphp_get_worker_handle(), which
// reaches EOF when the thread is drained, to exit gracefully on shutdown,
// reboot or handler transition.
type backgroundWorkerThread struct {
	workerLifecycle

	dummyFrankenPHPContext *frankenPHPContext
	failureCount           int // number of consecutive failed runs

	// crashCount is the number of runs that crashed past their ready point
	// in a row, paces their restarts; a cooperative exit resets it. Only
	// touched on the PHP thread.
	crashCount int

	// isBootingScript is true until the current run waits on its handle, a
	// stream_select() or a read on the stream returned by
	// frankenphp_get_worker_handle(), the background analog of an HTTP
	// worker reaching frankenphp_handle_request(). Only touched on the PHP
	// thread (setup, the C callback during execution, teardown).
	isBootingScript bool

	// bootTimer warns when a run has not waited on its handle after
	// backgroundBootWarnDelay; only touched on the PHP thread
	bootTimer *time.Timer

	// stopSock holds the Go side's end of this thread's stop socket pair
	// (per thread so pool workers drain independently); the other end is
	// exposed to the script via frankenphp_get_worker_handle(). Wide enough
	// for a Windows SOCKET, -1 when not held. Atomic because drain() closes
	// it from another goroutine.
	stopSock atomic.Int64
}

// backgroundBootWarnDelay is how long a run may go without waiting on its
// handle before a warning: Init() and Shutdown() wait for that point, so a
// script that never gets there hangs both silently
const backgroundBootWarnDelay = 10 * time.Second

func convertToBackgroundWorkerThread(thread *phpThread, worker *worker) {
	handler := &backgroundWorkerThread{workerLifecycle: newWorkerLifecycle(thread, worker)}
	handler.stopSock.Store(-1)
	thread.setHandler(handler)
	worker.attachThread(thread)
}

func (handler *backgroundWorkerThread) name() string {
	return "Background Worker PHP Thread - " + handler.worker.fileName
}

func (handler *backgroundWorkerThread) frankenPHPContext() *frankenPHPContext {
	return handler.dummyFrankenPHPContext
}

// drain closes the Go side's end of the stop socket pair so a script
// parked on the other end wakes up with EOF and can exit its loop. Called
// right before drainChan is closed on shutdown and reboot; also reused
// internally to release the socket on the other exit paths.
func (handler *backgroundWorkerThread) drain() {
	if s := handler.stopSock.Swap(-1); s >= 0 {
		C.frankenphp_worker_close_stop_sock(C.intptr_t(s))
	}
}

func (handler *backgroundWorkerThread) beforeScriptExecution() string {
	return handler.workerLifecycle.beforeScriptExecution(handler)
}

// startScript keeps trying to start the script: unlike an HTTP worker, whose
// setup cannot fail, a background worker needs a socket pair per run
func (handler *backgroundWorkerThread) startScript() string {
	for {
		err := handler.setupScript()
		if err == nil {
			return handler.worker.fileName
		}

		if globalLogger.Enabled(globalCtx, slog.LevelError) {
			globalLogger.LogAttrs(globalCtx, slog.LevelError, "failed to start background worker", slog.String("worker", handler.worker.qualifiedName), slog.Int("thread", handler.thread.threadIndex), slog.Any("error", err))
		}

		// fail fast during startup so Init() surfaces the error to the
		// operator; past startup, back off and retry like a crash
		if startupFailChan != nil {
			startupFailChan <- err
			handler.thread.state.Set(state.ShuttingDown)

			return handler.beforeScriptExecution()
		}

		handler.backoff()
		if !handler.state.Is(state.Ready) && !handler.state.Is(state.TransitionComplete) {
			// drained during the backoff (shutdown, reboot, transition)
			return handler.beforeScriptExecution()
		}
	}
}

// setupScript marks the thread as a background worker on the C side and
// takes ownership of the Go side's end of its stop socket pair.
func (handler *backgroundWorkerThread) setupScript() error {
	s := int64(C.frankenphp_set_background_worker_and_get_stop_sock())
	if s < 0 {
		return fmt.Errorf("failed to create the stop socket pair of background worker %q", handler.worker.qualifiedName)
	}
	handler.stopSock.Store(s)

	switch handler.state.Get() {
	case state.ShuttingDown, state.Rebooting, state.ForceRebooting, state.TransitionRequested:
		// a concurrent drain may have run before the socket was published;
		// close it now so the script observes EOF immediately
		handler.drain()
	}

	fc, err := newWorkerDummyContext(handler.worker)
	if err != nil {
		handler.drain()
		return err
	}
	handler.dummyFrankenPHPContext = fc

	handler.isBootingScript = true
	metrics.StartWorker(handler.worker.qualifiedName)
	// the run's logger and context, not the globals: Stop() does not wait
	// for a callback that already started, and a shutdown finishing
	// meanwhile resets those
	logger, ctx, name, threadIndex := fc.logger, fc.ctx, handler.worker.qualifiedName, handler.thread.threadIndex
	handler.bootTimer = time.AfterFunc(backgroundBootWarnDelay, func() {
		if logger.Enabled(ctx, slog.LevelWarn) {
			logger.LogAttrs(ctx, slog.LevelWarn, "background worker has not waited on its handle yet, Init() and Shutdown() wait for it, see frankenphp_get_worker_handle()", slog.String("worker", name), slog.Int("thread", threadIndex))
		}
	})

	if fc.logger.Enabled(fc.ctx, slog.LevelDebug) {
		fc.logger.LogAttrs(fc.ctx, slog.LevelDebug, "starting background worker", slog.String("worker", handler.worker.qualifiedName), slog.Int("thread", handler.thread.threadIndex))
	}

	// the thread stays in TransitionComplete until the script waits on its
	// handle, see go_frankenphp_background_worker_ready

	return nil
}

func (handler *backgroundWorkerThread) afterScriptExecution(exitStatus int) {
	// the Go side's end of the stop socket pair belongs to this thread;
	// release it on every exit path so the next run gets a fresh pair
	// (drain() already took it when the exit was drain-triggered)
	handler.drain()
	worker := handler.worker
	handler.dummyFrankenPHPContext = nil

	handler.stopBootTimer()
	handler.state.MarkAsWaiting(false)

	// cooperative exit: the script waited on its handle and returned cleanly,
	// re-run it, unless the thread is being drained (beforeScriptExecution
	// checks the state)
	if exitStatus == 0 && !handler.isBootingScript {
		handler.crashCount = 0
		metrics.StopWorker(worker.qualifiedName, StopReasonRestart)

		if globalLogger.Enabled(globalCtx, slog.LevelDebug) {
			globalLogger.LogAttrs(globalCtx, slog.LevelDebug, "restarting background worker", slog.String("worker", worker.qualifiedName), slog.Int("thread", handler.thread.threadIndex))
		}

		return
	}

	// crash after the ready point: restart without counting toward
	// max_consecutive_failures, that cap is about a script that never boots.
	// The wait still applies: unlike an HTTP worker, which can only crash
	// after a request reached frankenphp_handle_request() and is therefore
	// paced by traffic, a background worker reaches its ready point on its
	// own and a script crashing right after it would spin
	if !handler.isBootingScript {
		metrics.StopWorker(worker.qualifiedName, StopReasonCrash)

		if globalLogger.Enabled(globalCtx, slog.LevelWarn) {
			globalLogger.LogAttrs(globalCtx, slog.LevelWarn, "background worker crashed, restarting", slog.String("worker", worker.qualifiedName), slog.Int("thread", handler.thread.threadIndex), slog.Int("exit_status", exitStatus), slog.Int("crashes", handler.crashCount))
		}

		time.Sleep(restartBackoff(handler.crashCount))
		handler.crashCount++

		return
	}

	// boot failure: the script exited before waiting on its handle, a clean
	// exit included, which would otherwise respawn in a tight loop.
	// StopReasonBootFailure skips the ready-gauge decrement, matching the
	// ReadyWorker call that never happened
	metrics.StopWorker(worker.qualifiedName, StopReasonBootFailure)

	// max_consecutive_failures only fails hard during startup, where it
	// surfaces on startupFailChan so Init() returns the error to the
	// operator. Past startup, a failing background worker keeps
	// restarting with a louder log line: silently giving up would leave
	// the server in a broken half-state with no clear way to recover.
	pastCap := worker.maxConsecutiveFailures >= 0 && handler.failureCount >= worker.maxConsecutiveFailures
	if pastCap && startupFailChan != nil && !watcherIsEnabled {
		if exitStatus == 0 {
			startupFailChan <- fmt.Errorf("background worker %s exits without waiting on its handle, see frankenphp_get_worker_handle()", worker.fileName)
		} else {
			startupFailChan <- fmt.Errorf("too many consecutive failures: background worker %s keeps crashing", worker.fileName)
		}
		handler.thread.state.Set(state.ShuttingDown)
		return
	}

	logLevel := slog.LevelWarn
	logMsg := "background worker failed before waiting on its handle, restarting"
	if exitStatus == 0 {
		logMsg = "background worker exited without waiting on its handle, restarting"
	}
	if pastCap {
		logLevel = slog.LevelError
		logMsg = "background worker exceeded max_consecutive_failures, still restarting"
	}
	if globalLogger.Enabled(globalCtx, logLevel) {
		globalLogger.LogAttrs(globalCtx, logLevel, logMsg, slog.String("worker", worker.qualifiedName), slog.Int("thread", handler.thread.threadIndex), slog.Int("failures", handler.failureCount), slog.Int("exit_status", exitStatus))
	}

	handler.backoff()
}

func (handler *backgroundWorkerThread) stopBootTimer() {
	if handler.bootTimer != nil {
		handler.bootTimer.Stop()
		handler.bootTimer = nil
	}
}

//export go_frankenphp_background_worker_ready
func go_frankenphp_background_worker_ready(threadIndex C.uintptr_t) {
	// called on the PHP thread on the first wait on the handle; the handler
	// is a backgroundWorkerThread because frankenphp_get_worker_handle()
	// throws on every other thread kind
	if handler, ok := phpThreads[threadIndex].handler.(*backgroundWorkerThread); ok && handler.isBootingScript {
		handler.isBootingScript = false
		// the boot succeeded, only consecutive boot failures count
		handler.failureCount = 0
		handler.stopBootTimer()
		metrics.ReadyWorker(handler.worker.qualifiedName)
		// parked from now on as far as the threads state endpoint is concerned
		handler.state.MarkAsWaiting(true)

		// like an HTTP worker reaching frankenphp_handle_request(), the thread
		// is ready only now: initWorkers() waits for this state, so a script
		// that fails before waiting on its handle still fails Init()
		if handler.state.Is(state.TransitionComplete) {
			handler.state.Set(state.Ready)
		}
	}
}

// backoff waits before the next run of a crashed script, see restartBackoff
func (handler *backgroundWorkerThread) backoff() {
	time.Sleep(restartBackoff(handler.failureCount))
	handler.failureCount++
}
