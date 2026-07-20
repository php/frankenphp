package frankenphp

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
	state                  *state.ThreadState
	thread                 *phpThread
	worker                 *worker
	dummyFrankenPHPContext *frankenPHPContext
	failureCount           int // number of consecutive non-zero exits

	// stopFdWrite holds the write end of this thread's stop pipe (per
	// thread so pool workers drain independently); the read end is exposed
	// to the script via frankenphp_get_worker_handle(). Atomic because
	// drain() closes it from another goroutine.
	stopFdWrite atomic.Int32
}

func convertToBackgroundWorkerThread(thread *phpThread, worker *worker) {
	handler := &backgroundWorkerThread{
		state:  thread.state,
		thread: thread,
		worker: worker,
	}
	handler.stopFdWrite.Store(-1)
	thread.setHandler(handler)
	worker.attachThread(thread)
}

func (handler *backgroundWorkerThread) name() string {
	return "Background Worker PHP Thread - " + handler.worker.fileName
}

func (handler *backgroundWorkerThread) frankenPHPContext() *frankenPHPContext {
	return handler.dummyFrankenPHPContext
}

// drain closes the stop pipe's write end so a script parked in
// stream_select on the read end wakes up and can exit its loop. Called
// right before drainChan is closed on shutdown and reboot; also reused
// internally to release the fd on the other exit paths.
func (handler *backgroundWorkerThread) drain() {
	if fd := handler.stopFdWrite.Swap(-1); fd >= 0 {
		C.frankenphp_worker_close_fd(C.int(fd))
	}
}

// beforeScriptExecution returns the name of the script or an empty string on shutdown
func (handler *backgroundWorkerThread) beforeScriptExecution() string {
	switch handler.state.Get() {
	case state.TransitionRequested:
		if handler.worker.onThreadShutdown != nil {
			handler.worker.onThreadShutdown(handler.thread.threadIndex)
		}
		handler.worker.detachThread(handler.thread)
		return handler.thread.transitionToNewHandler()
	case state.Ready, state.TransitionComplete:
		handler.thread.updateContext(true)
		if handler.worker.onThreadReady != nil {
			handler.worker.onThreadReady(handler.thread.threadIndex)
		}

		for {
			err := handler.setupScript()
			if err == nil {
				return handler.worker.fileName
			}

			if globalLogger.Enabled(globalCtx, slog.LevelError) {
				globalLogger.LogAttrs(globalCtx, slog.LevelError, "failed to start background worker", slog.String("worker", handler.worker.name), slog.Int("thread", handler.thread.threadIndex), slog.Any("error", err))
			}

			// fail fast during startup so Init() surfaces the error to the
			// operator; past startup, back off and retry like a crash
			if startupFailChan != nil {
				startupFailChan <- err
				handler.thread.state.Set(state.ShuttingDown)
				return handler.beforeScriptExecution()
			}

			handler.backoff()
			if !handler.state.Is(state.Ready) {
				// drained during the backoff (shutdown, reboot, transition)
				return handler.beforeScriptExecution()
			}
		}
	case state.Rebooting, state.ForceRebooting:
		return ""
	case state.RebootReady:
		handler.state.Set(state.Ready)
		return handler.beforeScriptExecution()
	case state.ShuttingDown:
		if handler.worker.onThreadShutdown != nil {
			handler.worker.onThreadShutdown(handler.thread.threadIndex)
		}
		handler.worker.detachThread(handler.thread)

		// signal to stop
		return ""
	default:
		panic("unexpected state: " + handler.state.Name())
	}
}

// setupScript marks the thread as a background worker on the C side and
// takes ownership of the stop pipe's write end.
func (handler *backgroundWorkerThread) setupScript() error {
	fd := int32(C.frankenphp_set_background_worker_and_get_stop_fd_write())
	if fd < 0 {
		return fmt.Errorf("failed to create the stop pipe of background worker %q", handler.worker.name)
	}
	handler.stopFdWrite.Store(fd)

	switch handler.state.Get() {
	case state.ShuttingDown, state.Rebooting, state.ForceRebooting:
		// a concurrent drain may have run before the fd was published;
		// close it now so the script observes EOF immediately
		handler.drain()
	}

	fc, err := newWorkerDummyContext(handler.worker)
	if err != nil {
		handler.drain()
		return err
	}
	handler.dummyFrankenPHPContext = fc

	metrics.StartWorker(handler.worker.name)
	// background workers have no later "reached steady state" marker like
	// frankenphp_handle_request; they count as ready once the script starts
	metrics.ReadyWorker(handler.worker.name)

	if fc.logger.Enabled(fc.ctx, slog.LevelDebug) {
		fc.logger.LogAttrs(fc.ctx, slog.LevelDebug, "starting background worker", slog.String("worker", handler.worker.name), slog.Int("thread", handler.thread.threadIndex))
	}

	if handler.state.Is(state.TransitionComplete) {
		handler.state.Set(state.Ready)
	}

	return nil
}

func (handler *backgroundWorkerThread) afterScriptExecution(exitStatus int) {
	// the write end of the stop pipe belongs to this thread; release it on
	// every exit path so the next run gets a fresh pipe (drain() already
	// took it when the exit was drain-triggered)
	handler.drain()
	worker := handler.worker
	handler.dummyFrankenPHPContext = nil

	// cooperative exit: the script is re-run with a reset backoff, unless
	// the thread is being drained (beforeScriptExecution checks the state)
	if exitStatus == 0 {
		metrics.StopWorker(worker.name, StopReasonRestart)

		if globalLogger.Enabled(globalCtx, slog.LevelDebug) {
			globalLogger.LogAttrs(globalCtx, slog.LevelDebug, "restarting background worker", slog.String("worker", worker.name), slog.Int("thread", handler.thread.threadIndex))
		}

		handler.failureCount = 0
		return
	}

	metrics.StopWorker(worker.name, StopReasonCrash)

	// max_consecutive_failures only fails hard during startup, where it
	// surfaces on startupFailChan so Init() returns the error to the
	// operator. Past startup, a crashing background worker keeps
	// restarting with a louder log line: silently giving up would leave
	// the server in a broken half-state with no clear way to recover.
	pastCap := worker.maxConsecutiveFailures >= 0 && handler.failureCount >= worker.maxConsecutiveFailures
	if pastCap && startupFailChan != nil && !watcherIsEnabled {
		startupFailChan <- fmt.Errorf("too many consecutive failures: background worker %s keeps crashing", worker.fileName)
		handler.thread.state.Set(state.ShuttingDown)
		return
	}

	logLevel := slog.LevelWarn
	logMsg := "background worker crashed, restarting"
	if pastCap {
		logLevel = slog.LevelError
		logMsg = "background worker exceeded max_consecutive_failures, still restarting"
	}
	if globalLogger.Enabled(globalCtx, logLevel) {
		globalLogger.LogAttrs(globalCtx, logLevel, logMsg, slog.String("worker", worker.name), slog.Int("thread", handler.thread.threadIndex), slog.Int("failures", handler.failureCount), slog.Int("exit_status", exitStatus))
	}

	handler.backoff()
}

// backoff waits before the next run of a crashed script: quadratic in the
// number of consecutive failures, capped at 1 second.
func (handler *backgroundWorkerThread) backoff() {
	backoffDuration := time.Duration(handler.failureCount*handler.failureCount*100) * time.Millisecond
	if backoffDuration > time.Second {
		backoffDuration = time.Second
	}
	handler.failureCount++
	time.Sleep(backoffDuration)
}
