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
// are doing. Every method naming a worker takes the identifier declared
// through DeclareWorker. An implementation that also satisfies
// OpcacheMetrics is told about opcache restarts as well.
type Metrics interface {
	// DeclareWorker gives the labels of a worker, before any other method
	// mentions it: id is the identifier those methods use, name the name the
	// worker was declared with and server the php_server it belongs to,
	// empty for a global worker
	DeclareWorker(id, name, server string)
	// StartWorker collects started workers
	StartWorker(id string)
	// ReadyWorker collects ready workers
	ReadyWorker(id string)
	// StopWorker collects stopped workers
	StopWorker(id string, reason StopReason)
	// TotalWorkers collects expected workers
	TotalWorkers(id string, num int)
	// TotalThreads collects total threads
	TotalThreads(num int)
	// StartRequest collects started requests
	StartRequest()
	// StopRequest collects stopped requests
	StopRequest()
	// StopWorkerRequest collects stopped worker requests
	StopWorkerRequest(id string, duration time.Duration)
	// StartWorkerRequest collects started worker requests
	StartWorkerRequest(id string)
	Shutdown()
	QueuedWorkerRequest(id string)
	DequeuedWorkerRequest(id string)
	QueuedRequest()
	DequeuedRequest()
}

// OpcacheMetrics is the optional part of a Metrics implementation that counts
// the restarts of opcache's shared memory, by reason, where the build reports
// them (ZTS, PHP 8.4 and up). An implementation passed to WithMetrics() that
// lacks it only misses the counter, the restart is logged either way.
type OpcacheMetrics interface {
	OpcacheRestart(reason string)
}

type nullMetrics struct{}

func (n nullMetrics) DeclareWorker(string, string, string) {
}

func (n nullMetrics) StartWorker(string) {
}

func (n nullMetrics) ReadyWorker(string) {
}

func (n nullMetrics) StopWorker(string, StopReason) {
}

func (n nullMetrics) TotalWorkers(string, int) {
}

func (n nullMetrics) TotalThreads(int) {
}

func (n nullMetrics) StartRequest() {
}

func (n nullMetrics) StopRequest() {
}

func (n nullMetrics) StopWorkerRequest(string, time.Duration) {
}

func (n nullMetrics) StartWorkerRequest(string) {
}

func (n nullMetrics) Shutdown() {
}

func (n nullMetrics) QueuedWorkerRequest(string) {}

func (n nullMetrics) DequeuedWorkerRequest(string) {}

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
	opcacheRestarts    *prometheus.CounterVec
	// declaredWorkers maps the identifier of a worker to its label values,
	// see DeclareWorker
	declaredWorkers map[string][2]string
	mu              sync.RWMutex
}

// DeclareWorker records the labels of a worker, see the Metrics interface.
func (m *PrometheusMetrics) DeclareWorker(id, name, server string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.declaredWorkers == nil {
		m.declaredWorkers = make(map[string][2]string)
	}
	m.declaredWorkers[id] = [2]string{name, server}
}

// workerLabels resolves the identifier of a worker into its label values,
// the identifier itself and no server when it was never declared. Called
// with m.mu held.
func (m *PrometheusMetrics) workerLabels(id string) (string, string) {
	if labels, ok := m.declaredWorkers[id]; ok {
		return labels[0], labels[1]
	}

	return id, ""
}

// mustRegister registers c, tolerating a collector that is already registered.
func (m *PrometheusMetrics) mustRegister(c prometheus.Collector) {
	if err := m.registry.Register(c); err != nil {
		if _, ok := errors.AsType[prometheus.AlreadyRegisteredError](err); !ok {
			panic(err)
		}
	}
}

var (
	_ Metrics        = (*PrometheusMetrics)(nil)
	_ OpcacheMetrics = (*PrometheusMetrics)(nil)
)

func (m *PrometheusMetrics) StartWorker(id string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	name, server := m.workerLabels(id)

	m.busyThreads.Inc()

	// tests do not register workers before starting them
	if m.totalWorkers == nil {
		return
	}

	m.totalWorkers.WithLabelValues(name, server).Inc()
}

func (m *PrometheusMetrics) ReadyWorker(id string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	name, server := m.workerLabels(id)

	if m.totalWorkers == nil {
		return
	}

	m.readyWorkers.WithLabelValues(name, server).Inc()
}

func (m *PrometheusMetrics) StopWorker(id string, reason StopReason) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	name, server := m.workerLabels(id)

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

func (m *PrometheusMetrics) TotalWorkers(string, int) {
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

func (m *PrometheusMetrics) StopWorkerRequest(id string, duration time.Duration) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	name, server := m.workerLabels(id)

	if m.workerRequestTime == nil {
		return
	}

	m.workerRequestCount.WithLabelValues(name, server).Inc()
	m.busyWorkers.WithLabelValues(name, server).Dec()
	m.workerRequestTime.WithLabelValues(name, server).Add(duration.Seconds())
}

func (m *PrometheusMetrics) StartWorkerRequest(id string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	name, server := m.workerLabels(id)

	if m.busyWorkers == nil {
		return
	}
	m.busyWorkers.WithLabelValues(name, server).Inc()
}

func (m *PrometheusMetrics) QueuedWorkerRequest(id string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	name, server := m.workerLabels(id)

	if m.workerQueueDepth == nil {
		return
	}
	m.workerQueueDepth.WithLabelValues(name, server).Inc()
}

func (m *PrometheusMetrics) DequeuedWorkerRequest(id string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	name, server := m.workerLabels(id)

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

func (m *PrometheusMetrics) OpcacheRestart(reason string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.opcacheRestarts != nil {
		m.opcacheRestarts.WithLabelValues(reason).Inc()
	}
}

func (m *PrometheusMetrics) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.declaredWorkers = nil

	m.registry.Unregister(m.totalThreads)
	m.registry.Unregister(m.busyThreads)
	m.registry.Unregister(m.queueDepth)

	if m.opcacheRestarts != nil {
		m.registry.Unregister(m.opcacheRestarts)
	}

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
		// experimental: to be removed once opcache handles restarts safely under ZTS
		opcacheRestarts: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "frankenphp_opcache_restarts",
			Help: "Number of restarts of opcache's shared memory scheduled, by reason (experimental, should stay at zero)",
		}, []string{"reason"}),
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

	// only where the hook exists: a series stuck at zero would read as "no
	// restart" on a build that cannot report one
	if opcacheRestartHook {
		m.mustRegister(m.opcacheRestarts)

		// expose the series at zero so a rate or an alert on it works from the
		// first restart on, instead of missing it for lack of a previous sample
		for _, reason := range opcacheRestartReasons {
			m.opcacheRestarts.WithLabelValues(reason)
		}
	}

	return m
}
