package backoff

import (
	"sync"
	"time"
)

type ExponentialBackoff struct {
	backoff                time.Duration
	failureCount           int
	mu                     sync.RWMutex
	MaxBackoff             time.Duration
	MinBackoff             time.Duration
	MaxConsecutiveFailures int
}

// recordSuccess resets the backoff and failureCount
func (e *ExponentialBackoff) RecordSuccess() {
	e.mu.Lock()
	e.failureCount = 0
	e.backoff = e.MinBackoff
	e.mu.Unlock()
}

// recordFailure increments the failure count and increases the backoff, it returns true if MaxConsecutiveFailures has been reached
func (e *ExponentialBackoff) RecordFailure() bool {
	e.mu.Lock()
	e.failureCount += 1
	if e.backoff < e.MinBackoff {
		e.backoff = e.MinBackoff
	}

	e.backoff = min(e.backoff*2, e.MaxBackoff)

	e.mu.Unlock()
	return e.MaxConsecutiveFailures != -1 && e.failureCount >= e.MaxConsecutiveFailures
}

// wait sleeps for the backoff duration if failureCount is non-zero.
// NOTE: this is not tested and should be kept 'obviously correct' (i.e., simple)
func (e *ExponentialBackoff) Wait() {
	e.mu.RLock()
	if e.failureCount == 0 {
		e.mu.RUnlock()

		return
	}
	e.mu.RUnlock()

	time.Sleep(e.backoff)
}

func (e *ExponentialBackoff) FailureCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.failureCount
}