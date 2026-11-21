package frankenphp

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRestartBackoff(t *testing.T) {
	assert.Equal(t, time.Duration(0), restartBackoff(0))
	assert.Equal(t, 100*time.Millisecond, restartBackoff(1))
	assert.Equal(t, 400*time.Millisecond, restartBackoff(2))
	assert.Equal(t, 900*time.Millisecond, restartBackoff(3))
	assert.Equal(t, time.Second, restartBackoff(4))
	// a crash loop counts without bound, the quadratic must not overflow
	assert.Equal(t, time.Second, restartBackoff(303701))
	assert.Equal(t, time.Second, restartBackoff(math.MaxInt32))
}
