package frankenphp

import (
	"context"
	"sync"

	"golang.org/x/sync/semaphore"
)

// representation of a non-worker PHP thread
// executes PHP scripts in a web context
// implements the threadHandler interface
type regularThread struct {
	contextHolder

	state     *threadState
	thread    *phpThread
	workReady chan contextHolder // Channel to receive work directly
}

var (
	regularThreads     []*phpThread
	regularThreadMu    = &sync.RWMutex{}
	regularRequestChan chan contextHolder
	regularSemaphore   *semaphore.Weighted // FIFO admission control
	regularThreadPool  sync.Pool           // Pool of idle threads for direct handoff
)

func convertToRegularThread(thread *phpThread) {
	thread.setHandler(&regularThread{
		thread:    thread,
		state:     thread.state,
		workReady: make(chan contextHolder, 1), // Buffered to avoid blocking sender
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

	// Put this thread in the pool for direct handoff
	regularThreadPool.Put(handler)

	// Wait for work to be assigned (either via pool or fallback channel)
	var ch contextHolder
	select {
	case <-handler.thread.drainChan:
		// go back to beforeScriptExecution
		return handler.beforeScriptExecution()
	case ch = <-handler.workReady:
		// Work received via direct handoff from the pool
	case ch = <-regularRequestChan:
		// Fallback: work came via global channel
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
	metrics.QueuedRequest()

	if err := acquireSemaphoreWithAdmissionControl(regularSemaphore, scaleChan, ch.frankenPHPContext); err != nil {
		ch.frankenPHPContext.reject(err)
		metrics.StopRequest()
		return err
	}
	defer regularSemaphore.Release(1)

	// Fast path: try to get an idle thread from the pool
	if idle := regularThreadPool.Get(); idle != nil {
		handler := idle.(*regularThread)
		// Send work to the thread's dedicated channel
		handler.workReady <- ch
		metrics.DequeuedRequest()
		<-ch.frankenPHPContext.done
		metrics.StopRequest()
		return nil
	}

	// Slow path: no idle thread in pool, use the global channel
	regularRequestChan <- ch
	metrics.DequeuedRequest()
	<-ch.frankenPHPContext.done
	metrics.StopRequest()

	return nil
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
