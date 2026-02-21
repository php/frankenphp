package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dunglas/frankenphp/internal/fastabs"
	"github.com/dunglas/frankenphp/internal/state"
)

// represents a worker script and can have many threads assigned to it
type worker struct {
	mercureContext

	name                   string
	fileName               string
	num                    int
	maxThreads             int
	requestOptions         []RequestOption
	requestChan            chan contextHolder
	threads                []*phpThread
	threadMutex            sync.RWMutex
	allowPathMatching      bool
	maxConsecutiveFailures int
	onThreadReady          func(int)
	onThreadShutdown       func(int)
	queuedRequests         atomic.Int32
}

var (
	workers          []*worker
	workersByName    map[string]*worker
	workersByPath    map[string]*worker
	watcherIsEnabled bool
	startupFailChan  chan error
)

func initWorkers(opt []workerOpt) error {
	if len(opt) == 0 {
		return nil
	}

	var (
		workersReady        sync.WaitGroup
		totalThreadsToStart int
	)

	workers = make([]*worker, 0, len(opt))
	workersByName = make(map[string]*worker, len(opt))
	workersByPath = make(map[string]*worker, len(opt))

	for _, o := range opt {
		w, err := newWorker(o)
		if err != nil {
			return err
		}

		totalThreadsToStart += w.num
		workers = append(workers, w)
		workersByName[w.name] = w
		if w.allowPathMatching {
			workersByPath[w.fileName] = w
		}
	}

	startupFailChan = make(chan error, totalThreadsToStart)

	for _, w := range workers {
		for i := 0; i < w.num; i++ {
			thread := getInactivePHPThread()
			convertToWorkerThread(thread, w)

			workersReady.Go(func() {
				thread.state.WaitFor(state.Ready, state.ShuttingDown, state.Done)
			})
		}
	}

	workersReady.Wait()

	select {
	case err := <-startupFailChan:
		// at least 1 worker has failed, return an error
		return fmt.Errorf("failed to initialize workers: %w", err)
	default:
		// all workers started successfully
		startupFailChan = nil
	}

	return nil
}

func newWorker(o workerOpt) (*worker, error) {
	// Order is important!
	// This order ensures that FrankenPHP started from inside a symlinked directory will properly resolve any paths.
	// If it is started from outside a symlinked directory, it is resolved to the same path that we use in the Caddy module.
	absFileName, err := filepath.EvalSymlinks(o.fileName)
	if err != nil {
		return nil, fmt.Errorf("worker filename is invalid %q: %w", o.fileName, err)
	}

	absFileName, err = fastabs.FastAbs(absFileName)
	if err != nil {
		return nil, fmt.Errorf("worker filename is invalid %q: %w", o.fileName, err)
	}

	if _, err := os.Stat(absFileName); err != nil {
		return nil, fmt.Errorf("worker file not found %q: %w", absFileName, err)
	}

	if o.name == "" {
		o.name = absFileName
	}

	// workers that have a name starting with "m#" are module workers
	// they can only be matched by their name, not by their path
	allowPathMatching := !strings.HasPrefix(o.name, "m#")

	if w := workersByPath[absFileName]; w != nil && allowPathMatching {
		return w, fmt.Errorf("two workers cannot have the same filename: %q", absFileName)
	}
	if w := workersByName[o.name]; w != nil {
		return w, fmt.Errorf("two workers cannot have the same name: %q", o.name)
	}

	if o.env == nil {
		o.env = make(PreparedEnv, 1)
	}

	o.env["FRANKENPHP_WORKER\x00"] = "1"
	w := &worker{
		name:                   o.name,
		fileName:               absFileName,
		requestOptions:         o.requestOptions,
		num:                    o.num,
		maxThreads:             o.maxThreads,
		requestChan:            make(chan contextHolder),
		threads:                make([]*phpThread, 0, o.num),
		allowPathMatching:      allowPathMatching,
		maxConsecutiveFailures: o.maxConsecutiveFailures,
		onThreadReady:          o.onThreadReady,
		onThreadShutdown:       o.onThreadShutdown,
	}

	w.configureMercure(&o)

	w.requestOptions = append(
		w.requestOptions,
		WithRequestDocumentRoot(filepath.Dir(o.fileName), false),
		WithRequestPreparedEnv(o.env),
	)

	if o.extensionWorkers != nil {
		o.extensionWorkers.internalWorker = w
	}

	return w, nil
}

