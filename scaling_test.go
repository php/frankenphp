package frankenphp

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestScaleARegularThreadUpAndDown(t *testing.T) {
	t.Cleanup(Shutdown)

	assert.NoError(t, Init(
		WithNumThreads(1),
		WithMaxThreads(2),
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
	))

	autoScaledThread := phpThreads[1]

	// scale up
	scaleRegularThread()
	assert.Equal(t, stateReady, autoScaledThread.state.get())
	assert.IsType(t, &regularThread{}, autoScaledThread.handler)

	// on down-scale, the thread will be marked as inactive
	setLongWaitTime(t, autoScaledThread)
	deactivateThreads()
	assert.IsType(t, &inactiveThread{}, autoScaledThread.handler)
}

func TestScaleAWorkerThreadUpAndDown(t *testing.T) {
	t.Cleanup(Shutdown)

	workerName := "worker1"
	workerPath := testDataPath + "/transition-worker-1.php"
	assert.NoError(t, Init(
		WithNumThreads(2),
		WithMaxThreads(3),
		WithWorkers(workerName, workerPath, 1,
			WithWorkerEnv(map[string]string{}),
			WithWorkerWatchMode([]string{}),
			WithWorkerMaxFailures(0),
		),
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
	))

	autoScaledThread := phpThreads[2]

	// scale up
	scaleWorkerThread(getWorkerByPath(workerPath))
	assert.Equal(t, stateReady, autoScaledThread.state.get())

	// on down-scale, the thread will be marked as inactive
	setLongWaitTime(t, autoScaledThread)
	deactivateThreads()
	assert.IsType(t, &inactiveThread{}, autoScaledThread.handler)
}

func setLongWaitTime(t *testing.T, thread *phpThread) {
	t.Helper()

	thread.state.mu.Lock()
	thread.state.waitingSince = time.Now().Add(-time.Hour)
	thread.state.mu.Unlock()
}
