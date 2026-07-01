package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"fmt"
	"net/http"
	"os"
	"path"
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
	matchRequest           func(*http.Request) bool
	matchRelPath           string
	num                    int
	maxThreads             int
	requestOptions         []RequestOption
	requestChan            chan *frankenPHPContext
	threads                []*phpThread
	threadMutex            sync.RWMutex
	maxConsecutiveFailures int
	onThreadReady          func(int)
	onThreadShutdown       func(int)
	queuedRequests         atomic.Int32
	phpServer              *PhpServer
}

var (
	workers             []*worker
	workersByName       map[string]*worker
	globalWorkersByPath map[string]*worker
	watcherIsEnabled    bool
	startupFailChan     chan error
)

func initWorkers(opts []workerOpt) error {
	if len(opts) == 0 {
		return nil
	}

	var (
		workersReady        sync.WaitGroup
		totalThreadsToStart int
	)

	workers = make([]*worker, 0, len(opts))
	workersByName = make(map[string]*worker, len(opts))
	globalWorkersByPath = make(map[string]*worker, len(opts))

	for _, o := range opts {
		w, err := newWorker(o)
		if err != nil {
			return err
		}

		totalThreadsToStart += w.num
		workers = append(workers, w)
		workersByName[w.name] = w
		if w.phpServer == nil {
			globalWorkersByPath[w.fileName] = w
		} else {
			w.phpServer.workers = append(w.phpServer.workers, w)
			w.phpServer.workersByPath[w.fileName] = w
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
	absFileName, err := filepath.EvalSymlinks(filepath.FromSlash(o.fileName))
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

	if o.phpServer == nil {
		if w := globalWorkersByPath[absFileName]; w != nil {
			return w, fmt.Errorf("two workers cannot have the same filename: %q", absFileName)
		}
	}
	if w := workersByName[o.name]; w != nil {
		return w, fmt.Errorf("two workers cannot have the same name: %q", o.name)
	}

	// env should always contain FRANKENPHP_WORKER and the parent php_server env
	if o.env == nil {
		o.env = make(PreparedEnv, 1)
	}

	o.env["FRANKENPHP_WORKER\x00"] = "1"

	if o.phpServer != nil && len(o.phpServer.env) > 0 {
		for k, v := range o.phpServer.env {
			if _, exists := o.env[k]; !exists {
				o.env[k] = v
			}
		}
	}

	w := &worker{
		name:                   o.name,
		fileName:               absFileName,
		matchRequest:           o.matchRequest,
		requestOptions:         o.requestOptions,
		num:                    o.num,
		maxThreads:             o.maxThreads,
		requestChan:            make(chan *frankenPHPContext),
		threads:                make([]*phpThread, 0, o.num),
		maxConsecutiveFailures: o.maxConsecutiveFailures,
		onThreadReady:          o.onThreadReady,
		onThreadShutdown:       o.onThreadShutdown,
		phpServer:              o.phpServer,
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

	if o.matchRequest == nil && o.phpServer != nil && o.phpServer.root != "" {
		docRootWithSep := o.phpServer.root + string(filepath.Separator)
		if strings.HasPrefix(absFileName, docRootWithSep) {
			w.matchRelPath = filepath.ToSlash(absFileName[len(o.phpServer.root):])
		}
	}

	return w, nil
}

func (w *worker) matchesRequest(r *http.Request, documentRoot string) bool {
	if w.matchRequest != nil {
		return w.matchRequest(r)
	}

	if w.matchRelPath != "" {
		reqPath := r.URL.Path
		if reqPath == w.matchRelPath {
			return true
		}
		if reqPath == "" || reqPath[0] != '/' {
			reqPath = "/" + reqPath
		}
		return path.Clean(reqPath) == w.matchRelPath
	}

	fullPath, _ := fastabs.FastAbs(filepath.Join(documentRoot, r.URL.Path))
	return fullPath == w.fileName
}

// EXPERIMENTAL: DrainWorkers initiates a graceful drain of all php threads.
// Blocks until every drained thread yields. Force-kill is armed after a
// grace period to wake threads parked in blocking syscalls (sleep, I/O).
func DrainWorkers() {
	drainPHPThreads()
}

// RestartWorkers attempts to restart all workers gracefully.
// All workers must be restarted at the same time to prevent issues with
// opcache resetting. Blocks until every worker thread has yielded;
// force-kill is armed after a grace period to wake threads parked in
// blocking syscalls so a stuck sleep doesn't make this hang for the
// full duration of the syscall.
func RestartWorkers() {
	if mainThread != nil {
		mainThread.rebootAllThreads()
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

func (worker *worker) handleRequest(fc *frankenPHPContext) error {
	metrics.StartWorkerRequest(worker.name)

	runtime.Gosched()

	if worker.queuedRequests.Load() == 0 {
		// dispatch requests to all worker threads in order
		worker.threadMutex.RLock()
		for _, thread := range worker.threads {
			select {
			case thread.requestChan <- fc:
				worker.threadMutex.RUnlock()
				<-fc.done
				metrics.StopWorkerRequest(worker.name, time.Since(fc.startedAt))

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
		case worker.requestChan <- fc:
			worker.queuedRequests.Add(-1)
			metrics.DequeuedWorkerRequest(worker.name)
			<-fc.done
			metrics.StopWorkerRequest(worker.name, time.Since(fc.startedAt))

			return nil
		case workerScaleChan <- fc:
			// the request has triggered scaling, continue to wait for a thread
		case <-timeoutChan(time.Duration(maxWaitTime.Load())):
			// the request has timed out stalling
			worker.queuedRequests.Add(-1)
			metrics.DequeuedWorkerRequest(worker.name)
			metrics.StopWorkerRequest(worker.name, time.Since(fc.startedAt))

			fc.reject(ErrMaxWaitTimeExceeded)

			return ErrMaxWaitTimeExceeded
		}
	}
}
