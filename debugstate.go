package frankenphp

import (
	state "github.com/dunglas/frankenphp/internal/state"
)

// EXPERIMENTAL: ThreadDebugState prints the state of a single PHP thread - debugging purposes only
type ThreadDebugState struct {
	Index                    int
	Name                     string
	State                    string
	IsWaiting                bool
	IsBusy                   bool
	WaitingSinceMilliseconds int64
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
		if thread.state.Is(state.StateReserved) {
			fullState.ReservedThreadCount++
			continue
		}
		fullState.ThreadDebugStates = append(fullState.ThreadDebugStates, threadDebugState(thread))
	}

	return fullState
}

// threadDebugState creates a small jsonable status message for debugging purposes
func threadDebugState(thread *phpThread) ThreadDebugState {
	return ThreadDebugState{
		Index:                    thread.threadIndex,
		Name:                     thread.name(),
		State:                    thread.state.Name(),
		IsWaiting:                thread.state.IsInWaitingState(),
		IsBusy:                   !thread.state.IsInWaitingState(),
		WaitingSinceMilliseconds: thread.state.WaitTime(),
	}
}
