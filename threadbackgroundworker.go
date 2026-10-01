package frankenphp

// /* closesocket() lives in ws2_32: the PHP dll links it, a cgo build does not by itself */
// #cgo windows LDFLAGS: -lws2_32
// #include "frankenphp.h"
import "C"
import (
	"fmt"
	"log/slog"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/dunglas/frankenphp/internal/state"
)

// backgroundWorkerThread is the threadHandler of background worker scripts:
// boot the script, re-run it when it exits, restart it with a quadratic
// backoff when it crashes. The script parks on the stream returned by
// WorkerHandle::getStream(), which reaches EOF when the thread is drained, so
// it exits on shutdown, reboot or handler transition; the handle also carries
// a wake-up per task sent to the worker, see SentTaskHandle.
type backgroundWorkerThread struct {
	workerLifecycle

	// context of the current run, a background worker serves no request
	context      *frankenPHPContext
	failureCount int // number of consecutive failed runs

	// consecutive short runs, crashed or not, pacing the restarts; a run
	// that outlives maxRestartBackoff resets the count
	crashCount   int
	runStartedAt time.Time

	// true until the run calls WorkerHandle::tick(), the background analog
	// of an HTTP worker reaching frankenphp_handle_request()
	isBootingScript bool
	bootTimer       *time.Timer

	// tasks picked up and not closed yet: the thread is busy rather than
	// waiting on the threads endpoint meanwhile. Only touched on the PHP
	// thread, pickup and close both happen there
	openTasks int

	// the Go side's end of this thread's stop socket pair, the script holding
	// the other end; one per thread so pool workers drain independently. Wide
	// enough for a Windows SOCKET, -1 when not held. Guarded by
	// worker.tasks.mu, senders write their wake-up line to it
	stopSock int64

	// set by WorkerHandle::tick() when no task is queued, as the script is
	// about to wait on its handle: senders wake one parked thread per task.
	// Guarded by worker.tasks.mu
	parked bool

	// senders writing to stopSock outside of worker.tasks.mu, so the socket is
	// only closed once they are done: holding the mutex across that syscall
	// would park every contending thread, and a thread inside a cgo callback
	// parks at the price of a scheduler hand-off
	signaling atomic.Int32
}

// backgroundBootWarnDelay is how long a run may go without calling
// WorkerHandle::tick() before a warning: Init() and Shutdown() wait for
// that point, so a script that never gets there hangs both silently
const backgroundBootWarnDelay = 10 * time.Second

func convertToBackgroundWorkerThread(thread *phpThread, worker *worker) {
	handler := &backgroundWorkerThread{
		workerLifecycle: newWorkerLifecycle(thread, worker),
		stopSock:        -1,
	}
	thread.setHandler(handler)
	worker.attachThread(thread)
}

func (handler *backgroundWorkerThread) name() string {
	return "Background Worker PHP Thread - " + handler.worker.fileName
}

func (handler *backgroundWorkerThread) frankenPHPContext() *frankenPHPContext {
	return handler.context
}

// drain closes the Go side's end of the stop socket pair, so a script parked
// on the other end wakes up with EOF. Called right before drainChan is closed
// on shutdown and reboot, and on the other exit paths to release the socket.
func (handler *backgroundWorkerThread) drain() {
	q := &handler.worker.tasks
	q.mu.Lock()
	s := handler.stopSock
	handler.stopSock = -1
	handler.parked = false
	q.mu.Unlock()

	if s >= 0 {
		// senders that took the socket before it was withdrawn finish their write first
		for handler.signaling.Load() > 0 {
			runtime.Gosched()
		}
		C.frankenphp_worker_close_stop_sock(C.intptr_t(s))
	}
}

