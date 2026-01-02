package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"context"
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

	state  *state.ThreadState
	thread *phpThread
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
	case state.Restarting:
		handler.state.Set(state.Yielding)
		handler.state.WaitFor(state.OpcacheResetting)
		scheduleOpcacheReset(handler.thread)
		handler.state.Set(state.OpcacheResettingDone)
		handler.state.WaitFor(state.Ready, state.ShuttingDown)
		return handler.beforeScriptExecution()

	case state.Ready:
		return handler.waitForRequest()

	case state.ShuttingDown:
		detachRegularThread(handler.thread)
		// signal to stop
		return ""
	}

	panic("unexpected state: " + handler.state.Name())
}

func (handler *regularThread) afterScriptExecution(_ int) {
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

func (handler *regularThread) waitForRequest() string {
	// clear any previously sandboxed env
	clearSandboxedEnv(handler.thread)

	handler.state.MarkAsWaiting(true)

	var ch contextHolder

	select {
	case <-handler.thread.drainChan:
		// go back to beforeScriptExecution
		return handler.beforeScriptExecution()
	case ch = <-regularRequestChan:
	case ch = <-handler.thread.requestChan:
	}

	handler.ctx = ch.ctx
	handler.contextHolder.frankenPHPContext = ch.frankenPHPContext
	handler.state.MarkAsWaiting(false)

	// set the scriptFilename that should be executed
	return handler.contextHolder.frankenPHPContext.scriptFilename
}

func (handler *regularThread) afterRequest() {
	handler.contextHolder.frankenPHPContext.closeContext()
	handler.contextHolder.frankenPHPContext = nil
	handler.ctx = nil
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
