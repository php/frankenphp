package frankenphp

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	totalThreadsMetricName      = "frankenphp_total_threads"
	initialTotalThreadsMetric   = 3
	reconfiguredThreadsMetric   = 2
	expectedTotalThreadsMetrics = `
		# HELP frankenphp_total_threads Total number of PHP threads
		# TYPE frankenphp_total_threads gauge
		frankenphp_total_threads 2
	`
)

func createPrometheusMetrics() *PrometheusMetrics {
	return &PrometheusMetrics{
		registry:     prometheus.NewRegistry(),
		totalThreads: prometheus.NewGauge(prometheus.GaugeOpts{Name: "frankenphp_total_threads"}),
		busyThreads:  prometheus.NewGauge(prometheus.GaugeOpts{Name: "frankenphp_busy_threads"}),
		queueDepth:   prometheus.NewGauge(prometheus.GaugeOpts{Name: "frankenphp_queue_depth"}),
	}
}

func TestPrometheusMetrics_TotalThreadsReportsCurrentValue(t *testing.T) {
	registry := prometheus.NewRegistry()
	m := NewPrometheusMetrics(registry)

	m.TotalThreads(initialTotalThreadsMetric)
	m.TotalThreads(reconfiguredThreadsMetric)

	require.NoError(t, testutil.GatherAndCompare(registry, strings.NewReader(expectedTotalThreadsMetrics), totalThreadsMetricName))
}

func TestPrometheusMetrics_TotalWorkers(t *testing.T) {
	m := createPrometheusMetrics()

	require.Nil(t, m.totalWorkers)
	require.Nil(t, m.busyWorkers)
	require.Nil(t, m.readyWorkers)
	require.Nil(t, m.workerCrashes)
	require.Nil(t, m.workerRestarts)
	require.Nil(t, m.workerRequestTime)
	require.Nil(t, m.workerRequestCount)
	require.Nil(t, m.workerTaskCount)
	require.Nil(t, m.workerTaskTime)

	m.TotalWorkersOnServer("test_worker", "test_server", 2)

	require.NotNil(t, m.totalWorkers)
	require.NotNil(t, m.busyWorkers)
	require.NotNil(t, m.readyWorkers)
	require.NotNil(t, m.workerCrashes)
	require.NotNil(t, m.workerRestarts)
	require.NotNil(t, m.workerRequestTime)
	require.NotNil(t, m.workerRequestCount)
	require.NotNil(t, m.workerTaskCount)
	require.NotNil(t, m.workerTaskTime)
}

func TestPrometheusMetrics_WorkerTask(t *testing.T) {
	m := createPrometheusMetrics()
	m.TotalWorkersOnServer("bg_worker", "bg_server", 1)
	m.StartWorkerTask("bg_worker", "bg_server")
	m.StopWorkerTask("bg_worker", "bg_server", 3*time.Second)
	m.WorkerTaskOutcome("bg_worker", "bg_server", TaskOutcomeCompleted)
	m.WorkerTaskOutcome("bg_worker", "bg_server", TaskOutcomeTimeout)

	inputs := []struct {
		name     string
		c        prometheus.Collector
		metadata string
		expect   string
	}{
		{
			name: "Testing BusyWorkers",
			c:    m.busyWorkers,
			metadata: `
				# HELP frankenphp_busy_workers Number of busy PHP workers for this worker: processing a request, or a task for a background worker
				# TYPE frankenphp_busy_workers gauge
			`,
			expect: `
				frankenphp_busy_workers{server="bg_server",worker="bg_worker"} 0
			`,
		},
		{
			name: "Testing WorkerTaskTime",
			c:    m.workerTaskTime,
			metadata: `
				# HELP frankenphp_worker_task_time Time spent on tasks by all threads of this background worker, from pickup to the close of the task's stream
				# TYPE frankenphp_worker_task_time counter
			`,
			expect: `
				frankenphp_worker_task_time{server="bg_server",worker="bg_worker"} 3
			`,
		},
		{
			name: "Testing WorkerTaskCount",
			c:    m.workerTaskCount,
			metadata: `
				# HELP frankenphp_worker_task_count Number of tasks sent to this background worker, by outcome: completed, aborted (the script ended with the task open), abandoned (the sender closed its stream first) or timeout (no thread picked the task up in time)
				# TYPE frankenphp_worker_task_count counter
			`,
			expect: `
				frankenphp_worker_task_count{outcome="completed",server="bg_server",worker="bg_worker"} 1
				frankenphp_worker_task_count{outcome="timeout",server="bg_server",worker="bg_worker"} 1
			`,
		},
	}

	for _, input := range inputs {
		t.Run(input.name, func(t *testing.T) {
			require.NoError(t, testutil.CollectAndCompare(input.c, strings.NewReader(input.metadata+input.expect)))
		})
	}
}

