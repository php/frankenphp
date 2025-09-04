package frankenphp

// #include "frankenphp.h"
import "C"

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
		if thread.state.is(stateReserved) {
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
		State:                    thread.state.name(),
		IsWaiting:                thread.state.isInWaitingState(),
		IsBusy:                   !thread.state.isInWaitingState(),
		WaitingSinceMilliseconds: thread.state.waitTime(),
	}
}

// EXPERIMENTAL: Expose the current thread's information to PHP
//
//export go_frankenphp_info
func go_frankenphp_info(threadIndex C.uintptr_t) *C.zval {
	currentThread := phpThreads[threadIndex]
	_, isWorker := currentThread.handler.(*workerThread)

	threadInfos := make([]any, 0, len(phpThreads))
	for _, thread := range phpThreads {
		if thread.state.is(stateReserved) {
			continue
		}
		threadInfos = append(threadInfos, map[string]any{
			"index":                      thread.threadIndex,
			"name":                       thread.name(),
			"state":                      thread.state.name(),
			"is_waiting":                 thread.state.isInWaitingState(),
			"waiting_since_milliseconds": thread.state.waitTime(),
		})
	}

	zval := (*C.zval)(PHPMap(map[string]any{
		"frankenphp_version": C.GoString(C.frankenphp_get_version().frankenphp_version),
		"current_thread":     int64(threadIndex),
		"is_worker_thread":   isWorker,
		"threads":            threadInfos,
	}))

	// TODO: how to circumvent pinning?
	currentThread.Pin(zval)

	return zval
}
