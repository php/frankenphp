package backoff

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestExponentialBackoff_Reset(t *testing.T) {
	e := &ExponentialBackoff{
		MaxBackoff:             5 * time.Second,
		MinBackoff:             500 * time.Millisecond,
		MaxConsecutiveFailures: 3,
	}

	assert.False(t, e.RecordFailure())
	assert.False(t, e.RecordFailure())
	e.RecordSuccess()

	e.mu.RLock()
	defer e.mu.RUnlock()
	assert.Equal(t, 0, e.failureCount, "expected failureCount to be reset to 0")
	assert.Equal(t, e.backoff, e.MinBackoff, "expected backoff to be reset to MinBackoff")
}

func TestExponentialBackoff_Trigger(t *testing.T) {
	e := &ExponentialBackoff{
		MaxBackoff:             500 * 3 * time.Millisecond,
		MinBackoff:             500 * time.Millisecond,
		MaxConsecutiveFailures: 3,
	}

	assert.False(t, e.RecordFailure())
	assert.False(t, e.RecordFailure())
	assert.True(t, e.RecordFailure())

	e.mu.RLock()
	defer e.mu.RUnlock()
	assert.Equal(t, e.failureCount, e.MaxConsecutiveFailures, "expected failureCount to be MaxConsecutiveFailures")
	assert.Equal(t, e.backoff, e.MaxBackoff, "expected backoff to be MaxBackoff")
}
