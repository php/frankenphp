package frankenphp

import (
	"errors"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	StopReasonCrash = iota
	StopReasonRestart
	StopReasonBootFailure // worker crashed before reaching frankenphp_handle_request
)

type StopReason int

// Metrics is the worker-level instrumentation surface. Every method that
// identifies a specific worker takes a (server, name) pair: server is the
// per-php_server label resolved via ScopeLabel, name is the worker name.
// The pair is what disambiguates same-named workers declared in distinct
// php_server blocks.
type Metrics interface {
	// StartWorker collects started workers
	StartWorker(server, name string)
	// ReadyWorker collects ready workers
	ReadyWorker(server, name string)
	// StopWorker collects stopped workers
	StopWorker(server, name string, reason StopReason)
	// TotalWorkers collects expected workers
	TotalWorkers(server, name string, num int)
	// TotalThreads collects total threads
	TotalThreads(num int)
	// StartRequest collects started requests
	StartRequest()
	// StopRequest collects stopped requests
	StopRequest()
	// StopWorkerRequest collects stopped worker requests
	StopWorkerRequest(server, name string, duration time.Duration)
	// StartWorkerRequest collects started worker requests
	StartWorkerRequest(server, name string)
	Shutdown()
	QueuedWorkerRequest(server, name string)
	DequeuedWorkerRequest(server, name string)
	QueuedRequest()
	DequeuedRequest()
}

type nullMetrics struct{}

func (n nullMetrics) StartWorker(string, string) {
}

func (n nullMetrics) ReadyWorker(string, string) {
}

func (n nullMetrics) StopWorker(string, string, StopReason) {
}

func (n nullMetrics) TotalWorkers(string, string, int) {
}

func (n nullMetrics) TotalThreads(int) {
}

func (n nullMetrics) StartRequest() {
}

func (n nullMetrics) StopRequest() {
}

func (n nullMetrics) StopWorkerRequest(string, string, time.Duration) {
}

func (n nullMetrics) StartWorkerRequest(string, string) {
}

func (n nullMetrics) Shutdown() {
}

func (n nullMetrics) QueuedWorkerRequest(string, string) {}

func (n nullMetrics) DequeuedWorkerRequest(string, string) {}

func (n nullMetrics) QueuedRequest()   {}
func (n nullMetrics) DequeuedRequest() {}

type PrometheusMetrics struct {
	registry           prometheus.Registerer
	totalThreads       prometheus.Counter
	busyThreads        prometheus.Gauge
	totalWorkers       *prometheus.GaugeVec
	busyWorkers        *prometheus.GaugeVec
	readyWorkers       *prometheus.GaugeVec
	workerCrashes      *prometheus.CounterVec
	workerRestarts     *prometheus.CounterVec
	workerRequestTime  *prometheus.CounterVec
	workerRequestCount *prometheus.CounterVec
	workerQueueDepth   *prometheus.GaugeVec
	queueDepth         prometheus.Gauge
	mu                 sync.Mutex
}

func (m *PrometheusMetrics) StartWorker(server, name string) {
	m.busyThreads.Inc()

	// tests do not register workers before starting them
	if m.totalWorkers == nil {
		return
	}

	m.totalWorkers.WithLabelValues(server, name).Inc()
}

func (m *PrometheusMetrics) ReadyWorker(server, name string) {
	if m.totalWorkers == nil {
		return
	}

	m.readyWorkers.WithLabelValues(server, name).Inc()
}

func (m *PrometheusMetrics) StopWorker(server, name string, reason StopReason) {
	m.busyThreads.Dec()

	// tests do not register workers before starting them
	if m.totalWorkers == nil {
		return
	}

	m.totalWorkers.WithLabelValues(server, name).Dec()

	// only decrement readyWorkers if the worker actually reached frankenphp_handle_request
	if reason != StopReasonBootFailure {
		m.readyWorkers.WithLabelValues(server, name).Dec()
	}

	switch reason {
	case StopReasonCrash, StopReasonBootFailure:
		m.workerCrashes.WithLabelValues(server, name).Inc()
	case StopReasonRestart:
		m.workerRestarts.WithLabelValues(server, name).Inc()
	}
}

