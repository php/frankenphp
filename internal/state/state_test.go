package state

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test2GoroutinesYieldToEachOtherViaStates(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		threadState := &ThreadState{currentState: Booting}

		go func() {
			threadState.WaitFor(Inactive)
			assert.True(t, threadState.Is(Inactive))
			threadState.Set(Ready)
		}()

		threadState.Set(Inactive)
		threadState.WaitFor(Ready)
		assert.True(t, threadState.Is(Ready))
	})
}

func TestStateShouldHaveCorrectAmountOfSubscribers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
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
	})
}

func TestWaitForStateWithTimeoutGivesUpAndDropsItsSubscriber(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		threadState := &ThreadState{currentState: Booting}

		// the fake clock makes the timeout fire instantly instead of after a real second
		assert.False(t, threadState.WaitForStateWithTimeout(time.Second, Ready))
		assert.Empty(t, threadState.subscribers)
	})
}

func assertNumberOfSubscribers(t *testing.T, threadState *ThreadState, expected int) {
	t.Helper()

	// every subscriber goroutine is durably blocked on its channel once Wait returns,
	// so the subscriber list has reached its final shape for this state
	synctest.Wait()

	threadState.mu.RLock()
	defer threadState.mu.RUnlock()

	assert.Len(t, threadState.subscribers, expected)
}
