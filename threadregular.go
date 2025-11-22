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

	metrics.QueuedRequest()

	// Acquire semaphore with the appropriate strategy based on maxWaitTime and autoscaling
	if maxWaitTime > 0 && scaleChan != nil {
		// Both maxWaitTime and autoscaling enabled.
		// Try with minStallTime first to trigger autoscaling, then enforce maxWaitTime.
		// We just assume the operator is sane and minStallTime is less than maxWaitTime.
		ctx, cancel := context.WithTimeout(context.Background(), minStallTime)
		err := regularSemaphore.Acquire(ctx, 1)
		cancel()

		if err != nil {
			// Timed out after minStallTime: signal autoscaling
			select {
			case scaleChan <- ch.frankenPHPContext:
			default:
				// scaleChan full, autoscaling already in progress
			}

			// Continue trying with maxWaitTime limit
			ctx, cancel := context.WithTimeout(context.Background(), maxWaitTime)
			defer cancel()

			if err := regularSemaphore.Acquire(ctx, 1); err != nil {
				ch.frankenPHPContext.reject(ErrMaxWaitTimeExceeded)
				metrics.StopRequest()
				return ErrMaxWaitTimeExceeded
			}
		}
		defer regularSemaphore.Release(1)
	} else if maxWaitTime > 0 {
		// Only maxWaitTime enabled, no autoscaling
		ctx, cancel := context.WithTimeout(context.Background(), maxWaitTime)
		defer cancel()

		if err := regularSemaphore.Acquire(ctx, 1); err != nil {
			ch.frankenPHPContext.reject(ErrMaxWaitTimeExceeded)
			metrics.StopRequest()
			return ErrMaxWaitTimeExceeded
		}
		defer regularSemaphore.Release(1)
	} else if scaleChan != nil {
		// Only autoscaling enabled, no maxWaitTime
		ctx, cancel := context.WithTimeout(context.Background(), minStallTime)
		err := regularSemaphore.Acquire(ctx, 1)
		cancel()

		if err != nil {
			// Timed out: signal autoscaling
			select {
			case scaleChan <- ch.frankenPHPContext:
			default:
				// scaleChan full, autoscaling already in progress
			}

			if err := regularSemaphore.Acquire(context.Background(), 1); err != nil {
				ch.frankenPHPContext.reject(ErrMaxWaitTimeExceeded)
				metrics.StopRequest()
				return ErrMaxWaitTimeExceeded
			}
		}
		defer regularSemaphore.Release(1)
	} else {
		// No maxWaitTime, no autoscaling: block indefinitely
		if err := regularSemaphore.Acquire(context.Background(), 1); err != nil {
			// Should never happen with Background context
			ch.frankenPHPContext.reject(ErrMaxWaitTimeExceeded)
			metrics.StopRequest()
			return ErrMaxWaitTimeExceeded
		}
		defer regularSemaphore.Release(1)
	}

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
