package frankenphp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test2GoroutinesYieldToEachOtherViaStates(t *testing.T) {
	threadState := &threadState{currentState: stateBooting}

	go func() {
		threadState.waitFor(stateInactive)
		assert.True(t, threadState.is(stateInactive))
		threadState.set(stateReady)
	}()

	threadState.set(stateInactive)
	threadState.waitFor(stateReady)
	assert.True(t, threadState.is(stateReady))
}

func TestStateShouldHaveCorrectAmountOfSubscribers(t *testing.T) {
	threadState := &threadState{currentState: stateBooting}

	// 3 subscribers waiting for different states
	go threadState.waitFor(stateInactive)
	go threadState.waitFor(stateInactive, stateShuttingDown)
	go threadState.waitFor(stateShuttingDown)

	assertNumberOfSubscribers(t, threadState, 3)

	threadState.set(stateInactive)
	assertNumberOfSubscribers(t, threadState, 1)

	assert.True(t, threadState.compareAndSwap(stateInactive, stateShuttingDown))
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