func TestPrometheusMetrics_StopWorkerRequest(t *testing.T) {
	m := createPrometheusMetrics()
	m.TotalWorkersOnServer("test_worker", "test_server", 2)
	m.StopWorkerRequestOnServer("test_worker", "test_server", 2*time.Second)

	inputs := []struct {
		name     string
		c        prometheus.Collector
		metadata string
		expect   string
	}{
		{
			name: "Testing WorkerRequestCount",
			c:    m.workerRequestCount,
			metadata: `
				# HELP frankenphp_worker_request_count
				# TYPE frankenphp_worker_request_count counter
			`,
			expect: `
				frankenphp_worker_request_count{server="test_server",worker="test_worker"} 1
			`,
		},
		{
			name: "Testing BusyWorkers",
			c:    m.busyWorkers,
			metadata: `
				# HELP frankenphp_busy_workers Number of busy PHP workers for this worker: processing a request, or a task for a background worker
				# TYPE frankenphp_busy_workers gauge
			`,
			expect: `
				frankenphp_busy_workers{server="test_server",worker="test_worker"} -1
			`,
		},
		{
			name: "Testing WorkerRequestTime",
			c:    m.workerRequestTime,
			metadata: `
				# HELP frankenphp_worker_request_time
				# TYPE frankenphp_worker_request_time counter
			`,
			expect: `
				frankenphp_worker_request_time{server="test_server",worker="test_worker"} 2
			`,
		},
	}

	for _, input := range inputs {
		t.Run(input.name, func(t *testing.T) {
			require.NoError(t, testutil.CollectAndCompare(input.c, strings.NewReader(input.metadata+input.expect)))
		})

	}
}

func TestPrometheusMetrics_StartWorkerRequest(t *testing.T) {
	m := createPrometheusMetrics()
	m.TotalWorkersOnServer("test_worker", "test_server", 2)
	m.StartWorkerRequestOnServer("test_worker", "test_server")

	inputs := []struct {
		name     string
		c        prometheus.Collector
		metadata string
		expect   string
	}{
		{
			name: "Testing BusyWorkers",
			c:    m.busyWorkers,
			metadata: `
				# HELP frankenphp_busy_workers Number of busy PHP workers for this worker: processing a request, or a task for a background worker
				# TYPE frankenphp_busy_workers gauge
			`,
			expect: `
				frankenphp_busy_workers{server="test_server",worker="test_worker"} 1
			`,
		},
	}

	for _, input := range inputs {
		t.Run(input.name, func(t *testing.T) {
			require.NoError(t, testutil.CollectAndCompare(input.c, strings.NewReader(input.metadata+input.expect)))
		})

	}
}

func TestPrometheusMetrics_TestStopReasonCrash(t *testing.T) {
	m := createPrometheusMetrics()
	m.TotalWorkersOnServer("test_worker", "test_server", 2)
	m.StopWorkerOnServer("test_worker", "test_server", StopReasonCrash)

	inputs := []struct {
		name     string
		c        prometheus.Collector
		metadata string
		expect   string
	}{
		{
			name: "Testing BusyThreads",
			c:    m.busyThreads,
			metadata: `
				# HELP frankenphp_busy_threads
				# TYPE frankenphp_busy_threads gauge
			`,
			expect: `
				frankenphp_busy_threads -1
			`,
		},
		{
			name: "Testing TotalWorkers",
			c:    m.totalWorkers,
			metadata: `
				# HELP frankenphp_total_workers Total number of PHP workers for this worker
				# TYPE frankenphp_total_workers gauge
			`,
			expect: `
				frankenphp_total_workers{server="test_server",worker="test_worker"} -1
			`,
		},
		{
			name: "Testing ReadyWorkers",
			c:    m.readyWorkers,
			metadata: `
				# HELP frankenphp_ready_workers Running workers that have reached their ready point at least once: frankenphp_handle_request for HTTP workers, WorkerHandle::tick() for background workers
				# TYPE frankenphp_ready_workers gauge
			`,
			expect: `
				frankenphp_ready_workers{server="test_server",worker="test_worker"} -1
			`,
		},
		{
			name: "Testing WorkerCrashes",
			c:    m.workerCrashes,
			metadata: `
				# HELP frankenphp_worker_crashes Number of PHP worker crashes for this worker
				# TYPE frankenphp_worker_crashes counter
			`,
			expect: `
				frankenphp_worker_crashes{server="test_server",worker="test_worker"} 1
			`,
		},
	}

	for _, input := range inputs {
		t.Run(input.name, func(t *testing.T) {
			require.NoError(t, testutil.CollectAndCompare(input.c, strings.NewReader(input.metadata+input.expect)))
		})

	}
}

// packedMetrics records the worker names a Metrics implementation without
// ServerMetrics receives
type packedMetrics struct {
	nullMetrics
	names []string
}

