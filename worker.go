package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/dunglas/frankenphp/internal/fastabs"
	"github.com/dunglas/frankenphp/internal/watcher"
	"golang.org/x/sync/semaphore"
)

// represents a worker script and can have many threads assigned to it
type worker struct {
	name                   string
	fileName               string
	num                    int
	maxThreads             int
	env                    PreparedEnv
	requestChan            chan contextHolder
	semaphore              *semaphore.Weighted
	threads                []*phpThread
	threadMutex            sync.RWMutex
	threadPool             sync.Pool // Pool of idle worker threads for direct handoff
	allowPathMatching      bool
	maxConsecutiveFailures int
	onThreadReady          func(int)
	onThreadShutdown       func(int)
}

var (
	workers          []*worker
	watcherIsEnabled bool
)

func initWorkers(opt []workerOpt) error {
	workers = make([]*worker, 0, len(opt))
	directoriesToWatch := getDirectoriesToWatch(opt)
	watcherIsEnabled = len(directoriesToWatch) > 0

	for _, o := range opt {
		w, err := newWorker(o)
		if err != nil {
			return err
		}
		workers = append(workers, w)
	}

	var workersReady sync.WaitGroup

	for _, w := range workers {
		for i := 0; i < w.num; i++ {
			thread := getInactivePHPThread()
			convertToWorkerThread(thread, w)

			workersReady.Go(func() {
				thread.state.waitFor(stateReady)
			})
		}
	}

	workersReady.Wait()

	if !watcherIsEnabled {
		return nil
	}

	watcherIsEnabled = true
	if err := watcher.InitWatcher(globalCtx, directoriesToWatch, RestartWorkers, globalLogger); err != nil {
		return err
	}

	return nil
}

func getWorkerByName(name string) *worker {
	for _, w := range workers {
		if w.name == name {
			return w
		}
	}

	return nil
}

func getWorkerByPath(path string) *worker {
	for _, w := range workers {
		if w.fileName == path && w.allowPathMatching {
			return w
		}
	}

	return nil
}

func newWorker(o workerOpt) (*worker, error) {
	absFileName, err := fastabs.FastAbs(o.fileName)
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

	if w := getWorkerByPath(absFileName); w != nil && allowPathMatching {
		return w, fmt.Errorf("two workers cannot have the same filename: %q", absFileName)
	}
	if w := getWorkerByName(o.name); w != nil {
		return w, fmt.Errorf("two workers cannot have the same name: %q", o.name)
	}

	if o.env == nil {
		o.env = make(PreparedEnv, 1)
	}

	o.env["FRANKENPHP_WORKER\x00"] = "1"
	w := &worker{
		name:                   o.name,
		fileName:               absFileName,
		num:                    o.num,
		maxThreads:             o.maxThreads,
		env:                    o.env,
		requestChan:            make(chan contextHolder),
		semaphore:              semaphore.NewWeighted(int64(o.num)),
		threads:                make([]*phpThread, 0, o.num),
		allowPathMatching:      allowPathMatching,
		maxConsecutiveFailures: o.maxConsecutiveFailures,
		onThreadReady:          o.onThreadReady,
		onThreadShutdown:       o.onThreadShutdown,
	}

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
	ready := sync.WaitGroup{}
	drainedThreads := make([]*phpThread, 0)
	for _, worker := range workers {
		worker.threadMutex.RLock()
		ready.Add(len(worker.threads))
		for _, thread := range worker.threads {
			if !thread.state.requestSafeStateChange(stateRestarting) {
				ready.Done()
				// no state change allowed == thread is shutting down
				// we'll proceed to restart all other threads anyways
				continue
			}
			close(thread.drainChan)
			drainedThreads = append(drainedThreads, thread)
			go func(thread *phpThread) {
				thread.state.waitFor(stateYielding)
				ready.Done()
			}(thread)
		}
		worker.threadMutex.RUnlock()
	}
	ready.Wait()

	return drainedThreads
}

func drainWatcher() {
	if watcherIsEnabled {
		watcher.DrainWatcher()
	}
}

// RestartWorkers attempts to restart all workers gracefully
func RestartWorkers() {
	// disallow scaling threads while restarting workers
	scalingMu.Lock()
	defer scalingMu.Unlock()

	threadsToRestart := drainWorkerThreads()

	for _, thread := range threadsToRestart {
		thread.drainChan = make(chan struct{})
		thread.state.set(stateReady)
	}
}

func getDirectoriesToWatch(workerOpts []workerOpt) []string {
	directoriesToWatch := []string{}
	for _, w := range workerOpts {
		directoriesToWatch = append(directoriesToWatch, w.watch...)
	}
	return directoriesToWatch
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

	// mark the request as queued and use admission control
	metrics.QueuedWorkerRequest(worker.name)

	workerScaleChan := scaleChan
	if worker.isAtThreadLimit() {
		workerScaleChan = nil
	}

	if err := acquireSemaphoreWithAdmissionControl(ch.ctx, worker.semaphore, workerScaleChan, ch.frankenPHPContext); err != nil {
		metrics.DequeuedWorkerRequest(worker.name)
		ch.frankenPHPContext.reject(err)
		metrics.StopWorkerRequest(worker.name, time.Since(ch.frankenPHPContext.startedAt))
		return err
	}
	defer worker.semaphore.Release(1)

	// Fast path: try to get an idle thread from the pool
	if idle := worker.threadPool.Get(); idle != nil {
		handler := idle.(*workerThread)
		// Non-blocking send: detect stale handlers (threads that transitioned)
		select {
		case handler.workReady <- ch:
			metrics.DequeuedWorkerRequest(worker.name)
			<-ch.frankenPHPContext.done
			metrics.StopWorkerRequest(worker.name, time.Since(ch.frankenPHPContext.startedAt))
			return nil
		default:
		}
	}

	// Slow path: no idle thread in pool, use the global channel
	worker.requestChan <- ch
	metrics.DequeuedWorkerRequest(worker.name)
	<-ch.frankenPHPContext.done
	metrics.StopWorkerRequest(worker.name, time.Since(ch.frankenPHPContext.startedAt))

	return nil
}
