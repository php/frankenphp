package frankenphp

import (
	"context"
	"log/slog"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/dunglas/frankenphp/internal/state"
)

// representation of a non-worker PHP thread
// executes PHP scripts in a web context
// implements the threadHandler interface
type regularThread struct {
	contextHolder

	state        *state.ThreadState
	thread       *phpThread
	requestCount int
}

var (
	regularThreads       []*phpThread
	regularThreadMu      = &sync.RWMutex{}
	regularRequestChan   chan contextHolder
	queuedRegularThreads = atomic.Int32{}
)

func convertToRegularThread(thread *phpThread) {
	thread.setHandler(&regularThread{
		thread: thread,
		state:  thread.state,
	})
	attachRegularThread(thread)
}

// beforeScriptExecution returns the name of the script or an empty string on shutdown
func (handler *regularThread) beforeScriptExecution() string {
	switch handler.state.Get() {
	case state.TransitionRequested:
		detachRegularThread(handler.thread)
		return handler.thread.transitionToNewHandler()

	case state.TransitionComplete:
		handler.thread.updateContext(false)
		handler.state.Set(state.Ready)

		return handler.waitForRequest()

	case state.Ready:
		return handler.waitForRequest()

	case state.RebootReady:
		handler.requestCount = 0
		handler.state.Set(state.Ready)
		return handler.waitForRequest()

	case state.ShuttingDown:
		detachRegularThread(handler.thread)
		// signal to stop
		return ""
	}

	panic("unexpected state: " + handler.state.Name())
}

func (handler *regularThread) afterScriptExecution(_ int) {
	handler.thread.requestCount.Add(1)
	handler.afterRequest()
}

func (handler *regularThread) frankenPHPContext() *frankenPHPContext {
	return handler.contextHolder.frankenPHPContext
}

func (handler *regularThread) context() context.Context {
	return handler.ctx
}

func (handler *regularThread) name() string {
	return "Regular PHP Thread"
}

func (handler *regularThread) drain() {}

func (handler *regularThread) waitForRequest() string {
	// max_requests reached: restart the thread to clean up all ZTS state
	if maxRequestsPerThread > 0 && handler.requestCount >= maxRequestsPerThread {
		if globalLogger.Enabled(globalCtx, slog.LevelDebug) {
			globalLogger.LogAttrs(globalCtx, slog.LevelDebug, "max requests reached, restarting thread",
				slog.Int("thread", handler.thread.threadIndex),
				slog.Int("max_requests", maxRequestsPerThread),
			)
		}

		if handler.thread.reboot() {
			return ""
		}
	}

	handler.state.MarkAsWaiting(true)

	var ch contextHolder

	select {
	case <-handler.thread.drainChan:
		// go back to beforeScriptExecution
		return handler.beforeScriptExecution()
	case ch = <-regularRequestChan:
	case ch = <-handler.thread.requestChan:
	}

	handler.requestCount++
	handler.thread.contextMu.Lock()
	handler.ctx = ch.ctx
	handler.contextHolder.frankenPHPContext = ch.frankenPHPContext
	handler.thread.contextMu.Unlock()
	handler.state.MarkAsWaiting(false)

	// set the scriptFilename that should be executed
	return handler.contextHolder.frankenPHPContext.scriptFilename
}

func (handler *regularThread) afterRequest() {
	handler.contextHolder.frankenPHPContext.closeContext()
	handler.thread.contextMu.Lock()
	handler.contextHolder.frankenPHPContext = nil
	handler.ctx = nil
	handler.thread.contextMu.Unlock()
}

func handleRequestWithRegularPHPThreads(ch contextHolder) error {
	metrics.StartRequest()

	runtime.Gosched()

	if queuedRegularThreads.Load() == 0 {
		regularThreadMu.RLock()
		for _, thread := range regularThreads {
			select {
			case thread.requestChan <- ch:
				regularThreadMu.RUnlock()
				<-ch.frankenPHPContext.done
				metrics.StopRequest()

				return nil
			default:
				// thread was not available
			}
		}
		regularThreadMu.RUnlock()
	}

	// if no thread was available, mark the request as queued and fan it out to all threads
	queuedRegularThreads.Add(1)
	metrics.QueuedRequest()

	for {
		select {
		case regularRequestChan <- ch:
			queuedRegularThreads.Add(-1)
			metrics.DequeuedRequest()

			<-ch.frankenPHPContext.done
			metrics.StopRequest()

			return nil
		case scaleChan <- ch.frankenPHPContext:
			// the request has triggered scaling, continue to wait for a thread
		case <-timeoutChan(maxWaitTime):
			// the request has timed out stalling
			queuedRegularThreads.Add(-1)
			metrics.DequeuedRequest()
			metrics.StopRequest()

			ch.frankenPHPContext.reject(ErrMaxWaitTimeExceeded)

			return ErrMaxWaitTimeExceeded
		}
	}
}

func attachRegularThread(thread *phpThread) {
	regularThreadMu.Lock()
	regularThreads = append(regularThreads, thread)
	regularThreadMu.Unlock()
}

func detachRegularThread(thread *phpThread) {
	regularThreadMu.Lock()
	for i, t := range regularThreads {
		if t == thread {
			regularThreads = append(regularThreads[:i], regularThreads[i+1:]...)
			break
		}
	}
	regularThreadMu.Unlock()
}
