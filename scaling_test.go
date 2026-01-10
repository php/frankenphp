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
	workerPath := filepath.Join(testDataPath, "/transition-worker-1.php")
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
	scaleWorkerThread(getWorkerByPath(workerPath))
	assert.Equal(t, state.Ready, autoScaledThread.state.Get())

	// on down-scale, the thread will be marked as inactive
	setLongWaitTime(t, autoScaledThread)
	deactivateThreads()
	assert.IsType(t, &inactiveThread{}, autoScaledThread.handler)
}

func setLongWaitTime(t *testing.T, thread *phpThread) {
	t.Helper()

	thread.state.SetWaitTime(time.Now().Add(-time.Hour))
}
