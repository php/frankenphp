package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dunglas/frankenphp/internal/fastabs"
	"github.com/dunglas/frankenphp/internal/state"
	"github.com/dunglas/frankenphp/internal/watcher"
)

// represents a worker script and can have many threads assigned to it
type worker struct {
	name                   string
	fileName               string
	num                    int
	env                    PreparedEnv
	requestChan            chan *frankenPHPContext
	threads                []*phpThread
	threadMutex            sync.RWMutex
	allowPathMatching      bool
	maxConsecutiveFailures int
	onThreadReady          func(int)
	onThreadShutdown       func(int)
}

var (
	workers          []*worker
	watcherIsEnabled bool
	startupFailChan  chan (error)
)

func initWorkers(opt []workerOpt) error {
	workers = make([]*worker, 0, len(opt))
	directoriesToWatch := getDirectoriesToWatch(opt)
	watcherIsEnabled = len(directoriesToWatch) > 0
	totalThreadsToStart := 0

	for _, o := range opt {
		w, err := newWorker(o)
		if err != nil {
			return err
		}
		totalThreadsToStart += w.num
		workers = append(workers, w)
	}

	startupFailChan = make(chan error, totalThreadsToStart)
	var workersReady sync.WaitGroup
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
		// at least 1 worker has failed, shut down and return an error
		Shutdown()
		return fmt.Errorf("failed to initialize workers: %w", err)
	default:
		// all workers started successfully
		startupFailChan = nil
	}

	if !watcherIsEnabled {
		return nil
	}

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
		requestChan:            make(chan *frankenPHPContext),
		threads:                make([]*phpThread, 0, o.num),
		allowPathMatching:      allowPathMatching,
		maxConsecutiveFailures: o.maxConsecutiveFailures,
		onThreadReady:          o.onThreadReady,
		onThreadShutdown:       o.onThreadShutdown,
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
			if !thread.state.RequestSafeStateChange(state.Restarting) {
				ready.Done()
				// no state change allowed == thread is shutting down
				// we'll proceed to restart all other threads anyways
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
		thread.state.Set(state.Ready)
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

func (worker *worker) handleRequest(fc *frankenPHPContext) {
	metrics.StartWorkerRequest(worker.name)

	// dispatch requests to all worker threads in order
	worker.threadMutex.RLock()
	for _, thread := range worker.threads {
		select {
		case thread.requestChan <- fc:
			worker.threadMutex.RUnlock()
			<-fc.done
			metrics.StopWorkerRequest(worker.name, time.Since(fc.startedAt))
			return
		default:
			// thread is busy, continue
		}
	}
	worker.threadMutex.RUnlock()

	// if no thread was available, mark the request as queued and apply the scaling strategy
	metrics.QueuedWorkerRequest(worker.name)
	for {
		select {
		case worker.requestChan <- fc:
			metrics.DequeuedWorkerRequest(worker.name)
			<-fc.done
			metrics.StopWorkerRequest(worker.name, time.Since(fc.startedAt))
			return
		case scaleChan <- fc:
			// the request has triggered scaling, continue to wait for a thread
		case <-timeoutChan(maxWaitTime):
			metrics.DequeuedWorkerRequest(worker.name)
			// the request has timed out stalling
			fc.reject(504, "Gateway Timeout")
			return
		}
	}
}
