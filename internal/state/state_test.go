package state

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test2GoroutinesYieldToEachOtherViaStates(t *testing.T) {
	threadState := &ThreadState{currentState: Booting}

	go func() {
		threadState.WaitFor(Inactive)
		assert.True(t, threadState.Is(Inactive))
		threadState.Set(Ready)
	}()

	threadState.Set(Inactive)
	threadState.WaitFor(Ready)
	assert.True(t, threadState.Is(Ready))
}

func TestStateShouldHaveCorrectAmountOfSubscribers(t *testing.T) {
	threadState := &ThreadState{currentState: Booting}

	// 3 subscribers waiting for different states
	go threadState.WaitFor(Inactive)
	go threadState.WaitFor(Inactive, ShuttingDown)
	go threadState.WaitFor(ShuttingDown)

	assertNumberOfSubscribers(t, threadState, 3)

	threadState.Set(Inactive)
	assertNumberOfSubscribers(t, threadState, 1)

	assert.True(t, threadState.CompareAndSwap(Inactive, ShuttingDown))
	assertNumberOfSubscribers(t, threadState, 0)
}

func assertNumberOfSubscribers(t *testing.T, threadState *ThreadState, expected int) {
	t.Helper()
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
