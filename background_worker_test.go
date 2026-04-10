package frankenphp

import (
	"testing"
	"time"

	"github.com/dunglas/frankenphp/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type backgroundWorkerTestMetrics struct {
	readyCalls int
	stopCalls  []StopReason
}

func (m *backgroundWorkerTestMetrics) StartWorker(string) {}

func (m *backgroundWorkerTestMetrics) ReadyWorker(string) {
	m.readyCalls++
}

func (m *backgroundWorkerTestMetrics) StopWorker(_ string, reason StopReason) {
	m.stopCalls = append(m.stopCalls, reason)
}

func (m *backgroundWorkerTestMetrics) TotalWorkers(string, int) {}

func (m *backgroundWorkerTestMetrics) TotalThreads(int) {}

func (m *backgroundWorkerTestMetrics) StartRequest() {}

func (m *backgroundWorkerTestMetrics) StopRequest() {}

func (m *backgroundWorkerTestMetrics) StopWorkerRequest(string, time.Duration) {}

func (m *backgroundWorkerTestMetrics) StartWorkerRequest(string) {}

func (m *backgroundWorkerTestMetrics) Shutdown() {}

func (m *backgroundWorkerTestMetrics) QueuedWorkerRequest(string) {}

func (m *backgroundWorkerTestMetrics) DequeuedWorkerRequest(string) {}

func (m *backgroundWorkerTestMetrics) QueuedRequest() {}

func (m *backgroundWorkerTestMetrics) DequeuedRequest() {}

func TestStartBackgroundWorkerFailureIsRetryable(t *testing.T) {
	lookup := newBackgroundWorkerLookup()
	lookup.catchAll = newBackgroundWorkerRegistry(testDataPath + "/background-worker-with-argv.php")
	backgroundLookups = map[string]*backgroundWorkerLookup{"": lookup}
	thread := newPHPThread(0)
	thread.state.Set(state.Ready)
	thread.handler = &workerThread{
		thread: thread,
		worker: &worker{backgroundLookup: lookup},
	}
	phpThreads = []*phpThread{thread}
	t.Cleanup(func() {
		phpThreads = nil
	})

	registry := lookup.Resolve("retryable-background-worker")

	err := startBackgroundWorker(thread, "retryable-background-worker")
	require.EqualError(t, err, "no available PHP thread for background worker (increase max_threads)")
	assert.Empty(t, registry.workers)

	err = startBackgroundWorker(thread, "retryable-background-worker")
	require.EqualError(t, err, "no available PHP thread for background worker (increase max_threads)")
	assert.Empty(t, registry.workers)
}

func TestBackgroundWorkerSetVarsMarksWorkerReady(t *testing.T) {
	originalMetrics := metrics
	testMetrics := &backgroundWorkerTestMetrics{}
	metrics = testMetrics
	t.Cleanup(func() {
		metrics = originalMetrics
	})

	handler := &backgroundWorkerThread{
		thread:          newPHPThread(0),
		worker:          &worker{name: "background-worker", fileName: "background-worker.php", maxConsecutiveFailures: -1},
		isBootingScript: true,
	}

	handler.markBackgroundReady()
	handler.markBackgroundReady()

	assert.False(t, handler.isBootingScript)
	assert.Equal(t, 0, handler.failureCount)
	assert.Equal(t, 1, testMetrics.readyCalls)
}

func TestBackgroundWorkerBootFailureStaysBootFailureUntilReady(t *testing.T) {
	originalMetrics := metrics
	testMetrics := &backgroundWorkerTestMetrics{}
	metrics = testMetrics
	t.Cleanup(func() {
		metrics = originalMetrics
	})

	handler := &backgroundWorkerThread{
		thread: newPHPThread(0),
		worker: &worker{
			name:                   "background-worker",
			fileName:               "background-worker.php",
			maxConsecutiveFailures: -1,
		},
		isBootingScript: true,
	}

	handler.afterScriptExecution(1)
	require.Len(t, testMetrics.stopCalls, 1)
	assert.Equal(t, StopReason(StopReasonBootFailure), testMetrics.stopCalls[0])

	testMetrics.stopCalls = nil
	handler.isBootingScript = true
	handler.markBackgroundReady()
	handler.afterScriptExecution(1)
	require.Len(t, testMetrics.stopCalls, 1)
	assert.Equal(t, StopReason(StopReasonCrash), testMetrics.stopCalls[0])
}
