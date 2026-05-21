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
	scope                  Scope
	// bg, if non-nil, marks this as a background (non-HTTP) worker and
	// holds its bg-specific lifecycle state.
	bg *backgroundWorkerExtras
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

	declared := make([]*worker, len(opt))
	for i, o := range opt {
		w, err := newWorker(o)
		if err != nil {
			return err
		}

		totalThreadsToStart += w.num
		declared[i] = w
		workers = append(workers, w)
		// Background workers are resolved per-scope via backgroundLookups
		// so the same user-facing name can appear in multiple scopes
		// without colliding in the global workersByName map.
		if w.bg == nil {
			workersByName[w.name] = w
		}
		if w.allowPathMatching {
			workersByPath[w.fileName] = w
		}
	}

	// Build the per-scope lookups (named + catch-all per scope). Each
	// php_server block gets its own scope; the global/embed scope is 0.
	// Each declaration registers in its scope's lookup so lazy-start
	// siblings inherit the template options.
	var err error
	backgroundLookups, err = buildBackgroundWorkerLookups(declared, opt)
	if err != nil {
		return err
	}

	startupFailChan = make(chan error, totalThreadsToStart)

	for _, w := range workers {
		for i := 0; i < w.num; i++ {
			thread := getInactivePHPThread()
			if w.bg != nil {
				convertToBackgroundWorkerThread(thread, w, w.name, w.bg.ready)
			} else {
				convertToWorkerThread(thread, w)
			}

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

	// workers that have a name starting with "m#" are module workers
	// they can only be matched by their name, not by their path.
	// Background workers are matched only by name, never by path, since
	// they don't handle HTTP requests.
	allowPathMatching := !strings.HasPrefix(o.name, "m#") && !o.isBackgroundWorker

	// Background workers are matched only by name, never by path. Multiple
	// named bg workers can share an entrypoint file; uniqueness is enforced
	// per-scope via backgroundLookups, not at the global path level.
	if !o.isBackgroundWorker {
		if w := workersByPath[absFileName]; w != nil && allowPathMatching {
			return w, fmt.Errorf("two workers cannot have the same filename: %q", absFileName)
		}
		// Background workers are resolved through per-scope lookups, not the
		// global workersByName map; the same user-facing name can appear in
		// multiple php_server scopes without collision.
		if w := workersByName[o.name]; w != nil {
			return w, fmt.Errorf("two workers cannot have the same name: %q", o.name)
		}
	}

	if o.env == nil {
		o.env = make(PreparedEnv)
	}

	// $_SERVER['FRANKENPHP_WORKER'] is populated downstream from
	// fc.worker.name via frankenphp_register_server_vars.
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
		scope:                  o.scope,
	}
	if o.isBackgroundWorker {
		w.bg = &backgroundWorkerExtras{ready: newBackgroundWorkerState()}
	}

	// backgroundWorker state is reserved lazily via the registry at
	// thread-setup time, not here; lazy-start callers set it directly
	// and eager inits go through setupScript's sync.Once.

	w.configureMercure(&o)

	// o.env is shared by reference across instances; treat it as
	// read-only after init. Deep-clone if per-instance env is ever
	// needed.
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

// drainGracePeriod: time to wait for threads to yield before arming force-kill.
var drainGracePeriod = 30 * time.Second

// EXPERIMENTAL: DrainWorkers initiates a graceful drain of all worker scripts.
// Blocks until every drained thread yields. Force-kill is armed after a
// grace period to wake threads parked in blocking syscalls (sleep, I/O).
func DrainWorkers() {
	// Defensive RLock paired with RestartWorkers' scalingMu.Lock to
	// guard against future runtime mutators of the workers slice.
	scalingMu.RLock()
	defer scalingMu.RUnlock()
	_ = drainWorkerThreads()
}

// drainWorkerThreads walks the global workers slice. The caller must
// ensure no concurrent mutation of the slice (either hold scalingMu or
// be called from a context where scaling is paused).
func drainWorkerThreads() (drainedThreads []*phpThread) {
	var ready sync.WaitGroup

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

			thread.handler.drain()
			close(thread.drainChan)
			drainedThreads = append(drainedThreads, thread)

			go func(thread *phpThread) {
				thread.state.WaitFor(state.Yielding, state.ShuttingDown, state.Done)
				ready.Done()
			}(thread)
		}

		worker.threadMutex.RUnlock()
	}

	done := make(chan struct{})
	go func() {
		ready.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(drainGracePeriod):
		// Force-kill any thread still stuck in a blocking syscall, then
		// keep waiting unconditionally. On platforms where force-kill
		// cannot interrupt the syscall (macOS, Windows non-alertable
		// Sleep) the thread exits when the syscall completes naturally.
		for _, thread := range drainedThreads {
			if !thread.state.Is(state.Yielding) {
				thread.forceKillMu.RLock()
				C.frankenphp_force_kill_thread(thread.forceKill)
				thread.forceKillMu.RUnlock()
			}
		}
		<-done
	}

	return drainedThreads
}

// RestartWorkers attempts to restart all workers gracefully.
// All workers must be restarted at the same time to prevent issues with
// opcache resetting. Blocks until every worker thread has yielded;
// force-kill is armed after a grace period to wake threads parked in
// blocking syscalls so a stuck sleep doesn't make this hang for the
// full duration of the syscall.
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

// invalidateBackgroundEntry releases the registry slot held by a bg
// worker exiting on max_consecutive_failures so a future ensure() can
// lazy-spawn a fresh thread. Single-instance only (catch-all instances,
// lazy named).
func (worker *worker) invalidateBackgroundEntry(name string) {
	bg := worker.bg
	if bg.catchAllNames != nil {
		bg.catchAllMu.Lock()
		delete(bg.catchAllNames, name)
		bg.catchAllMu.Unlock()
		return
	}
	if worker.num == 0 {
		// Fresh slot so the retry isn't poisoned by the prior abort.
		bg.lazyMu.Lock()
		bg.ready = newBackgroundWorkerState()
		bg.lazyStarted = false
		bg.lazyMu.Unlock()
	}
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
