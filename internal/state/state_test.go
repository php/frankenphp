package frankenphp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test2GoroutinesYieldToEachOtherViaStates(t *testing.T) {
	threadState := &threadState{currentState: stateBooting}

	go func() {
		threadState.WaitFor(stateInactive)
		assert.True(t, threadState.is(stateInactive))
		threadstate.Set(stateReady)
	}()

	threadstate.Set(stateInactive)
	threadState.WaitFor(stateReady)
	assert.True(t, threadState.is(stateReady))
}

func TestStateShouldHaveCorrectAmountOfSubscribers(t *testing.T) {
	threadState := &threadState{currentState: stateBooting}

	// 3 subscribers waiting for different states
	go threadState.WaitFor(stateInactive)
	go threadState.WaitFor(stateInactive, StateShuttingDown)
	go threadState.WaitFor(StateShuttingDown)

	assertNumberOfSubscribers(t, threadState, 3)

	threadstate.Set(stateInactive)
	assertNumberOfSubscribers(t, threadState, 1)

	assert.True(t, threadstate.CompareAndSwap(stateInactive, stateShuttingDown))
	assertNumberOfSubscribers(t, threadState, 0)
}

func assertNumberOfSubscribers(t *testing.T, threadState *threadState, expected int) {
	for range 10_000 { // wait for 1 second max
		time.Sleep(100 * time.Microsecond)
		threadState.mu.RLock()
		if len(threadState.subscribers) == expected {
			threadState.mu.RUnlock()
			break
		}
		threadState.mu.RUnlock()
	}
	threadState.mu.RLock()
	assert.Len(t, threadState.subscribers, expected)
	threadState.mu.RUnlock()
}
