//go:build !nowatcher

package frankenphp

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type workerStartCounter struct {
	nullMetrics
	starts atomic.Int32
}

func (m *workerStartCounter) StartWorker(string) {
	m.starts.Add(1)
}

func TestWatcherRestartSkippedDuringInitRunsAfterwards(t *testing.T) {
	prevMetrics := metrics
	t.Cleanup(func() {
		Shutdown()
		metrics = prevMetrics
	})

	m := &workerStartCounter{}
	require.NoError(t, Init(
		WithNumThreads(2),
		WithMetrics(m),
		WithWorkers("worker", testDataPath+"/worker-with-counter.php", 1, WithWorkerOnServerStartup(func() {
			// a watched file changes while Init() holds startupMu
			restartWorkers.Store(true)
			restartChangedWorkers()
			assert.Equal(t, int32(1), m.starts.Load())
		})),
	))

	// the reboot returns once threads are Ready, before they start the worker script
	assert.Eventually(t, func() bool { return m.starts.Load() == 2 }, 5*time.Second, 10*time.Millisecond, "the skipped restart must run once Init() releases startupMu")
	assert.False(t, restartWorkers.Load())

	restartWorkers.Store(true)
	Shutdown()
	assert.False(t, restartWorkers.Load(), "the next Init() boots from fresh code")
}

func TestWatcherRestartSkippedDuringRestartRunsAfterwards(t *testing.T) {
	prevMetrics := metrics
	t.Cleanup(func() {
		Shutdown()
		metrics = prevMetrics
	})

	m := &workerStartCounter{}
	require.NoError(t, Init(
		WithNumThreads(2),
		WithMetrics(m),
		WithWorkers("worker", testDataPath+"/worker-with-counter.php", 1),
	))

	// a watched file changes while another restart holds startupMu
	startupMu.Lock()
	restartWorkers.Store(true)
	restartChangedWorkers()
	assert.Equal(t, int32(1), m.starts.Load())

	unlockStartup()
	assert.Eventually(t, func() bool { return m.starts.Load() == 2 }, 5*time.Second, 10*time.Millisecond, "the skipped restart must run once startupMu is released")
	assert.False(t, restartWorkers.Load())
}
