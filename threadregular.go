package frankenphp

import (
	"sync"

	"github.com/dunglas/frankenphp/internal/state"
)

// representation of a non-worker PHP thread
// executes PHP scripts in a web context
// implements the threadHandler interface
type regularThread struct {
	state          *state.ThreadState
	thread         *phpThread
	requestContext *frankenPHPContext
}

var (
	regularThreads     []*phpThread
	regularThreadMu    = &sync.RWMutex{}
	regularRequestChan chan *frankenPHPContext
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
	case state.ShuttingDown:
		detachRegularThread(handler.thread)
		// signal to stop
		return ""
	}
	panic("unexpected state: " + handler.state.Name())
}

func (handler *regularThread) afterScriptExecution(int) {
	handler.afterRequest()
}

func (handler *regularThread) getRequestContext() *frankenPHPContext {
	return handler.requestContext
}

func (handler *regularThread) name() string {
	return "Regular PHP Thread"
}

func (handler *regularThread) waitForRequest() string {
	// clear any previously sandboxed env
	clearSandboxedEnv(handler.thread)

	handler.state.MarkAsWaiting(true)

	var fc *frankenPHPContext
	select {
	case <-handler.thread.drainChan:
		// go back to beforeScriptExecution
		return handler.beforeScriptExecution()
	case fc = <-regularRequestChan:
	}

	handler.requestContext = fc
	handler.state.MarkAsWaiting(false)

	// set the scriptFilename that should be executed
	return fc.scriptFilename
}

func (handler *regularThread) afterRequest() {
	handler.requestContext.closeContext()
	handler.requestContext = nil
}

func handleRequestWithRegularPHPThreads(fc *frankenPHPContext) {
	metrics.StartRequest()
	select {
	case regularRequestChan <- fc:
		// a thread was available to handle the request immediately
		<-fc.done
		metrics.StopRequest()
		return
	default:
		// no thread was available
	}

	// if no thread was available, mark the request as queued and fan it out to all threads
	metrics.QueuedRequest()
	for {
		select {
		case regularRequestChan <- fc:
			metrics.DequeuedRequest()
			<-fc.done
			metrics.StopRequest()
			return
		case scaleChan <- fc:
			// the request has triggered scaling, continue to wait for a thread
		case <-timeoutChan(maxWaitTime):
			// the request has timed out stalling
			metrics.DequeuedRequest()
			fc.reject(504, "Gateway Timeout")
			return
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
