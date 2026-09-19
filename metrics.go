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
	StopReasonBootFailure // worker exited before reaching its ready point: frankenphp_handle_request, or WorkerHandle::tick() for background workers
)

type StopReason int

// Metrics reports what the workers and the threads of a FrankenPHP instance
// are doing. A worker is identified by its name alone, where a worker scoped
// to a server is reported as "<server name>:<name>". An implementation that
// also satisfies ServerMetrics gets the two apart instead.
type Metrics interface {
	// StartWorker collects started workers
	StartWorker(name string)
	// ReadyWorker collects ready workers
	ReadyWorker(name string)
	// StopWorker collects stopped workers
	StopWorker(name string, reason StopReason)
	// TotalWorkers collects expected workers
	TotalWorkers(name string, num int)
	// TotalThreads collects total threads
	TotalThreads(num int)
	// StartRequest collects started requests
	StartRequest()
	// StopRequest collects stopped requests
	StopRequest()
	// StopWorkerRequest collects stopped worker requests
	StopWorkerRequest(name string, duration time.Duration)
	// StartWorkerRequest collects started worker requests
	StartWorkerRequest(name string)
	Shutdown()
	QueuedWorkerRequest(name string)
	DequeuedWorkerRequest(name string)
	QueuedRequest()
	DequeuedRequest()
}

// ServerMetrics is the optional part of a Metrics implementation that keeps
// the name of a worker and the name of its server apart, the latter empty
// for a global worker, so a series keyed on the name alone still selects
// the worker of every server. When the implementation passed to
// WithMetrics() satisfies it, the runtime reports workers through these
// methods and never through the worker methods of Metrics.
type ServerMetrics interface {
	StartWorkerOnServer(name, server string)
	ReadyWorkerOnServer(name, server string)
	StopWorkerOnServer(name, server string, reason StopReason)
	TotalWorkersOnServer(name, server string, num int)
	StopWorkerRequestOnServer(name, server string, duration time.Duration)
	StartWorkerRequestOnServer(name, server string)
	QueuedWorkerRequestOnServer(name, server string)
	DequeuedWorkerRequestOnServer(name, server string)
}

// workerMetrics is what the runtime reports on: Metrics with the worker
// methods taking the server as well
type workerMetrics interface {
	StartWorker(name, server string)
	ReadyWorker(name, server string)
	StopWorker(name, server string, reason StopReason)
	TotalWorkers(name, server string, num int)
	TotalThreads(num int)
	StartRequest()
	StopRequest()
	StopWorkerRequest(name, server string, duration time.Duration)
	StartWorkerRequest(name, server string)
	Shutdown()
	QueuedWorkerRequest(name, server string)
	DequeuedWorkerRequest(name, server string)
	QueuedRequest()
	DequeuedRequest()
}

// metricsAdapter routes the worker methods to ServerMetrics when the
// implementation has it, and packs the server into the name otherwise
type metricsAdapter struct {
	Metrics
	server ServerMetrics
}

func adaptMetrics(m Metrics) workerMetrics {
	a := metricsAdapter{Metrics: m}
	a.server, _ = m.(ServerMetrics)

	return a
}

func packedWorkerName(name, server string) string {
	if server == "" {
		return name
	}

	return server + ":" + name
}

func (a metricsAdapter) StartWorker(name, server string) {
	if a.server != nil {
		a.server.StartWorkerOnServer(name, server)
		return
	}
	a.Metrics.StartWorker(packedWorkerName(name, server))
}

func (a metricsAdapter) ReadyWorker(name, server string) {
	if a.server != nil {
		a.server.ReadyWorkerOnServer(name, server)
		return
	}
	a.Metrics.ReadyWorker(packedWorkerName(name, server))
}

func (a metricsAdapter) StopWorker(name, server string, reason StopReason) {
	if a.server != nil {
		a.server.StopWorkerOnServer(name, server, reason)
		return
	}
	a.Metrics.StopWorker(packedWorkerName(name, server), reason)
}

func (a metricsAdapter) TotalWorkers(name, server string, num int) {
	if a.server != nil {
		a.server.TotalWorkersOnServer(name, server, num)
		return
	}
	a.Metrics.TotalWorkers(packedWorkerName(name, server), num)
}

