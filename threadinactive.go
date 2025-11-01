package frankenphp

import (
	"github.com/dunglas/frankenphp/internal/state"
)

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

	switch thread.state.Get() {
	case state.TransitionRequested:
		return thread.transitionToNewHandler()
	case state.Booting, state.TransitionComplete:
		thread.state.Set(state.Inactive)

		// wait for external signal to start or shut down
		thread.state.MarkAsWaiting(true)
		thread.state.WaitFor(state.TransitionRequested, state.ShuttingDown)
		thread.state.MarkAsWaiting(false)
		return handler.beforeScriptExecution()
	case state.ShuttingDown:
		// signal to stop
		return ""
	}
	panic("unexpected state: " + thread.state.Name())
}

func (handler *inactiveThread) afterScriptExecution(int) {
	panic("inactive threads should not execute scripts")
}

func (handler *inactiveThread) getRequestContext() *frankenPHPContext {
	return nil
}

func (handler *inactiveThread) name() string {
	return "Inactive PHP Thread"
}
