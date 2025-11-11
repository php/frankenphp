package frankenphp

import (
	"context"
	"sync"
)

// representation of a non-worker PHP thread
// executes PHP scripts in a web context
// implements the threadHandler interface
type regularThread struct {
	state  *threadState
	thread *phpThread
	ctx    context.Context
}

var (
	regularThreads     []*phpThread
	regularThreadMu    = &sync.RWMutex{}
	regularRequestChan chan context.Context
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
	switch handler.state.get() {
	case stateTransitionRequested:
		detachRegularThread(handler.thread)
		return handler.thread.transitionToNewHandler()
	case stateTransitionComplete:
		handler.state.set(stateReady)
		return handler.waitForRequest()
	case stateReady:
		return handler.waitForRequest()
	case stateShuttingDown:
		detachRegularThread(handler.thread)
		// signal to stop
		return ""
	}
	panic("unexpected state: " + handler.state.name())
}

func (handler *regularThread) afterScriptExecution(int) {
	handler.afterRequest()
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

	handler.state.markAsWaiting(true)

	var ctx context.Context
	select {
	case <-handler.thread.drainChan:
		// go back to beforeScriptExecution
		return handler.beforeScriptExecution()
	case ctx = <-regularRequestChan:
	}

	handler.ctx = ctx
	handler.state.markAsWaiting(false)

	// set the scriptFilename that should be executed
	return ctx.Value(contextKey).(*frankenPHPContext).scriptFilename
}

func (handler *regularThread) afterRequest() {
	fc := handler.ctx.Value(contextKey).(*frankenPHPContext)

	fc.closeContext()
	handler.ctx = nil
}

func handleRequestWithRegularPHPThreads(ctx context.Context) error {
	metrics.StartRequest()

	fc := ctx.Value(contextKey).(*frankenPHPContext)

	select {
	case regularRequestChan <- ctx:
		// a thread was available to handle the request immediately
		<-fc.done
		metrics.StopRequest()

		return nil
	default:
		// no thread was available
	}

	// if no thread was available, mark the request as queued and fan it out to all threads
	metrics.QueuedRequest()
	for {
		select {
		case regularRequestChan <- ctx:
			metrics.DequeuedRequest()
			<-fc.done
			metrics.StopRequest()

			return nil
		case scaleChan <- fc:
			// the request has triggered scaling, continue to wait for a thread
		case <-timeoutChan(maxWaitTime):
			// the request has timed out stalling
			metrics.DequeuedRequest()

			fc.reject(ErrMaxWaitTimeExceeded)

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