func (a metricsAdapter) StopWorkerRequest(name, server string, duration time.Duration) {
	if a.server != nil {
		a.server.StopWorkerRequestOnServer(name, server, duration)
		return
	}
	a.Metrics.StopWorkerRequest(packedWorkerName(name, server), duration)
}

func (a metricsAdapter) StartWorkerRequest(name, server string) {
	if a.server != nil {
		a.server.StartWorkerRequestOnServer(name, server)
		return
	}
	a.Metrics.StartWorkerRequest(packedWorkerName(name, server))
}

func (a metricsAdapter) QueuedWorkerRequest(name, server string) {
	if a.server != nil {
		a.server.QueuedWorkerRequestOnServer(name, server)
		return
	}
	a.Metrics.QueuedWorkerRequest(packedWorkerName(name, server))
}

func (a metricsAdapter) DequeuedWorkerRequest(name, server string) {
	if a.server != nil {
		a.server.DequeuedWorkerRequestOnServer(name, server)
		return
	}
	a.Metrics.DequeuedWorkerRequest(packedWorkerName(name, server))
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

var (
	_ Metrics       = (*PrometheusMetrics)(nil)
	_ ServerMetrics = (*PrometheusMetrics)(nil)
)

func (m *PrometheusMetrics) StartWorker(name string) { m.StartWorkerOnServer(name, "") }

func (m *PrometheusMetrics) ReadyWorker(name string) { m.ReadyWorkerOnServer(name, "") }

func (m *PrometheusMetrics) StopWorker(name string, reason StopReason) {
	m.StopWorkerOnServer(name, "", reason)
}

func (m *PrometheusMetrics) TotalWorkers(name string, num int) { m.TotalWorkersOnServer(name, "", num) }

func (m *PrometheusMetrics) StopWorkerRequest(name string, duration time.Duration) {
	m.StopWorkerRequestOnServer(name, "", duration)
}

func (m *PrometheusMetrics) StartWorkerRequest(name string) { m.StartWorkerRequestOnServer(name, "") }

func (m *PrometheusMetrics) QueuedWorkerRequest(name string) { m.QueuedWorkerRequestOnServer(name, "") }

func (m *PrometheusMetrics) DequeuedWorkerRequest(name string) {
	m.DequeuedWorkerRequestOnServer(name, "")
}

func (m *PrometheusMetrics) StartWorkerOnServer(name, server string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.busyThreads.Inc()

	// tests do not register workers before starting them
	if m.totalWorkers == nil {
		return
	}

	m.totalWorkers.WithLabelValues(name, server).Inc()
}

func (m *PrometheusMetrics) ReadyWorkerOnServer(name, server string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.totalWorkers == nil {
		return
	}

	m.readyWorkers.WithLabelValues(name, server).Inc()
}

func (m *PrometheusMetrics) StopWorkerOnServer(name, server string, reason StopReason) {
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

func (m *PrometheusMetrics) TotalWorkersOnServer(string, string, int) {
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
			Help:      "Running workers that have reached their ready point at least once: frankenphp_handle_request for HTTP workers, WorkerHandle::tick() for background workers",
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

func (m *PrometheusMetrics) StopWorkerRequestOnServer(name, server string, duration time.Duration) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.workerRequestTime == nil {
		return
	}

	m.workerRequestCount.WithLabelValues(name, server).Inc()
	m.busyWorkers.WithLabelValues(name, server).Dec()
	m.workerRequestTime.WithLabelValues(name, server).Add(duration.Seconds())
}

func (m *PrometheusMetrics) StartWorkerRequestOnServer(name, server string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.busyWorkers == nil {
		return
	}
	m.busyWorkers.WithLabelValues(name, server).Inc()
}

func (m *PrometheusMetrics) QueuedWorkerRequestOnServer(name, server string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.workerQueueDepth == nil {
		return
	}
	m.workerQueueDepth.WithLabelValues(name, server).Inc()
}

func (m *PrometheusMetrics) DequeuedWorkerRequestOnServer(name, server string) {
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
