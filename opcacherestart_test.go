package frankenphp

import (
	"bytes"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOpcacheRestartsWithinTheCooldownAreCoalesced(t *testing.T) {
	var buf bytes.Buffer
	prevLogger, prevCooldown := globalLogger, opcacheRestartCooldown
	globalLogger, opcacheRestartCooldown = slog.New(slog.NewTextHandler(&buf, nil)), 100*time.Millisecond
	resetOpcacheRestarts()
	t.Cleanup(func() {
		resetOpcacheRestarts()
		globalLogger, opcacheRestartCooldown = prevLogger, prevCooldown
	})

	round := opcacheRestarts.round.Load()
	wake := opcacheWakeChan()

	restartWorkersForOpcache()
	assert.Equal(t, round+1, opcacheRestarts.round.Load(), "the first restart must restart the workers right away")
	select {
	case <-wake:
	default:
		t.Error("the idle workers must be woken up")
	}

	restartWorkersForOpcache()
	restartWorkersForOpcache()
	assert.Equal(t, round+1, opcacheRestarts.round.Load(), "restarts within the cooldown must be delayed")
	assert.Contains(t, buf.String(), `level=WARN msg="opcache restarts are too frequent`)

	assert.Eventually(t, func() bool { return opcacheRestarts.round.Load() == round+2 }, time.Second, 10*time.Millisecond)
	time.Sleep(2 * opcacheRestartCooldown)
	assert.Equal(t, round+2, opcacheRestarts.round.Load(), "delayed restarts must make a single round")
}
