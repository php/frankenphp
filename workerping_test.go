package frankenphp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNextAlignedPing(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 34, 56, 0, time.UTC)

	next := nextAlignedPing(time.Minute, now)
	assert.Equal(t, time.Date(2026, 7, 10, 12, 35, 0, 0, time.UTC), next)

	next = nextAlignedPing(time.Hour, now)
	assert.Equal(t, time.Date(2026, 7, 10, 13, 0, 0, 0, time.UTC), next)
}

func TestWorkerPing(t *testing.T) {
	t.Cleanup(Shutdown)

	require.NoError(t, Init(
		WithWorkers("ping-worker", "testdata/message-worker.php", 1,
			WithWorkerPings(50*time.Millisecond, "ping", false, false),
		),
	))

	time.Sleep(180 * time.Millisecond)
}