func (m *PrometheusMetrics) TotalWorkers(string, string, int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	const ns, sub = "frankenphp", "worker"
	basicLabels := []string{"server", "worker"}

	if m.totalWorkers == nil {
		m.totalWorkers = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "total_workers",
			Help:      "Total number of PHP workers for this worker",
		}, basicLabels)
		if err := m.registry.Register(m.totalWorkers); err != nil &&
			!errors.As(err, &prometheus.AlreadyRegisteredError{}) {
			panic(err)
		}
	}

	if m.readyWorkers == nil {
		m.readyWorkers = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "ready_workers",
			Help:      "Running workers that have successfully called frankenphp_handle_request at least once",
		}, basicLabels)
		if err := m.registry.Register(m.readyWorkers); err != nil &&
			!errors.As(err, &prometheus.AlreadyRegisteredError{}) {
			panic(err)
		}
	}

	if m.busyWorkers == nil {
		m.busyWorkers = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "busy_workers",
			Help:      "Number of busy PHP workers for this worker",
		}, basicLabels)
		if err := m.registry.Register(m.busyWorkers); err != nil &&
			!errors.As(err, &prometheus.AlreadyRegisteredError{}) {
			panic(err)
		}
	}

	if m.workerCrashes == nil {
		m.workerCrashes = prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Subsystem: sub,
			Name:      "crashes",
			Help:      "Number of PHP worker crashes for this worker",
		}, basicLabels)
		if err := m.registry.Register(m.workerCrashes); err != nil &&
			!errors.As(err, &prometheus.AlreadyRegisteredError{}) {
			panic(err)
		}
	}

	if m.workerRestarts == nil {
		m.workerRestarts = prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Subsystem: sub,
			Name:      "restarts",
			Help:      "Number of PHP worker restarts for this worker",
		}, basicLabels)
		if err := m.registry.Register(m.workerRestarts); err != nil &&
			!errors.As(err, &prometheus.AlreadyRegisteredError{}) {
			panic(err)
		}
	}

	if m.workerRequestTime == nil {
		m.workerRequestTime = prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Subsystem: sub,
			Name:      "request_time",
		}, basicLabels)
		if err := m.registry.Register(m.workerRequestTime); err != nil &&
			!errors.As(err, &prometheus.AlreadyRegisteredError{}) {
			panic(err)
		}
	}

	if m.workerRequestCount == nil {
		m.workerRequestCount = prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Subsystem: sub,
			Name:      "request_count",
		}, basicLabels)
		if err := m.registry.Register(m.workerRequestCount); err != nil &&
			!errors.As(err, &prometheus.AlreadyRegisteredError{}) {
			panic(err)
		}
	}

	if m.workerQueueDepth == nil {
		m.workerQueueDepth = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "frankenphp",
			Subsystem: sub,
			Name:      "queue_depth",
		}, basicLabels)
		if err := m.registry.Register(m.workerQueueDepth); err != nil &&
			!errors.As(err, &prometheus.AlreadyRegisteredError{}) {
			panic(err)
		}
	}
}

func (m *PrometheusMetrics) TotalThreads(num int) {
	m.totalThreads.Add(float64(num))
}

func (m *PrometheusMetrics) StartRequest() {
	m.busyThreads.Inc()
}

func (m *PrometheusMetrics) StopRequest() {
	m.busyThreads.Dec()
}

func (m *PrometheusMetrics) StopWorkerRequest(server, name string, duration time.Duration) {
	if m.workerRequestTime == nil {
		return
	}

	m.workerRequestCount.WithLabelValues(server, name).Inc()
	m.busyWorkers.WithLabelValues(server, name).Dec()
	m.workerRequestTime.WithLabelValues(server, name).Add(duration.Seconds())
}

func (m *PrometheusMetrics) StartWorkerRequest(server, name string) {
	if m.busyWorkers == nil {
		return
	}
	m.busyWorkers.WithLabelValues(server, name).Inc()
}

