package frankenphp

import (
	"fmt"
	"log/slog"
	"time"
)

// defaultMaxConsecutiveFailures is the default maximum number of consecutive failures before panicking
const defaultMaxConsecutiveFailures = 6

// Option instances allow to configure FrankenPHP.
type Option func(h *opt) error

// WorkerOption instances allow configuring FrankenPHP worker.
type WorkerOption func(*workerOpt) error

// opt contains the available options.
//
// If you change this, also update the Caddy module and the documentation.
type opt struct {
	numThreads  int
	maxThreads  int
	workers     []workerOpt
	logger      *slog.Logger
	metrics     Metrics
	phpIni      map[string]string
	maxWaitTime time.Duration
}

type workerOpt struct {
	name                   string
	fileName               string
	num                    int
	maxThreads             int
	env                    PreparedEnv
	watch                  []string
	maxConsecutiveFailures int
	onThreadReady          func(int)
	onThreadShutdown       func(int)
	onServerStartup        func()
	onServerShutdown       func()
}

// WithNumThreads configures the number of PHP threads to start.
func WithNumThreads(numThreads int) Option {
	return func(o *opt) error {
		o.numThreads = numThreads

		return nil
	}
}

func WithMaxThreads(maxThreads int) Option {
	return func(o *opt) error {
		o.maxThreads = maxThreads

		return nil
	}
}

func WithMetrics(m Metrics) Option {
	return func(o *opt) error {
		o.metrics = m

		return nil
	}
}

// WithWorkers configures the PHP workers to start
func WithWorkers(name string, fileName string, num int, options ...WorkerOption) Option {
	return func(o *opt) error {
		worker := workerOpt{
			name:                   name,
			fileName:               fileName,
			num:                    num,
			env:                    PrepareEnv(nil),
			watch:                  []string{},
			maxConsecutiveFailures: defaultMaxConsecutiveFailures,
		}

		for _, option := range options {
			if err := option(&worker); err != nil {
				return err
			}
		}

		o.workers = append(o.workers, worker)

		return nil
	}
}

// WithWorkerEnv sets environment variables for the worker
func WithWorkerEnv(env map[string]string) WorkerOption {
	return func(w *workerOpt) error {
		w.env = PrepareEnv(env)

		return nil
	}
}

// WithWorkerEnv sets environment variables for the worker
func WithWorkerMaxThreads(num int) WorkerOption {
	return func(w *workerOpt) error {
		w.maxThreads = num

		return nil
	}
}

// WithWorkerWatchMode sets directories to watch for file changes
func WithWorkerWatchMode(watch []string) WorkerOption {
	return func(w *workerOpt) error {
		w.watch = watch

		return nil
	}
}

// WithWorkerMaxFailures sets the maximum number of consecutive failures before panicking
func WithWorkerMaxFailures(maxFailures int) WorkerOption {
	return func(w *workerOpt) error {
		if maxFailures < -1 {
			return fmt.Errorf("max consecutive failures must be >= -1, got %d", maxFailures)
		}
		w.maxConsecutiveFailures = maxFailures

		return nil
	}
}

func WithWorkerOnReady(f func(int)) WorkerOption {
	return func(w *workerOpt) error {
		w.onThreadReady = f

		return nil
	}
}

func WithWorkerOnShutdown(f func(int)) WorkerOption {
	return func(w *workerOpt) error {
		w.onThreadShutdown = f

		return nil
	}
}

// WithWorkerOnServerStartup adds a function to be called right after server startup. Useful for extensions.
func WithWorkerOnServerStartup(f func()) WorkerOption {
	return func(w *workerOpt) error {
		w.onServerStartup = f

		return nil
	}
}

// WithWorkerOnServerShutdown adds a function to be called right before server shutdown. Useful for extensions.
func WithWorkerOnServerShutdown(f func()) WorkerOption {
	return func(w *workerOpt) error {
		w.onServerShutdown = f

		return nil
	}
}

// WithLogger configures the global logger to use.
func WithLogger(l *slog.Logger) Option {
	return func(o *opt) error {
		o.logger = l

		return nil
	}
}

// WithPhpIni configures user defined PHP ini settings.
func WithPhpIni(overrides map[string]string) Option {
	return func(o *opt) error {
		o.phpIni = overrides
		return nil
	}
}

// WithMaxWaitTime configures the max time a request may be stalled waiting for a thread.
func WithMaxWaitTime(maxWaitTime time.Duration) Option {
	return func(o *opt) error {
		o.maxWaitTime = maxWaitTime

		return nil
	}
}
