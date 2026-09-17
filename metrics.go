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
	StopReasonBootFailure // worker exited before reaching its ready point: frankenphp_handle_request, or frankenphp_worker_tick for background workers
)

type StopReason int

// Metrics reports what the workers and the threads of a FrankenPHP instance
// are doing. A worker is identified by two values: its declared name, and
// the name of the server it is scoped to, empty for a global worker. They
// stay apart rather than packed into one string, so a series keyed on the
// name alone still selects the worker of every server.
type Metrics interface {
	// StartWorker collects started workers
	StartWorker(name, server string)
	// ReadyWorker collects ready workers
	ReadyWorker(name, server string)
	// StopWorker collects stopped workers
	StopWorker(name, server string, reason StopReason)
	// TotalWorkers collects expected workers
	TotalWorkers(name, server string, num int)
	// TotalThreads collects total threads
	TotalThreads(num int)
	// StartRequest collects started requests
	StartRequest()
	// StopRequest collects stopped requests
	StopRequest()
	// StopWorkerRequest collects stopped worker requests
	StopWorkerRequest(name, server string, duration time.Duration)
	// StartWorkerRequest collects started worker requests
	StartWorkerRequest(name, server string)
	Shutdown()
	QueuedWorkerRequest(name, server string)
	DequeuedWorkerRequest(name, server string)
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
	totalThreads       prometheus.Gauge
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
	mu                 sync.RWMutex
}

// mustRegister registers c, tolerating a collector that is already registered.
func (m *PrometheusMetrics) mustRegister(c prometheus.Collector) {
	if err := m.registry.Register(c); err != nil {
		if _, ok := errors.AsType[prometheus.AlreadyRegisteredError](err); !ok {
			panic(err)
		}
	}
}

func (m *PrometheusMetrics) StartWorker(name, server string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.busyThreads.Inc()

	// tests do not register workers before starting them
	if m.totalWorkers == nil {
		return
	}

	m.totalWorkers.WithLabelValues(name, server).Inc()
}

func (m *PrometheusMetrics) ReadyWorker(name, server string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.totalWorkers == nil {
		return
	}

	m.readyWorkers.WithLabelValues(name, server).Inc()
}

func (m *PrometheusMetrics) StopWorker(name, server string, reason StopReason) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.busyThreads.Dec()

	// tests do not register workers before starting them
	if m.totalWorkers == nil {
		return
	}

	m.totalWorkers.WithLabelValues(name, server).Dec()

	// only decrement readyWorkers if the worker actually reached its ready point
	if reason != StopReasonBootFailure {
		m.readyWorkers.WithLabelValues(name, server).Dec()
	}

	switch reason {
	case StopReasonCrash, StopReasonBootFailure:
		m.workerCrashes.WithLabelValues(name, server).Inc()
	case StopReasonRestart:
		m.workerRestarts.WithLabelValues(name, server).Inc()
	}
}

func (m *PrometheusMetrics) TotalWorkers(string, string, int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	const ns, sub = "frankenphp", "worker"
	// a worker of a php_server keeps its declared name, the block it belongs
	// to is a label of its own, empty for a global worker
	basicLabels := []string{"worker", "server"}

	if m.totalWorkers == nil {
		m.totalWorkers = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "total_workers",
			Help:      "Total number of PHP workers for this worker",
		}, basicLabels)
		m.mustRegister(m.totalWorkers)
	}

	if m.readyWorkers == nil {
		m.readyWorkers = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "ready_workers",
			Help:      "Running workers that have reached their ready point at least once: frankenphp_handle_request for HTTP workers, frankenphp_worker_tick for background workers",
		}, basicLabels)
		m.mustRegister(m.readyWorkers)
	}

	if m.busyWorkers == nil {
		m.busyWorkers = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "busy_workers",
			Help:      "Number of busy PHP workers for this worker",
		}, basicLabels)
		m.mustRegister(m.busyWorkers)
	}

	if m.workerCrashes == nil {
		m.workerCrashes = prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Subsystem: sub,
			Name:      "crashes",
			Help:      "Number of PHP worker crashes for this worker",
		}, basicLabels)
		m.mustRegister(m.workerCrashes)
	}

	if m.workerRestarts == nil {
		m.workerRestarts = prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Subsystem: sub,
			Name:      "restarts",
			Help:      "Number of PHP worker restarts for this worker",
		}, basicLabels)
		m.mustRegister(m.workerRestarts)
	}

	if m.workerRequestTime == nil {
		m.workerRequestTime = prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Subsystem: sub,
			Name:      "request_time",
		}, basicLabels)
		m.mustRegister(m.workerRequestTime)
	}

	if m.workerRequestCount == nil {
		m.workerRequestCount = prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Subsystem: sub,
			Name:      "request_count",
		}, basicLabels)
		m.mustRegister(m.workerRequestCount)
	}

	if m.workerQueueDepth == nil {
		m.workerQueueDepth = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "frankenphp",
			Subsystem: sub,
			Name:      "queue_depth",
		}, basicLabels)
		m.mustRegister(m.workerQueueDepth)
	}
}

