package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
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
	declaredFileName       string
	matchRequest           func(*http.Request) bool
	num                    int
	maxThreads             int
	requestOptions         []RequestOption
	requestChan            chan *frankenPHPContext
	threads                []*phpThread
	threadMutex            sync.RWMutex
	maxConsecutiveFailures int
	bootTimeout            time.Duration
	// Init() gave up waiting for the ready point: the run is drained and must not start again
	bootTimedOut       atomic.Bool
	onThreadReady      func(int)
	onThreadShutdown   func(int)
	queuedRequests     atomic.Int32
	server             *Server
	isBackgroundWorker bool
}

var (
	workers             []*worker
	workersByName       map[string]*worker
	globalWorkersByPath map[string]*worker
	watcherIsEnabled    bool
	startupFailChan     chan error
	// the only window where a boot failure reaches startupFailChan: past it, the handlers log and keep restarting
	startupPhase atomic.Bool
)

// DefaultWorkerBootTimeout is how long a background worker may take to reach
// its first WorkerHandle::tick() before Init() fails, on PHP builds without
// Zend max execution timers: FrankenPHP disables max_execution_time there,
// and this matches its default. Elsewhere max_execution_time bounds the
// bootstrap instead. WithWorkerBootTimeout() sets it on any build.
const DefaultWorkerBootTimeout = 30 * time.Second

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
		if w.server != nil {
			if err := w.server.addWorker(w); err != nil {
				return err
			}
		} else if !w.isBackgroundWorker {
			// background workers are never matched against requests
			globalWorkersByPath[w.fileName] = w
			globalWorkersByPath[w.declaredFileName] = w
		}
	}

	startupFailChan = make(chan error, totalThreadsToStart)
	startupPhase.Store(true)

	for _, w := range workers {
		for range w.num {
			thread := getInactivePHPThread()
			if w.isBackgroundWorker {
				convertToBackgroundWorkerThread(thread, w)
			} else {
				convertToWorkerThread(thread, w)
			}

			bootTimeout := time.Duration(0)
			if w.isBackgroundWorker {
				bootTimeout = w.bootTimeout
			}
			workersReady.Go(func() {
				if bootTimeout <= 0 {
					thread.state.WaitFor(state.Ready, state.ShuttingDown, state.Done)

					return
				}

				if thread.state.WaitForStateWithTimeout(bootTimeout, state.Ready, state.ShuttingDown, state.Done) {
					return
				}

				// the script never ticked: stop the thread, so no run starts
				// after this one, which ends at once if it waits on its handle
				// and through the force-kill of the shutdown otherwise
				if !thread.state.CompareAndSwap(state.TransitionComplete, state.ShuttingDown) {
					return
				}
				w.bootTimedOut.Store(true)
				reportStartupFailure(fmt.Errorf("background worker %q did not call WorkerHandle::tick() within %s", w.name, bootTimeout))
				if handler, ok := thread.handler.(*backgroundWorkerThread); ok {
					handler.drain()
				}
			})
		}
	}

	workersReady.Wait()
	startupPhase.Store(false)

	select {
	case err := <-startupFailChan:
		// at least 1 worker has failed, return an error
		return fmt.Errorf("failed to initialize workers: %w", err)
	default:
		// all workers started successfully
	}

	return nil
}

// reportStartupFailure hands a boot failure to initWorkers() while it waits,
// so Init() fails, and reports whether it did: past the startup phase the
// handler logs it and keeps restarting. It never blocks.
func reportStartupFailure(err error) bool {
	if !startupPhase.Load() {
		return false
	}

	select {
	case startupFailChan <- err:
	default:
	}

	return true
}

// resolveWorkerFile turns a declared filename into the path a worker runs,
// and names the worker after it when the declaration left the name out.
//
// Order is important!
// This order ensures that FrankenPHP started from inside a symlinked directory will properly resolve any paths.
// If it is started from outside a symlinked directory, it is resolved to the same path that we use in the Caddy module.
func resolveWorkerFile(o workerOpt) (workerOpt, error) {
	declaredFileName, err := fastabs.FastAbs(filepath.FromSlash(o.fileName))
	if err != nil {
		return o, fmt.Errorf("worker filename is invalid %q: %w", o.fileName, err)
	}

	absFileName, err := filepath.EvalSymlinks(filepath.FromSlash(o.fileName))
	if err != nil {
		return o, fmt.Errorf("worker filename is invalid %q: %w", o.fileName, err)
	}

	absFileName, err = fastabs.FastAbs(absFileName)
	if err != nil {
		return o, fmt.Errorf("worker filename is invalid %q: %w", o.fileName, err)
	}

	if _, err := os.Stat(absFileName); err != nil {
		return o, fmt.Errorf("worker file not found %q: %w", absFileName, err)
	}

	o.declaredFileName = declaredFileName
	o.fileName = absFileName
	if o.name == "" {
		o.name = absFileName
	}

	return o, nil
}

