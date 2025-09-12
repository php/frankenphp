package frankenphp

import (
	"sync"
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

var (
	regularThreads     []*phpThread
	regularThreadMu    = sync.RWMutex{}
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
	case fc = <-regularRequestChan:
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

	if latencyTrackingEnabled.Load() && isHighLatencyRequest(fc) {
		regularThreadMu.RLock()
		slowThread := getRandomSlowThread(regularThreads)
		regularThreadMu.RUnlock()
		stallRegularPHPRequests(fc, slowThread.requestChan, true)

		return
	}

	regularThreadMu.RLock()
	for _, thread := range regularThreads {
		select {
		case thread.requestChan <- fc:
			regularThreadMu.RUnlock()
			<-fc.done
			metrics.StopRequest()
			trackRequestLatency(fc, time.Since(fc.startedAt), false)

			return
		default:
			// thread is busy, continue
		}
	}
	regularThreadMu.RUnlock()

	// if no thread was available, mark the request as queued and fan it out to all threads
	stallRegularPHPRequests(fc, regularRequestChan, false)
}

// stall the request and trigger scaling or timeouts
func stallRegularPHPRequests(fc *frankenPHPContext, requestChan chan *frankenPHPContext, forceTracking bool) {
	metrics.QueuedRequest()
	for {
		select {
		case requestChan <- fc:
			metrics.DequeuedRequest()
			stallTime := time.Since(fc.startedAt)
			<-fc.done
			metrics.StopRequest()
			trackRequestLatency(fc, time.Since(fc.startedAt)-stallTime, forceTracking)
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
