package frankenphp

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
)

func TestStallSampler_EmptyBufferReturnsZero(t *testing.T) {
	s := &stallSampler{}

	w1, w3, w5 := s.snapshot()

	require.Equal(t, 0.0, w1)
	require.Equal(t, 0.0, w3)
	require.Equal(t, 0.0, w5)
}

func TestStallSampler_AlwaysStalled(t *testing.T) {
	s := &stallSampler{}

	for i := 0; i < stallWindow5; i++ {
		s.record(true)
	}

	w1, w3, w5 := s.snapshot()

	require.Equal(t, 1.0, w1)
	require.Equal(t, 1.0, w3)
	require.Equal(t, 1.0, w5)
}

func TestStallSampler_NeverStalled(t *testing.T) {
	s := &stallSampler{}

	for i := 0; i < stallWindow5; i++ {
		s.record(false)
	}

	w1, w3, w5 := s.snapshot()

	require.Equal(t, 0.0, w1)
	require.Equal(t, 0.0, w3)
	require.Equal(t, 0.0, w5)
}

func TestStallSampler_PartiallyStalled(t *testing.T) {
	s := &stallSampler{}

	// Half stalled, half not — interleaved across the full 5-second window.
	for i := 0; i < stallWindow5; i++ {
		s.record(i%2 == 0)
	}

	w1, w3, w5 := s.snapshot()

	require.InDelta(t, 0.5, w1, 0.1)
	require.InDelta(t, 0.5, w3, 0.05)
	require.InDelta(t, 0.5, w5, 0.0001)
}

func TestStallSampler_RecentSamplesDominate(t *testing.T) {
	s := &stallSampler{}

	// 4 seconds of idle, then 1 second of stalled.
	for i := 0; i < stallWindow5-stallWindow1; i++ {
		s.record(false)
	}
	for i := 0; i < stallWindow1; i++ {
		s.record(true)
	}

	w1, w3, w5 := s.snapshot()

	require.Equal(t, 1.0, w1, "1s window should be fully stalled")
	require.InDelta(t, float64(stallWindow1)/float64(stallWindow3), w3, 0.0001, "3s window covers 1s of stall out of 3s")
	require.InDelta(t, float64(stallWindow1)/float64(stallWindow5), w5, 0.0001, "5s window covers 1s of stall out of 5s")
}

func TestStallSampler_RingBufferOverwrites(t *testing.T) {
	s := &stallSampler{}

	// Fill with `false`, then overwrite the entire buffer with `true`.
	// The earliest `false` samples must have been evicted.
	for i := 0; i < stallWindow5; i++ {
		s.record(false)
	}
	for i := 0; i < stallWindow5; i++ {
		s.record(true)
	}

	w1, w3, w5 := s.snapshot()

	require.Equal(t, 1.0, w1)
	require.Equal(t, 1.0, w3)
	require.Equal(t, 1.0, w5)
}

func TestStallSampler_PartialFillUsesActualSampleCount(t *testing.T) {
	s := &stallSampler{}

	// Only 2 samples ever recorded, both stalled. snapshot() should
	// report 1.0 for every window — averaging over the actual sample
	// count, not zero-padding the missing slots.
	s.record(true)
	s.record(true)

	w1, w3, w5 := s.snapshot()

	require.Equal(t, 1.0, w1)
	require.Equal(t, 1.0, w3)
	require.Equal(t, 1.0, w5)
}

func TestPrometheusMetrics_WorkerStalled(t *testing.T) {
	m := createPrometheusMetrics()
	m.TotalWorkers("test_worker", 2)

	require.NotNil(t, m.workerStalled1s)
	require.NotNil(t, m.workerStalled3s)
	require.NotNil(t, m.workerStalled5s)

	m.WorkerStalled("test_worker", 0.25, 0.5, 0.75)

	cases := []struct {
		name     string
		c        prometheus.Collector
		metadata string
		expect   string
	}{
		{
			name: "stalled_1s",
			c:    m.workerStalled1s,
			metadata: `
					# HELP frankenphp_worker_stalled_1s Fraction of the last 1 second the worker's request queue was non-empty (0..1)
					# TYPE frankenphp_worker_stalled_1s gauge
				`,
			expect: `
					frankenphp_worker_stalled_1s{worker="test_worker"} 0.25
				`,
		},
		{
			name: "stalled_3s",
			c:    m.workerStalled3s,
			metadata: `
					# HELP frankenphp_worker_stalled_3s Fraction of the last 3 seconds the worker's request queue was non-empty (0..1)
					# TYPE frankenphp_worker_stalled_3s gauge
				`,
			expect: `
					frankenphp_worker_stalled_3s{worker="test_worker"} 0.5
				`,
		},
		{
			name: "stalled_5s",
			c:    m.workerStalled5s,
			metadata: `
					# HELP frankenphp_worker_stalled_5s Fraction of the last 5 seconds the worker's request queue was non-empty (0..1)
					# TYPE frankenphp_worker_stalled_5s gauge
				`,
			expect: `
					frankenphp_worker_stalled_5s{worker="test_worker"} 0.75
				`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.NoError(t, testutil.CollectAndCompare(c.c, strings.NewReader(c.metadata+c.expect)))
		})
	}
}

func TestPrometheusMetrics_WorkerStalledNoOpBeforeRegistration(t *testing.T) {
	m := createPrometheusMetrics()

	// Must not panic even if TotalWorkers has not been called yet.
	m.WorkerStalled("test_worker", 0.5, 0.5, 0.5)
}
