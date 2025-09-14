package frankenphp

import (
	"time"
)

// representation of a non-worker PHP thread
// executes PHP scripts in a web context
// implements the threadHandler interface
type regularThread struct {
	state          *threadState
	thread         *phpThread
	requestContext *frankenPHPContext
}

var regularThreadPool *threadPool

func initRegularPHPThreads(num int) {
	regularThreadPool = newThreadPool(num)
	for i := 0; i < num; i++ {
		convertToRegularThread(getInactivePHPThread())
	}
}

func convertToRegularThread(thread *phpThread) {
	thread.setHandler(&regularThread{
		thread: thread,
		state:  thread.state,
	})
	regularThreadPool.attach(thread)
}

// beforeScriptExecution returns the name of the script or an empty string on shutdown
func (handler *regularThread) beforeScriptExecution() string {
	switch handler.state.get() {
	case stateTransitionRequested:
		regularThreadPool.detach(handler.thread)
		return handler.thread.transitionToNewHandler()
	case stateTransitionComplete:
		handler.state.set(stateReady)
		return handler.waitForRequest()
	case stateReady:
		return handler.waitForRequest()
	case stateShuttingDown:
		regularThreadPool.detach(handler.thread)
		// signal to stop
		return ""
	}
	panic("unexpected state: " + handler.state.name())
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
	thread := handler.thread
	clearSandboxedEnv(thread) // clear any previously sandboxed env

	handler.state.markAsWaiting(true)

	var fc *frankenPHPContext
	select {
	case <-thread.drainChan:
		// go back to beforeScriptExecution
		return handler.beforeScriptExecution()
	case fc = <-thread.requestChan:
	case fc = <-regularThreadPool.requestChan(thread):
	}

	handler.requestContext = fc
	handler.state.markAsWaiting(false)

	// set the scriptFilename that should be executed
	return fc.scriptFilename
}

func (handler *regularThread) afterRequest() {
	handler.requestContext.closeContext()
	handler.requestContext = nil
}

func handleRequestWithRegularPHPThreads(fc *frankenPHPContext) {
	metrics.StartRequest()

	trackLatency := latencyTrackingEnabled.Load()
	isSlowRequest := trackLatency && isHighLatencyRequest(fc)

	// dispatch requests to all regular threads in order
	if !isSlowRequest && regularThreadPool.dispatchRequest(fc) {
		<-fc.done
		metrics.StopRequest()
		trackRequestLatency(fc, time.Since(fc.startedAt), false)

		return
	}

	metrics.QueuedRequest()
	requestWasReceived := regularThreadPool.queueRequest(fc, trackLatency && !isSlowRequest)
	metrics.DequeuedRequest()

	if !requestWasReceived {
		return
	}

	stallTime := time.Since(fc.startedAt)
	<-fc.done
	metrics.StopRequest()
	trackRequestLatency(fc, time.Since(fc.startedAt)-stallTime, isSlowRequest)
}