// EXPERIMENTAL: DrainWorkers finishes all worker scripts before a graceful shutdown
func DrainWorkers() {
	_ = drainWorkerThreads()
}

func drainWorkerThreads() []*phpThread {
	var (
		ready          sync.WaitGroup
		drainedThreads []*phpThread
	)

	for _, worker := range workers {
		worker.threadMutex.RLock()
		ready.Add(len(worker.threads))

		for _, thread := range worker.threads {
			if !thread.state.RequestSafeStateChange(state.Restarting) {
				ready.Done()

				// no state change allowed == thread is shutting down
				// we'll proceed to restart all other threads anyway
				continue
			}

			close(thread.drainChan)
			drainedThreads = append(drainedThreads, thread)

			go func(thread *phpThread) {
				thread.state.WaitFor(state.Yielding)
				ready.Done()
			}(thread)
		}

		worker.threadMutex.RUnlock()
	}

	ready.Wait()

	return drainedThreads
}

// RestartWorkers attempts to restart all workers gracefully
// All workers must be restarted at the same time to prevent issues with opcache resetting.
func RestartWorkers() {
	// disallow scaling threads while restarting workers
	scalingMu.Lock()
	defer scalingMu.Unlock()

	threadsToRestart := drainWorkerThreads()

	for _, thread := range threadsToRestart {
		thread.drainChan = make(chan struct{})
		thread.state.Set(state.Ready)
	}
}

func (worker *worker) attachThread(thread *phpThread) {
	worker.threadMutex.Lock()
	worker.threads = append(worker.threads, thread)
	worker.threadMutex.Unlock()
}

func (worker *worker) detachThread(thread *phpThread) {
	worker.threadMutex.Lock()
	for i, t := range worker.threads {
		if t == thread {
			worker.threads = append(worker.threads[:i], worker.threads[i+1:]...)
			break
		}
	}
	worker.threadMutex.Unlock()
}

func (worker *worker) countThreads() int {
	worker.threadMutex.RLock()
	l := len(worker.threads)
	worker.threadMutex.RUnlock()

	return l
}

// check if max_threads has been reached
func (worker *worker) isAtThreadLimit() bool {
	if worker.maxThreads <= 0 {
		return false
	}

	worker.threadMutex.RLock()
	atMaxThreads := len(worker.threads) >= worker.maxThreads
	worker.threadMutex.RUnlock()

	return atMaxThreads
}

func (worker *worker) handleRequest(ch contextHolder) error {
	metrics.StartWorkerRequest(worker.name)

	runtime.Gosched()

	if worker.queuedRequests.Load() == 0 {
		// dispatch requests to all worker threads in order
		worker.threadMutex.RLock()
		for _, thread := range worker.threads {
			select {
			case thread.requestChan <- ch:
				worker.threadMutex.RUnlock()
				<-ch.frankenPHPContext.done
				metrics.StopWorkerRequest(worker.name, time.Since(ch.frankenPHPContext.startedAt))

				return nil
			default:
				// thread is busy, continue
			}
		}
		worker.threadMutex.RUnlock()
	}

	// if no thread was available, mark the request as queued and apply the scaling strategy
	worker.queuedRequests.Add(1)
	metrics.QueuedWorkerRequest(worker.name)

	for {
		workerScaleChan := scaleChan
		if worker.isAtThreadLimit() {
			workerScaleChan = nil // max_threads for this worker reached, do not attempt scaling
		}

		select {
		case worker.requestChan <- ch:
			worker.queuedRequests.Add(-1)
			metrics.DequeuedWorkerRequest(worker.name)
			<-ch.frankenPHPContext.done
			metrics.StopWorkerRequest(worker.name, time.Since(ch.frankenPHPContext.startedAt))

			return nil
		case workerScaleChan <- ch.frankenPHPContext:
			// the request has triggered scaling, continue to wait for a thread
		case <-timeoutChan(maxWaitTime):
			// the request has timed out stalling
			worker.queuedRequests.Add(-1)
			metrics.DequeuedWorkerRequest(worker.name)
			metrics.StopWorkerRequest(worker.name, time.Since(ch.frankenPHPContext.startedAt))

			ch.frankenPHPContext.reject(ErrMaxWaitTimeExceeded)

			return ErrMaxWaitTimeExceeded
		}
	}
}