func (m *PrometheusMetrics) TotalThreads(num int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.totalThreads.Set(float64(num))
}

func (m *PrometheusMetrics) StartRequest() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.busyThreads.Inc()
}

func (m *PrometheusMetrics) StopRequest() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.busyThreads.Dec()
}

func (m *PrometheusMetrics) StopWorkerRequest(name, server string, duration time.Duration) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.workerRequestTime == nil {
		return
	}

	m.workerRequestCount.WithLabelValues(name, server).Inc()
	m.busyWorkers.WithLabelValues(name, server).Dec()
	m.workerRequestTime.WithLabelValues(name, server).Add(duration.Seconds())
}

func (m *PrometheusMetrics) StartWorkerRequest(name, server string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.busyWorkers == nil {
		return
	}
	m.busyWorkers.WithLabelValues(name, server).Inc()
}

func (m *PrometheusMetrics) QueuedWorkerRequest(name, server string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.workerQueueDepth == nil {
		return
	}
	m.workerQueueDepth.WithLabelValues(name, server).Inc()
}

func (m *PrometheusMetrics) DequeuedWorkerRequest(name, server string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.workerQueueDepth == nil {
		return
	}
	m.workerQueueDepth.WithLabelValues(name, server).Dec()
}

func (m *PrometheusMetrics) QueuedRequest() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.queueDepth.Inc()
}

func (m *PrometheusMetrics) DequeuedRequest() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.queueDepth.Dec()
}

func (m *PrometheusMetrics) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.registry.Unregister(m.totalThreads)
	m.registry.Unregister(m.busyThreads)
	m.registry.Unregister(m.queueDepth)

	if m.totalWorkers != nil {
		m.registry.Unregister(m.totalWorkers)
	}

	if m.busyWorkers != nil {
		m.registry.Unregister(m.busyWorkers)
	}

	if m.workerRequestTime != nil {
		m.registry.Unregister(m.workerRequestTime)
	}

	if m.workerRequestCount != nil {
		m.registry.Unregister(m.workerRequestCount)
	}

	if m.workerCrashes != nil {
		m.registry.Unregister(m.workerCrashes)
	}

	if m.workerRestarts != nil {
		m.registry.Unregister(m.workerRestarts)
	}

	if m.readyWorkers != nil {
		m.registry.Unregister(m.readyWorkers)
	}

	if m.workerQueueDepth != nil {
		m.registry.Unregister(m.workerQueueDepth)
	}
}

func NewPrometheusMetrics(registry prometheus.Registerer) *PrometheusMetrics {
	if registry == nil {
		registry = prometheus.NewRegistry()
	}

	m := &PrometheusMetrics{
		registry: registry,
		totalThreads: prometheus.NewGauge(prometheus.GaugeOpts{
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

	m.mustRegister(m.totalThreads)

	m.mustRegister(m.busyThreads)

	m.mustRegister(m.queueDepth)

	return m
}
