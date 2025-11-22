package frankenphp

import (
	"context"
	"runtime"
	"sync"

	"golang.org/x/sync/semaphore"
)

// representation of a non-worker PHP thread
// executes PHP scripts in a web context
// implements the threadHandler interface
type regularThread struct {
	contextHolder

	state  *threadState
	thread *phpThread
}

var (
	regularThreads     []*phpThread
	regularThreadMu    = &sync.RWMutex{}
	regularRequestChan chan contextHolder
	regularSemaphore   *semaphore.Weighted // FIFO admission control
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

	handler.state.markAsWaiting(true)

	var ch contextHolder

	select {
	case <-handler.thread.drainChan:
		// go back to beforeScriptExecution
		return handler.beforeScriptExecution()
	case ch = <-regularRequestChan:
	}

	handler.ctx = ch.ctx
	handler.contextHolder.frankenPHPContext = ch.frankenPHPContext
	handler.state.markAsWaiting(false)

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

	// yield to ensure this goroutine doesn't end up on the same P queue
	runtime.Gosched()

	// Enforce FIFO ordering of requests
	if err := regularSemaphore.Acquire(ch.ctx, 1); err != nil {
		ch.frankenPHPContext.reject(err)
		return err
	}
	defer regularSemaphore.Release(1)

	select {
	case regularRequestChan <- ch:
		// a thread was available to handle the request immediately
		<-ch.frankenPHPContext.done
		metrics.StopRequest()

		return nil
	default:
		// no thread was available
	}

	// if no thread was available, mark the request as queued and fan it out to all threads
	metrics.QueuedRequest()
	for {
		select {
		case regularRequestChan <- ch:
			metrics.DequeuedRequest()
			<-ch.frankenPHPContext.done
			metrics.StopRequest()

			return nil
		case scaleChan <- ch.frankenPHPContext:
			// the request has triggered scaling, continue to wait for a thread
		case <-timeoutChan(maxWaitTime):
			// the request has timed out stalling
			metrics.DequeuedRequest()

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