func (m *packedMetrics) StartWorker(name string)              { m.names = append(m.names, name) }
func (m *packedMetrics) ReadyWorker(name string)              { m.names = append(m.names, name) }
func (m *packedMetrics) StopWorker(name string, _ StopReason) { m.names = append(m.names, name) }
func (m *packedMetrics) TotalWorkers(name string, _ int)      { m.names = append(m.names, name) }
func (m *packedMetrics) StopWorkerRequest(name string, _ time.Duration) {
	m.names = append(m.names, name)
}
func (m *packedMetrics) StartWorkerRequest(name string)    { m.names = append(m.names, name) }
func (m *packedMetrics) QueuedWorkerRequest(name string)   { m.names = append(m.names, name) }
func (m *packedMetrics) DequeuedWorkerRequest(name string) { m.names = append(m.names, name) }

// splitMetrics records the name and server pairs a ServerMetrics
// implementation receives, and fails the test if a packed method is called
type splitMetrics struct {
	t     *testing.T
	pairs [][2]string
}

func (m *splitMetrics) record(name, server string) {
	m.pairs = append(m.pairs, [2]string{name, server})
}

func (m *splitMetrics) StartWorkerOnServer(name, server string)              { m.record(name, server) }
func (m *splitMetrics) ReadyWorkerOnServer(name, server string)              { m.record(name, server) }
func (m *splitMetrics) StopWorkerOnServer(name, server string, _ StopReason) { m.record(name, server) }
func (m *splitMetrics) TotalWorkersOnServer(name, server string, _ int)      { m.record(name, server) }
func (m *splitMetrics) StopWorkerRequestOnServer(name, server string, _ time.Duration) {
	m.record(name, server)
}
func (m *splitMetrics) StartWorkerRequestOnServer(name, server string)    { m.record(name, server) }
func (m *splitMetrics) QueuedWorkerRequestOnServer(name, server string)   { m.record(name, server) }
func (m *splitMetrics) DequeuedWorkerRequestOnServer(name, server string) { m.record(name, server) }
func (m *splitMetrics) StartWorkerTask(name, server string)               { m.record(name, server) }
func (m *splitMetrics) StopWorkerTask(name, server string, _ time.Duration) {
	m.record(name, server)
}
func (m *splitMetrics) WorkerTaskOutcome(name, server string, _ TaskOutcome) {
	m.record(name, server)
}

func (m *splitMetrics) StartWorker(string)            { m.t.Fatal("packed StartWorker called") }
func (m *splitMetrics) ReadyWorker(string)            { m.t.Fatal("packed ReadyWorker called") }
func (m *splitMetrics) StopWorker(string, StopReason) { m.t.Fatal("packed StopWorker called") }
func (m *splitMetrics) TotalWorkers(string, int)      { m.t.Fatal("packed TotalWorkers called") }
func (m *splitMetrics) TotalThreads(int)              {}
func (m *splitMetrics) StartRequest()                 {}
func (m *splitMetrics) StopRequest()                  {}
func (m *splitMetrics) StopWorkerRequest(string, time.Duration) {
	m.t.Fatal("packed StopWorkerRequest called")
}
func (m *splitMetrics) StartWorkerRequest(string)  { m.t.Fatal("packed StartWorkerRequest called") }
func (m *splitMetrics) Shutdown()                  {}
func (m *splitMetrics) QueuedWorkerRequest(string) { m.t.Fatal("packed QueuedWorkerRequest called") }
func (m *splitMetrics) DequeuedWorkerRequest(string) {
	m.t.Fatal("packed DequeuedWorkerRequest called")
}
func (m *splitMetrics) QueuedRequest()   {}
func (m *splitMetrics) DequeuedRequest() {}

func reportEveryWorkerMethod(m workerMetrics, name, server string) {
	m.StartWorker(name, server)
	m.ReadyWorker(name, server)
	m.StopWorker(name, server, StopReasonRestart)
	m.TotalWorkers(name, server, 1)
	m.StopWorkerRequest(name, server, time.Second)
	m.StartWorkerRequest(name, server)
	m.QueuedWorkerRequest(name, server)
	m.DequeuedWorkerRequest(name, server)
}

// a Metrics implementation without ServerMetrics gets the packed name, the
// bare name for a global worker
func TestMetricsAdapterPacksTheServerIntoTheName(t *testing.T) {
	m := &packedMetrics{}
	a := adaptMetrics(m)

	reportEveryWorkerMethod(a, "queue", "api")
	reportEveryWorkerMethod(a, "queue", "")

	require.Len(t, m.names, 16)
	assert.Equal(t, []string{"api:queue", "api:queue", "api:queue", "api:queue", "api:queue", "api:queue", "api:queue", "api:queue"}, m.names[:8])
	assert.Equal(t, []string{"queue", "queue", "queue", "queue", "queue", "queue", "queue", "queue"}, m.names[8:])
}

// a ServerMetrics implementation gets the two names apart and its packed
// methods are never called
func TestMetricsAdapterKeepsTheServerApart(t *testing.T) {
	m := &splitMetrics{t: t}
	a := adaptMetrics(m)

	reportEveryWorkerMethod(a, "queue", "api")

	require.Len(t, m.pairs, 8)
	for _, pair := range m.pairs {
		assert.Equal(t, [2]string{"queue", "api"}, pair)
	}
}
