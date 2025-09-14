package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dunglas/frankenphp/internal/fastabs"
	"github.com/dunglas/frankenphp/internal/watcher"
)

// represents a worker script and can have many threads assigned to it
type worker struct {
	name                   string
	fileName               string
	num                    int
	env                    PreparedEnv
	threadPool             *threadPool
	allowPathMatching      bool
	maxConsecutiveFailures int
}

var (
	workers          []*worker
	watcherIsEnabled bool
)

func initWorkers(opt []workerOpt) error {
	workers = make([]*worker, 0, len(opt))
	workersReady := sync.WaitGroup{}
	directoriesToWatch := getDirectoriesToWatch(opt)
	watcherIsEnabled = len(directoriesToWatch) > 0

	for _, o := range opt {
		w, err := newWorker(o)
		if err != nil {
			return err
		}
		workers = append(workers, w)
	}

	for _, worker := range workers {
		workersReady.Add(worker.num)
		for i := 0; i < worker.num; i++ {
			thread := getInactivePHPThread()
			convertToWorkerThread(thread, worker)
			go func() {
				thread.state.waitFor(stateReady)
				workersReady.Done()
			}()
		}
	}

	workersReady.Wait()

	if !watcherIsEnabled {
		return nil
	}

	watcherIsEnabled = true
	if err := watcher.InitWatcher(directoriesToWatch, RestartWorkers, logger); err != nil {
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
		env:                    o.env,
		threadPool:             newThreadPool(o.num),
		allowPathMatching:      allowPathMatching,
		maxConsecutiveFailures: o.maxConsecutiveFailures,
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
		worker.threadPool.mu.RLock()
		ready.Add(len(worker.threadPool.threads))
		for _, thread := range worker.threadPool.threads {
			if !thread.state.requestSafeStateChange(stateRestarting) {
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
		worker.threadPool.mu.RUnlock()
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

func (worker *worker) handleRequest(fc *frankenPHPContext) {
	metrics.StartWorkerRequest(worker.name)

	trackLatency := latencyTrackingEnabled.Load()
	isSlowRequest := trackLatency && isHighLatencyRequest(fc)

	// dispatch requests to all worker threads in order
	if !isSlowRequest && worker.threadPool.dispatchRequest(fc) {
		<-fc.done
		requestTime := time.Since(fc.startedAt)
		metrics.StopWorkerRequest(worker.name, requestTime)
		trackRequestLatency(fc, requestTime, false)

		return
	}

	metrics.QueuedWorkerRequest(worker.name)
	requestWasReceived := worker.threadPool.queueRequest(fc, trackLatency && !isSlowRequest)
	metrics.DequeuedWorkerRequest(worker.name)

	if !requestWasReceived {
		return
	}

	stallDuration := time.Since(fc.startedAt)
	<-fc.done
	requestTime := time.Since(fc.startedAt)
	metrics.StopWorkerRequest(worker.name, requestTime)
	trackRequestLatency(fc, requestTime-stallDuration, isSlowRequest)
}
