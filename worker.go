package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dunglas/frankenphp/internal/fastabs"
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
}

var (
	workers []*worker
)

func initWorkers(opt []workerOpt) error {
	workers = make([]*worker, 0, len(opt))
	workersReady := sync.WaitGroup{}

	for _, o := range opt {
		w, err := newWorker(o)
		if err != nil {
			return err
		}
		workers = append(workers, w)
	}

	for _, w := range workers {
		workersReady.Add(w.num)
		for i := 0; i < w.num; i++ {
			thread := getInactivePHPThread()
			convertToWorkerThread(thread, w)
			go func() {
				thread.state.waitFor(stateReady)

				// create a pipe from the external worker to the main worker
				// note: this is locked to the initial thread size the external worker requested
				if workerThread, ok := thread.handler.(*workerThread); ok && workerThread.externalWorker != nil {
					go startWorker(w, workerThread.externalWorker, thread)
				}
				workersReady.Done()
			}()
		}
	}

	workersReady.Wait()

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
	threadsToDrain := make([]*phpThread, 0)
	for _, worker := range workers {
		worker.threadMutex.RLock()
		threadsToDrain = append(threadsToDrain, worker.threads...)
		worker.threadMutex.RUnlock()
	}

	for _, taskWorker := range taskWorkers {
		taskWorker.threadMutex.RLock()
		threadsToDrain = append(threadsToDrain, taskWorker.threads...)
		taskWorker.threadMutex.RUnlock()
	}

	for _, thread := range threadsToDrain {
		if !thread.state.requestSafeStateChange(stateRestarting) {
			// no state change allowed == thread is shutting down
			// we'll proceed to restart all other threads anyways
			continue
		}
		ready.Add(1)
		close(thread.drainChan)
		drainedThreads = append(drainedThreads, thread)
		go func(thread *phpThread) {
			thread.state.waitFor(stateYielding)
			ready.Done()
		}(thread)
	}
	ready.Wait()

	return drainedThreads
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
