//go:build !nowatcher

package frankenphp

import (
	"sync/atomic"
	"testing"

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

	assert.Equal(t, int32(2), m.starts.Load(), "the skipped restart must run once Init() releases startupMu")
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
	assert.Equal(t, int32(2), m.starts.Load(), "the skipped restart must run once startupMu is released")
	assert.False(t, restartWorkers.Load())
}
