package frankenphp

import "github.com/dunglas/frankenphp/internal/state"

// workerLifecycle is the part of a worker thread that HTTP and background
// workers share: the thread, its worker, and the states both walk through
// between two runs of the script. Handlers embed it and supply what differs,
// see workerHandler.
type workerLifecycle struct {
	state  *state.ThreadState
	thread *phpThread
	worker *worker
}

// workerHandler is a threadHandler running a worker script, plus the step
// the shared lifecycle delegates
type workerHandler interface {
	threadHandler
	// startScript prepares a run and returns the script to execute, or an
	// empty string to stop the thread
	startScript() string
}

func newWorkerLifecycle(thread *phpThread, worker *worker) workerLifecycle {
	return workerLifecycle{state: thread.state, thread: thread, worker: worker}
}

// beforeScriptExecution returns the name of the script to run, or an empty
// string to stop the thread
func (l *workerLifecycle) beforeScriptExecution(handler workerHandler) string {
	switch l.state.Get() {
	case state.TransitionRequested:
		l.detach()

		return l.thread.transitionToNewHandler()
	case state.Ready, state.TransitionComplete:
		l.thread.updateContext(true)
		if l.worker.onThreadReady != nil {
			l.worker.onThreadReady(l.thread.threadIndex)
		}

		return handler.startScript()
	case state.Rebooting, state.ForceRebooting:
		return ""
	case state.RebootReady:
		l.state.Set(state.Ready)

		return handler.beforeScriptExecution()
	case state.ShuttingDown:
		l.detach()

		// signal to stop
		return ""
	default:
		panic("unexpected state: " + l.state.Name())
	}
}

// detach takes the thread off its worker, on the paths that stop running its
// script for good
func (l *workerLifecycle) detach() {
	if l.worker.onThreadShutdown != nil {
		l.worker.onThreadShutdown(l.thread.threadIndex)
	}
	l.worker.detachThread(l.thread)
}
