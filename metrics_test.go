package frankenphp

import (
	"bytes"
	"log/slog"
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

	m.TotalWorkers("test_worker", 2)

	require.NotNil(t, m.totalWorkers)
	require.NotNil(t, m.busyWorkers)
	require.NotNil(t, m.readyWorkers)
	require.NotNil(t, m.workerCrashes)
	require.NotNil(t, m.workerRestarts)
	require.NotNil(t, m.workerRequestTime)
	require.NotNil(t, m.workerRequestCount)
}

func TestPrometheusMetrics_StopWorkerRequest(t *testing.T) {
	m := createPrometheusMetrics()
	m.TotalWorkers("test_worker", 2)
	m.StopWorkerRequest("test_worker", 2*time.Second)

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
				frankenphp_worker_request_count{worker="test_worker"} 1
			`,
		},
		{
			name: "Testing BusyWorkers",
			c:    m.busyWorkers,
			metadata: `
				# HELP frankenphp_busy_workers Number of busy PHP workers for this worker
				# TYPE frankenphp_busy_workers gauge
			`,
			expect: `
				frankenphp_busy_workers{worker="test_worker"} -1
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
				frankenphp_worker_request_time{worker="test_worker"} 2
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
	m.TotalWorkers("test_worker", 2)
	m.StartWorkerRequest("test_worker")

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
				# HELP frankenphp_busy_workers Number of busy PHP workers for this worker
				# TYPE frankenphp_busy_workers gauge
			`,
			expect: `
				frankenphp_busy_workers{worker="test_worker"} 1
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
	m.TotalWorkers("test_worker", 2)
	m.StopWorker("test_worker", StopReasonCrash)

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
				frankenphp_total_workers{worker="test_worker"} -1
			`,
		},
		{
			name: "Testing ReadyWorkers",
			c:    m.readyWorkers,
			metadata: `
				# HELP frankenphp_ready_workers Running workers that have successfully called frankenphp_handle_request at least once
				# TYPE frankenphp_ready_workers gauge
			`,
			expect: `
				frankenphp_ready_workers{worker="test_worker"} -1
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
				frankenphp_worker_crashes{worker="test_worker"} 1
			`,
		},
	}

	for _, input := range inputs {
		t.Run(input.name, func(t *testing.T) {
			require.NoError(t, testutil.CollectAndCompare(input.c, strings.NewReader(input.metadata+input.expect)))
		})

	}
}

func TestPrometheusMetrics_OpcacheRestart(t *testing.T) {
	if !opcacheRestartHook {
		t.Skip("this build has no opcache restart hook")
	}

	registry := prometheus.NewRegistry()
	m := NewPrometheusMetrics(registry)
	m.OpcacheRestart("hash")
	m.OpcacheRestart("hash")
	m.OpcacheRestart("oom")

	// known reasons are exposed from the start, unknown ones only once seen
	require.NoError(t, testutil.GatherAndCompare(registry, strings.NewReader(`
		# HELP frankenphp_opcache_restarts Number of restarts of opcache's shared memory scheduled, by reason (experimental, should stay at zero)
		# TYPE frankenphp_opcache_restarts counter
		frankenphp_opcache_restarts{reason="hash"} 2
		frankenphp_opcache_restarts{reason="manual"} 0
		frankenphp_opcache_restarts{reason="oom"} 1
	`), "frankenphp_opcache_restarts"))

	m.OpcacheRestart("unknown")

	require.NoError(t, testutil.CollectAndCompare(m.opcacheRestarts, strings.NewReader(`
		# HELP frankenphp_opcache_restarts Number of restarts of opcache's shared memory scheduled, by reason (experimental, should stay at zero)
		# TYPE frankenphp_opcache_restarts counter
		frankenphp_opcache_restarts{reason="hash"} 2
		frankenphp_opcache_restarts{reason="manual"} 0
		frankenphp_opcache_restarts{reason="oom"} 1
		frankenphp_opcache_restarts{reason="unknown"} 1
	`)))
}

func TestOpcacheRestartScheduledLogsAndCounts(t *testing.T) {
	if !opcacheRestartHook {
		t.Skip("this build has no opcache restart hook")
	}

	var buf bytes.Buffer
	m := NewPrometheusMetrics(prometheus.NewRegistry())
	prevLogger, prevMetrics := globalLogger, metrics
	globalLogger, metrics = slog.New(slog.NewTextHandler(&buf, nil)), m
	t.Cleanup(func() { globalLogger, metrics = prevLogger, prevMetrics })

	opcacheRestartScheduled(1)
	opcacheRestartScheduled(7)

	assert.Contains(t, buf.String(), `level=WARN msg="opcache restart scheduled`)
	assert.Contains(t, buf.String(), "reason=hash")
	assert.Contains(t, buf.String(), "reason=unknown")
	assert.Equal(t, float64(1), testutil.ToFloat64(m.opcacheRestarts.WithLabelValues("hash")))
	assert.Equal(t, float64(1), testutil.ToFloat64(m.opcacheRestarts.WithLabelValues("unknown")))
}

func TestOpcacheRestartScheduledWithoutOpcacheMetrics(t *testing.T) {
	// nullMetrics is a Metrics without the optional part, like an
	// implementation from before OpcacheMetrics existed
	_, ok := Metrics(nullMetrics{}).(OpcacheMetrics)
	require.False(t, ok)

	var buf bytes.Buffer
	prevLogger, prevMetrics := globalLogger, metrics
	globalLogger, metrics = slog.New(slog.NewTextHandler(&buf, nil)), nullMetrics{}
	t.Cleanup(func() { globalLogger, metrics = prevLogger, prevMetrics })

	assert.NotPanics(t, func() { opcacheRestartScheduled(0) })
	assert.Contains(t, buf.String(), "reason=oom")
}
