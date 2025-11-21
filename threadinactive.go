package frankenphp

import "context"

// representation of a thread with no work assigned to it
// implements the threadHandler interface
// each inactive thread weighs around ~350KB
// keeping threads at 'inactive' will consume more memory, but allow a faster transition
type inactiveThread struct {
	thread *phpThread
}

func convertToInactiveThread(thread *phpThread) {
	thread.setHandler(&inactiveThread{thread: thread})
}

func (handler *inactiveThread) beforeScriptExecution() string {
	thread := handler.thread

	switch thread.state.get() {
	case stateTransitionRequested:
		return thread.transitionToNewHandler()

	case stateBooting, stateTransitionComplete:
		thread.state.set(stateInactive)

		// wait for external signal to start or shut down
		thread.state.markAsWaiting(true)
		thread.state.waitFor(stateTransitionRequested, stateShuttingDown)
		thread.state.markAsWaiting(false)

		return handler.beforeScriptExecution()

	case stateShuttingDown:
		// signal to stop
		return ""
	}

	panic("unexpected state: " + thread.state.name())
}

func (handler *inactiveThread) afterScriptExecution(int) {
	panic("inactive threads should not execute scripts")
}

func (handler *inactiveThread) frankenPHPContext() *frankenPHPContext {
	return nil
}

func (handler *inactiveThread) context() context.Context {
	return globalCtx
}

func (handler *inactiveThread) name() string {
	return "Inactive PHP Thread"
}
