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
	// WorkerStalled reports the rolling 1/3/5-second fraction of time
	// the worker's request queue was non-empty. Each value is in [0,1].
	WorkerStalled(name string, win1s, win3s, win5s float64)
}

type nullMetrics struct{}

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

func (n nullMetrics) WorkerStalled(string, float64, float64, float64) {}

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
	workerStalled1s    *prometheus.GaugeVec
	workerStalled3s    *prometheus.GaugeVec
	workerStalled5s    *prometheus.GaugeVec
	queueDepth         prometheus.Gauge
	mu                 sync.Mutex
}

func (m *PrometheusMetrics) StartWorker(name string) {
	m.busyThreads.Inc()

	// tests do not register workers before starting them
	if m.totalWorkers == nil {
		return
	}

	m.totalWorkers.WithLabelValues(name).Inc()
}

func (m *PrometheusMetrics) ReadyWorker(name string) {
	if m.totalWorkers == nil {
		return
	}

	m.readyWorkers.WithLabelValues(name).Inc()
}

func (m *PrometheusMetrics) StopWorker(name string, reason StopReason) {
	m.busyThreads.Dec()

	// tests do not register workers before starting them
	if m.totalWorkers == nil {
		return
	}

	m.totalWorkers.WithLabelValues(name).Dec()

	// only decrement readyWorkers if the worker actually reached frankenphp_handle_request
	if reason != StopReasonBootFailure {
		m.readyWorkers.WithLabelValues(name).Dec()
	}

	switch reason {
	case StopReasonCrash, StopReasonBootFailure:
		m.workerCrashes.WithLabelValues(name).Inc()
	case StopReasonRestart:
		m.workerRestarts.WithLabelValues(name).Inc()
	}
}

func (m *PrometheusMetrics) TotalWorkers(string, int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	const ns, sub = "frankenphp", "worker"
	basicLabels := []string{"worker"}

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

	if m.workerStalled1s == nil {
		m.workerStalled1s = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns,
			Subsystem: sub,
			Name:      "stalled_1s",
			Help:      "Fraction of the last 1 second the worker's request queue was non-empty (0..1)",
		}, basicLabels)
		if err := m.registry.Register(m.workerStalled1s); err != nil &&
			!errors.As(err, &prometheus.AlreadyRegisteredError{}) {
			panic(err)
		}
	}

	if m.workerStalled3s == nil {
		m.workerStalled3s = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns,
			Subsystem: sub,
			Name:      "stalled_3s",
			Help:      "Fraction of the last 3 seconds the worker's request queue was non-empty (0..1)",
		}, basicLabels)
		if err := m.registry.Register(m.workerStalled3s); err != nil &&
			!errors.As(err, &prometheus.AlreadyRegisteredError{}) {
			panic(err)
		}
	}

	if m.workerStalled5s == nil {
		m.workerStalled5s = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns,
			Subsystem: sub,
			Name:      "stalled_5s",
			Help:      "Fraction of the last 5 seconds the worker's request queue was non-empty (0..1)",
		}, basicLabels)
		if err := m.registry.Register(m.workerStalled5s); err != nil &&
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

func (m *PrometheusMetrics) StopWorkerRequest(name string, duration time.Duration) {
	if m.workerRequestTime == nil {
		return
	}

	m.workerRequestCount.WithLabelValues(name).Inc()
	m.busyWorkers.WithLabelValues(name).Dec()
	m.workerRequestTime.WithLabelValues(name).Add(duration.Seconds())
}

func (m *PrometheusMetrics) StartWorkerRequest(name string) {
	if m.busyWorkers == nil {
		return
	}
	m.busyWorkers.WithLabelValues(name).Inc()
}

func (m *PrometheusMetrics) QueuedWorkerRequest(name string) {
	if m.workerQueueDepth == nil {
		return
	}
	m.workerQueueDepth.WithLabelValues(name).Inc()
}

func (m *PrometheusMetrics) DequeuedWorkerRequest(name string) {
	if m.workerQueueDepth == nil {
		return
	}
	m.workerQueueDepth.WithLabelValues(name).Dec()
}

func (m *PrometheusMetrics) WorkerStalled(name string, win1s, win3s, win5s float64) {
	if m.workerStalled1s == nil {
		return
	}
	m.workerStalled1s.WithLabelValues(name).Set(win1s)
	m.workerStalled3s.WithLabelValues(name).Set(win3s)
	m.workerStalled5s.WithLabelValues(name).Set(win5s)
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

	if m.workerStalled1s != nil {
		m.registry.Unregister(m.workerStalled1s)
		m.workerStalled1s = nil
	}

	if m.workerStalled3s != nil {
		m.registry.Unregister(m.workerStalled3s)
		m.workerStalled3s = nil
	}

	if m.workerStalled5s != nil {
		m.registry.Unregister(m.workerStalled5s)
		m.workerStalled5s = nil
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
		workerStalled1s:    nil,
		workerStalled3s:    nil,
		workerStalled5s:    nil,
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