// checkWorkerDeclaration holds the rules a set of workers must follow, so
// Validate() and newWorker() answer the same way; pathTaken is asked for the
// global workers with a nil server.
func checkWorkerDeclaration(o workerOpt, nameTaken func(string) bool, pathTaken func(*Server, string) bool) error {
	if o.isBackgroundWorker {
		// never matched against requests, so several may share a script
		if o.matchRequest != nil {
			return fmt.Errorf("background worker %q cannot match requests", o.name)
		}
		if o.maxThreads > 0 {
			return fmt.Errorf("background worker %q cannot set max_threads, it does not autoscale", o.name)
		}
		// Workers.SendRequest() and SendMessage() dispatch on requestChan,
		// which no background thread reads
		if o.extensionWorkers != nil {
			return fmt.Errorf("background worker %q cannot be an extension worker, those handle requests", o.name)
		}
	} else if o.server == nil {
		if pathTaken(nil, o.fileName) {
			return fmt.Errorf("two global workers cannot have the same filename: %q", o.fileName)
		}

		// no server means no set of requests to match against, the matcher would never run
		if o.matchRequest != nil {
			return fmt.Errorf("worker %q has a request matcher but no server scope, use WithWorkerServerScope()", o.name)
		}
	} else if o.matchRequest == nil && pathTaken(o.server, o.fileName) {
		// a matcher tells the workers of a server sharing a file apart
		return fmt.Errorf("two workers in a server cannot have the same filename: %q", o.fileName)
	}

	if nameTaken(o.name) {
		return fmt.Errorf("two workers cannot have the same name: %q", o.name)
	}

	return nil
}

func newWorker(o workerOpt) (*worker, error) {
	o, err := resolveWorkerFile(o)
	if err != nil {
		return nil, err
	}

	nameTaken := func(name string) bool { return workersByName[name] != nil }
	pathTaken := func(server *Server, path string) bool {
		if server == nil {
			return globalWorkersByPath[path] != nil
		}

		return server.workersByPath[path] != nil
	}

	if err := checkWorkerDeclaration(o, nameTaken, pathTaken); err != nil {
		return nil, err
	}

	absFileName := o.fileName

	// env should always contain FRANKENPHP_WORKER and the parent php_server env
	if o.env == nil {
		o.env = make(PreparedEnv, 1)
	}

	// if the worker is scoped to a server, inherit the server env
	if o.server != nil && len(o.server.env) > 0 {
		for k, v := range o.server.env {
			if _, exists := o.env[k]; !exists {
				o.env[k] = v
			}
		}
	}

	// a script serving both roles tests which of the two is set
	if o.isBackgroundWorker {
		o.env["FRANKENPHP_WORKER_BACKGROUND\x00"] = o.name
	} else {
		o.env["FRANKENPHP_WORKER\x00"] = "1"
	}

	// a background worker that never reaches its ready point would keep Init()
	// waiting for ever where max_execution_time is disabled, see initWorkers()
	bootTimeout := time.Duration(0)
	if !Config().ZendMaxExecutionTimers {
		bootTimeout = DefaultWorkerBootTimeout
	}
	if o.bootTimeoutIsSet {
		bootTimeout = o.bootTimeout
	}

	w := &worker{
		name:                   o.name,
		fileName:               absFileName,
		declaredFileName:       o.declaredFileName,
		matchRequest:           o.matchRequest,
		requestOptions:         o.requestOptions,
		num:                    o.num,
		maxThreads:             o.maxThreads,
		requestChan:            make(chan *frankenPHPContext),
		threads:                make([]*phpThread, 0, o.num),
		maxConsecutiveFailures: o.maxConsecutiveFailures,
		bootTimeout:            bootTimeout,
		onThreadReady:          o.onThreadReady,
		onThreadShutdown:       o.onThreadShutdown,
		server:                 o.server,
		isBackgroundWorker:     o.isBackgroundWorker,
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
		case <-fc.ctx.Done():
			worker.queuedRequests.Add(-1)
			metrics.DequeuedWorkerRequest(worker.name)
			metrics.StopWorkerRequest(worker.name, time.Since(fc.startedAt))

			return nil
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