func (handler *backgroundWorkerThread) beforeScriptExecution() string {
	return handler.workerLifecycle.beforeScriptExecution(handler.startScript)
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
		if reportStartupFailure(err) {
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

// setupScript marks the thread as a background worker on the C side and takes
// ownership of the Go side's end of its stop socket pair.
func (handler *backgroundWorkerThread) setupScript() error {
	s := int64(C.frankenphp_set_background_worker_and_get_stop_sock())
	if s < 0 {
		return fmt.Errorf("failed to create the stop socket pair of background worker %q", handler.worker.qualifiedName)
	}
	// tasks queued meanwhile reach the new run when it parks, see
	// go_frankenphp_background_worker_park
	q := &handler.worker.tasks
	q.mu.Lock()
	handler.stopSock = s
	q.mu.Unlock()

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
	handler.thread.contextMu.Lock()
	handler.context = fc
	handler.thread.contextMu.Unlock()

	handler.isBootingScript = true
	handler.runStartedAt = time.Now()
	handler.openTasks = 0
	metrics.StartWorker(handler.worker.name, handler.worker.server.name)
	// the run's logger and context, not the globals: a shutdown finishing
	// meanwhile resets those, and Stop() does not wait for this callback
	logger, ctx, name, threadIndex := fc.logger, fc.ctx, handler.worker.qualifiedName, handler.thread.threadIndex
	handler.bootTimer = time.AfterFunc(backgroundBootWarnDelay, func() {
		if logger.Enabled(ctx, slog.LevelWarn) {
			logger.LogAttrs(ctx, slog.LevelWarn, "background worker has not called WorkerHandle::tick() yet, Init() and Shutdown() wait for it", slog.String("worker", name), slog.Int("thread", threadIndex))
		}
	})

	if fc.logger.Enabled(fc.ctx, slog.LevelDebug) {
		fc.logger.LogAttrs(fc.ctx, slog.LevelDebug, "starting background worker", slog.String("worker", handler.worker.qualifiedName), slog.Int("thread", handler.thread.threadIndex))
	}

	// the thread stays in TransitionComplete until the script calls
	// WorkerHandle::tick(), see go_frankenphp_background_worker_ready

	return nil
}

func (handler *backgroundWorkerThread) afterScriptExecution(exitStatus int) {
	// release the socket on every exit path so the next run gets a fresh
	// pair; drain() already took it when the exit was drain-triggered
	handler.drain()
	worker := handler.worker
	handler.thread.contextMu.Lock()
	handler.context = nil
	handler.thread.contextMu.Unlock()

	handler.stopBootTimer()
	handler.state.MarkAsWaiting(false)

	// exit past the ready point, cooperative or a crash: re-run the script
	// without counting toward max_consecutive_failures, that cap is about a
	// script that never boots. An HTTP worker is paced by traffic, a
	// background one reaches its ready point on its own, so pace it here
	if !handler.isBootingScript {
		if exitStatus == 0 {
			metrics.StopWorker(worker.name, worker.server.name, StopReasonRestart)

			if globalLogger.Enabled(globalCtx, slog.LevelDebug) {
				globalLogger.LogAttrs(globalCtx, slog.LevelDebug, "restarting background worker", slog.String("worker", worker.qualifiedName), slog.Int("thread", handler.thread.threadIndex))
			}
		} else {
			metrics.StopWorker(worker.name, worker.server.name, StopReasonCrash)

			if globalLogger.Enabled(globalCtx, slog.LevelWarn) {
				globalLogger.LogAttrs(globalCtx, slog.LevelWarn, "background worker crashed, restarting", slog.String("worker", worker.qualifiedName), slog.Int("thread", handler.thread.threadIndex), slog.Int("exit_status", exitStatus), slog.Int("crashes", handler.crashCount))
			}
		}

		// a worker processing a batch and returning is not a worker spinning
		// on an immediate exit; a crash never resets the count, however long
		// the run lasted
		if exitStatus == 0 && time.Since(handler.runStartedAt) > minHealthyRun {
			handler.crashCount = 0
		}
		handler.wait(restartBackoff(handler.crashCount))
		handler.crashCount++

		return
	}

	// the script exited before calling WorkerHandle::tick(), a clean exit
	// included; StopReasonBootFailure skips the ready-gauge decrement,
	// matching the ReadyWorker call that never happened
	metrics.StopWorker(worker.name, worker.server.name, StopReasonBootFailure)

	// max_consecutive_failures only fails hard during startup, through
	// startupFailChan; past it a failing worker keeps restarting with a
	// louder log line rather than leave the server in a broken half-state.
	// A worker Init() gave up on stops here whatever the cap says, it was
	// drained precisely so this path could end the thread
	pastCap := worker.maxConsecutiveFailures >= 0 && handler.failureCount >= worker.maxConsecutiveFailures
	if worker.bootTimedOut.Load() {
		reportStartupFailure(fmt.Errorf("background worker %s did not call WorkerHandle::tick() in time", worker.fileName))
		handler.thread.state.Set(state.ShuttingDown)

		return
	}
	if pastCap && !watcherIsEnabled {
		var err error
		if exitStatus == 0 {
			err = fmt.Errorf("background worker %s exits without calling WorkerHandle::tick()", worker.fileName)
		} else {
			err = fmt.Errorf("too many consecutive failures: background worker %s keeps crashing", worker.fileName)
		}
		if reportStartupFailure(err) {
			handler.thread.state.Set(state.ShuttingDown)
			return
		}
	}

	logLevel := slog.LevelWarn
	logMsg := "background worker failed before calling WorkerHandle::tick(), restarting"
	if exitStatus == 0 {
		logMsg = "background worker exited without calling WorkerHandle::tick(), restarting"
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
	// called on the PHP thread by the first WorkerHandle::tick() of a run;
	// that function throws on every other thread kind
	handler, ok := phpThreads[threadIndex].handler.(*backgroundWorkerThread)
	if !ok {
		panic("WorkerHandle::tick() called on a thread that is not a background worker")
	}

	if handler.isBootingScript {
		handler.isBootingScript = false
		// the boot succeeded, only consecutive boot failures count
		handler.failureCount = 0
		handler.stopBootTimer()
		handler.worker.markReady()
		metrics.ReadyWorker(handler.worker.name, handler.worker.server.name)
		// parked from now on as far as the threads state endpoint is concerned
		handler.state.MarkAsWaiting(true)

		// initWorkers() waits for this state, so a script that fails before
		// its first tick still fails Init()
		if handler.state.Is(state.TransitionComplete) {
			handler.state.Set(state.Ready)
		}
	}
}

// backoff waits before the next run of a crashed script, see restartBackoff
func (handler *backgroundWorkerThread) backoff() {
	handler.wait(restartBackoff(handler.failureCount))
	handler.failureCount++
}

// wait sleeps between two runs, cut short by a drain, which the next
// beforeScriptExecution() picks up from the state
func (handler *backgroundWorkerThread) wait(d time.Duration) {
	select {
	case <-handler.thread.drainChan:
	case <-time.After(d):
	}
}
