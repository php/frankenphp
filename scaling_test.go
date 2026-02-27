package frankenphp

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/dunglas/frankenphp/internal/state"
	"github.com/stretchr/testify/assert"
)

func TestScaleARegularThreadUpAndDown(t *testing.T) {
	t.Cleanup(Shutdown)

	assert.NoError(t, Init(
		WithNumThreads(1),
		WithMaxThreads(2),
	))

	autoScaledThread := phpThreads[1]

	// scale up
	scaleRegularThread()
	assert.Equal(t, state.Ready, autoScaledThread.state.Get())
	assert.IsType(t, &regularThread{}, autoScaledThread.handler)

	// on down-scale, the thread will be marked as inactive
	setLongWaitTime(t, autoScaledThread)
	deactivateThreads()
	assert.IsType(t, &inactiveThread{}, autoScaledThread.handler)
}

func TestScaleAWorkerThreadUpAndDown(t *testing.T) {
	t.Cleanup(Shutdown)

	workerName := "worker1"
	workerPath := filepath.Join(testDataPath, "transition-worker-1.php")
	assert.NoError(t, Init(
		WithNumThreads(2),
		WithMaxThreads(3),
		WithWorkers(workerName, workerPath, 1,
			WithWorkerEnv(map[string]string{}),
			WithWorkerWatchMode([]string{}),
			WithWorkerMaxFailures(0),
		),
	))

	autoScaledThread := phpThreads[2]

	// scale up
	scaleWorkerThread(workersByPath[workerPath])
	assert.Equal(t, state.Ready, autoScaledThread.state.Get())

	// on down-scale, the thread will be marked as inactive
	setLongWaitTime(t, autoScaledThread)
	deactivateThreads()
	assert.IsType(t, &inactiveThread{}, autoScaledThread.handler)
}

func TestMaxIdleTimePreventsEarlyDeactivation(t *testing.T) {
	t.Cleanup(Shutdown)

	assert.NoError(t, Init(
		WithNumThreads(1),
		WithMaxThreads(2),
		WithMaxIdleTime(time.Hour),
	))

	autoScaledThread := phpThreads[1]

	// scale up
	scaleRegularThread()
	assert.Equal(t, state.Ready, autoScaledThread.state.Get())

	// set wait time to 30 minutes (less than 1 hour max idle time)
	autoScaledThread.state.SetWaitTime(time.Now().Add(-30 * time.Minute))
	deactivateThreads()
	assert.IsType(t, &regularThread{}, autoScaledThread.handler, "thread should still be active after 30min with 1h max idle time")

	// set wait time to over 1 hour (exceeds max idle time)
	autoScaledThread.state.SetWaitTime(time.Now().Add(-time.Hour - time.Minute))
	deactivateThreads()
	assert.IsType(t, &inactiveThread{}, autoScaledThread.handler, "thread should be deactivated after exceeding max idle time")
}

func TestDefaultMaxIdleTimeIs5Seconds(t *testing.T) {
	t.Cleanup(Shutdown)

	assert.NoError(t, Init(
		WithNumThreads(1),
		WithMaxThreads(2),
	))

	autoScaledThread := phpThreads[1]

	// scale up
	scaleRegularThread()
	assert.Equal(t, state.Ready, autoScaledThread.state.Get())

	// set wait time to 10 seconds (exceeds default 5s max idle time)
	autoScaledThread.state.SetWaitTime(time.Now().Add(-10 * time.Second))
	deactivateThreads()
	assert.IsType(t, &inactiveThread{}, autoScaledThread.handler, "thread should be deactivated after 10s with default 5s max idle time")
}

func setLongWaitTime(t *testing.T, thread *phpThread) {
	t.Helper()

	thread.state.SetWaitTime(time.Now().Add(-time.Hour))
}
