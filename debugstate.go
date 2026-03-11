package frankenphp

import (
	"github.com/dunglas/frankenphp/internal/state"
)

// EXPERIMENTAL: ThreadDebugState prints the state of a single PHP thread - debugging purposes only
type ThreadDebugState struct {
	Index                    int
	Name                     string
	State                    string
	IsWaiting                bool
	IsBusy                   bool
	WaitingSinceMilliseconds int64
	CurrentURI               string
	CurrentMethod            string
	RequestStartedAt         int64
	RequestCount             int64
	MemoryUsage              int64
}

// EXPERIMENTAL: FrankenPHPDebugState prints the state of all PHP threads - debugging purposes only
type FrankenPHPDebugState struct {
	ThreadDebugStates   []ThreadDebugState
	ReservedThreadCount int
}

// EXPERIMENTAL: DebugState prints the state of all PHP threads - debugging purposes only
func DebugState() FrankenPHPDebugState {
	fullState := FrankenPHPDebugState{
		ThreadDebugStates:   make([]ThreadDebugState, 0, len(phpThreads)),
		ReservedThreadCount: 0,
	}
	for _, thread := range phpThreads {
		if thread.state.Is(state.Reserved) {
			fullState.ReservedThreadCount++
			continue
		}
		fullState.ThreadDebugStates = append(fullState.ThreadDebugStates, threadDebugState(thread))
	}

	return fullState
}

// threadDebugState creates a small jsonable status message for debugging purposes
func threadDebugState(thread *phpThread) ThreadDebugState {
	isBusy := !thread.state.IsInWaitingState()

	s := ThreadDebugState{
		Index:                    thread.threadIndex,
		Name:                     thread.name(),
		State:                    thread.state.Name(),
		IsWaiting:                thread.state.IsInWaitingState(),
		IsBusy:                   isBusy,
		WaitingSinceMilliseconds: thread.state.WaitTime(),
	}

	s.RequestCount = thread.requestCount.Load()
	s.MemoryUsage = thread.lastMemoryUsage.Load()

	if isBusy {
		thread.handlerMu.RLock()
		fc := thread.handler.frankenPHPContext()
		thread.handlerMu.RUnlock()

		if fc != nil && fc.request != nil && fc.responseWriter != nil {
			if fc.originalRequest != nil {
				s.CurrentURI = fc.originalRequest.URL.RequestURI()
				s.CurrentMethod = fc.originalRequest.Method
			} else {
				s.CurrentURI = fc.requestURI
				s.CurrentMethod = fc.request.Method
			}
			if !fc.startedAt.IsZero() {
				s.RequestStartedAt = fc.startedAt.UnixMilli()
			}
		}
	}

	return s
}