func (m *PrometheusMetrics) QueuedWorkerRequest(server, name string) {
	if m.workerQueueDepth == nil {
		return
	}
	m.workerQueueDepth.WithLabelValues(server, name).Inc()
}

func (m *PrometheusMetrics) DequeuedWorkerRequest(server, name string) {
	if m.workerQueueDepth == nil {
		return
	}
	m.workerQueueDepth.WithLabelValues(server, name).Dec()
}

func (m *PrometheusMetrics) QueuedRequest() {
	m.queueDepth.Inc()
}

func (m *PrometheusMetrics) DequeuedRequest() {
	m.queueDepth.Dec()
}

func (m *PrometheusMetrics) Shutdown() {
	m.registry.Unregister(m.totalThreads)
	m.registry.Unregister(m.busyThreads)
	m.registry.Unregister(m.queueDepth)

	if m.totalWorkers != nil {
		m.registry.Unregister(m.totalWorkers)
		m.totalWorkers = nil
	}

	if m.busyWorkers != nil {
		m.registry.Unregister(m.busyWorkers)
		m.busyWorkers = nil
	}

	if m.workerRequestTime != nil {
		m.registry.Unregister(m.workerRequestTime)
		m.workerRequestTime = nil
	}

	if m.workerRequestCount != nil {
		m.registry.Unregister(m.workerRequestCount)
		m.workerRequestCount = nil
	}

	if m.workerCrashes != nil {
		m.registry.Unregister(m.workerCrashes)
		m.workerCrashes = nil
	}

	if m.workerRestarts != nil {
		m.registry.Unregister(m.workerRestarts)
		m.workerRestarts = nil
	}

	if m.readyWorkers != nil {
		m.registry.Unregister(m.readyWorkers)
		m.readyWorkers = nil
	}

	if m.workerQueueDepth != nil {
		m.registry.Unregister(m.workerQueueDepth)
		m.workerQueueDepth = nil
	}

	m.totalThreads = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "frankenphp_total_threads",
		Help: "Total number of PHP threads",
	})
	m.busyThreads = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "frankenphp_busy_threads",
		Help: "Number of busy PHP threads",
	})
	m.queueDepth = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "frankenphp_queue_depth",
		Help: "Number of regular queued requests",
	})

	if err := m.registry.Register(m.totalThreads); err != nil &&
		!errors.As(err, &prometheus.AlreadyRegisteredError{}) {
		panic(err)
	}

	if err := m.registry.Register(m.busyThreads); err != nil &&
		!errors.As(err, &prometheus.AlreadyRegisteredError{}) {
		panic(err)
	}

	if err := m.registry.Register(m.queueDepth); err != nil &&
		!errors.As(err, &prometheus.AlreadyRegisteredError{}) {
		panic(err)
	}
}

func NewPrometheusMetrics(registry prometheus.Registerer) *PrometheusMetrics {
	if registry == nil {
		registry = prometheus.NewRegistry()
	}

	m := &PrometheusMetrics{
		registry: registry,
		totalThreads: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "frankenphp_total_threads",
			Help: "Total number of PHP threads",
		}),
		busyThreads: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "frankenphp_busy_threads",
			Help: "Number of busy PHP threads",
		}),
		queueDepth: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "frankenphp_queue_depth",
			Help: "Number of regular queued requests",
		}),
		totalWorkers:       nil,
		busyWorkers:        nil,
		workerRequestTime:  nil,
		workerRequestCount: nil,
		workerRestarts:     nil,
		workerCrashes:      nil,
		readyWorkers:       nil,
		workerQueueDepth:   nil,
	}

	if err := m.registry.Register(m.totalThreads); err != nil &&
		!errors.As(err, &prometheus.AlreadyRegisteredError{}) {
		panic(err)
	}

	if err := m.registry.Register(m.busyThreads); err != nil &&
		!errors.As(err, &prometheus.AlreadyRegisteredError{}) {
		panic(err)
	}

	if err := m.registry.Register(m.queueDepth); err != nil &&
		!errors.As(err, &prometheus.AlreadyRegisteredError{}) {
		panic(err)
	}

	return m
}
