package frankenphp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAutomaticOpcacheRestartsAreReportedOncePerWindow(t *testing.T) {
	reset := func() {
		automaticOpcacheRestarts.Lock()
		automaticOpcacheRestarts.windowStart = time.Time{}
		automaticOpcacheRestarts.count = 0
		automaticOpcacheRestarts.Unlock()
	}

	t.Run("stays quiet below the threshold", func(t *testing.T) {
		reset()
		now := time.Now()
		for i := 1; i < automaticOpcacheRestartThreshold; i++ {
			assert.False(t, recordAutomaticOpcacheRestart(now), "restart %d must not report", i)
		}
	})

	t.Run("reports exactly once when the threshold is crossed", func(t *testing.T) {
		reset()
		now := time.Now()
		reported := 0
		for i := 0; i < automaticOpcacheRestartThreshold*3; i++ {
			if recordAutomaticOpcacheRestart(now) {
				reported++
			}
		}
		assert.Equal(t, 1, reported, "a burst must be reported once, not on every restart")
	})

	t.Run("restarts spread out never reach the threshold", func(t *testing.T) {
		reset()
		now := time.Now()
		for i := 0; i < automaticOpcacheRestartThreshold*3; i++ {
			now = now.Add(automaticOpcacheRestartWindow + time.Second)
			assert.False(t, recordAutomaticOpcacheRestart(now), "occasional restarts are normal")
		}
	})
}
