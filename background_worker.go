package frankenphp

// #include <stdint.h>
// #include "frankenphp.h"
import "C"
import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

var backgroundScopeCounter atomic.Uint64

// NextBackgroundWorkerScope returns a unique scope ID for background worker isolation.
// Each php_server block should call this once during provisioning.
func NextBackgroundWorkerScope() string {
	return fmt.Sprintf("php_server_%d", backgroundScopeCounter.Add(1))
}

// defaultMaxBackgroundWorkers is the default safety cap for catch-all background workers.
const defaultMaxBackgroundWorkers = 16

// backgroundLookups maps scope IDs to their background worker lookups.
// Each php_server block gets its own scope. The global frankenphp block
// uses the empty string as its scope ID.
var backgroundLookups map[string]*backgroundWorkerLookup

// backgroundWorkerLookup maps worker names to registries, enabling multiple entrypoint files.
type backgroundWorkerLookup struct {
	byName   map[string]*backgroundWorkerRegistry
	catchAll *backgroundWorkerRegistry
}

func newBackgroundWorkerLookup() *backgroundWorkerLookup {
	return &backgroundWorkerLookup{
		byName: make(map[string]*backgroundWorkerRegistry),
	}
}

func (l *backgroundWorkerLookup) AddNamed(name string, registry *backgroundWorkerRegistry) {
	l.byName[name] = registry
}

func (l *backgroundWorkerLookup) SetCatchAll(registry *backgroundWorkerRegistry) {
	l.catchAll = registry
}

// Resolve returns the registry for the given name, falling back to catch-all.
func (l *backgroundWorkerLookup) Resolve(name string) *backgroundWorkerRegistry {
	if r, ok := l.byName[name]; ok {
		return r
	}
	return l.catchAll
}

type backgroundWorkerRegistry struct {
	entrypoint     string
	num            int      // threads per background worker (0 = lazy-start with 1 thread)
	maxWorkers     int      // max lazy-started instances (0 = unlimited)
	autoStartNames []string // names to start at boot when num >= 1
	mu             sync.Mutex
	workers        map[string]*backgroundWorkerState
}

func newBackgroundWorkerRegistry(entrypoint string) *backgroundWorkerRegistry {
	return &backgroundWorkerRegistry{
		entrypoint: entrypoint,
		workers:    make(map[string]*backgroundWorkerState),
	}
}

func (registry *backgroundWorkerRegistry) MaxThreads() int {
	if registry.num > 0 {
		return registry.num
	}
	return 1
}

func (registry *backgroundWorkerRegistry) SetNum(num int) {
	registry.num = num
}

func (registry *backgroundWorkerRegistry) AddAutoStartNames(names ...string) {
	registry.autoStartNames = append(registry.autoStartNames, names...)
}

func (registry *backgroundWorkerRegistry) SetMaxWorkers(max int) {
	registry.maxWorkers = max
}

// buildBackgroundWorkerLookups constructs per-scope background worker lookups
// from worker options. Each scope (php_server block) gets its own lookup.
func buildBackgroundWorkerLookups(workers []*worker, opts []workerOpt) map[string]*backgroundWorkerLookup {
	lookups := make(map[string]*backgroundWorkerLookup)
	scopeRegistries := make(map[string]map[string]*backgroundWorkerRegistry)

	for i, o := range opts {
		if !o.isBackgroundWorker {
			continue
		}

		scope := o.backgroundScope
		lookup, ok := lookups[scope]
		if !ok {
			lookup = newBackgroundWorkerLookup()
			lookups[scope] = lookup
			scopeRegistries[scope] = make(map[string]*backgroundWorkerRegistry)
		}

		entrypoint := o.fileName
		registry, ok := scopeRegistries[scope][entrypoint]
		if !ok {
			registry = newBackgroundWorkerRegistry(entrypoint)
			scopeRegistries[scope][entrypoint] = registry
		}

		workers[i].backgroundScope = scope

		w := workers[i]
		phpName := strings.TrimPrefix(w.name, "m#")
		if phpName != "" && phpName != w.fileName {
			// Named background worker
			if o.num > 0 {
				registry.AddAutoStartNames(phpName)
				registry.SetNum(o.num)
			}
			lookup.AddNamed(phpName, registry)
		} else {
			// Catch-all background worker; maxThreads > 1 means user set it explicitly
			maxW := defaultMaxBackgroundWorkers
			if o.maxThreads > 1 {
				maxW = o.maxThreads
			}
			registry.SetMaxWorkers(maxW)
			lookup.SetCatchAll(registry)
		}

		w.backgroundRegistry = registry
	}

	if len(lookups) == 0 {
		return nil
	}

	return lookups
}

func (registry *backgroundWorkerRegistry) reserve(name string) (*backgroundWorkerState, bool, error) {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	if bgw := registry.workers[name]; bgw != nil {
		return bgw, true, nil
	}

	if registry.maxWorkers > 0 && len(registry.workers) >= registry.maxWorkers {
		return nil, false, fmt.Errorf("cannot start background worker %q: limit of %d reached - increase max_threads on the catch-all background worker or declare it as a named worker", name, registry.maxWorkers)
	}

	bgw := &backgroundWorkerState{
		ready: make(chan struct{}),
		tasks: make(chan *taskRequest, 1), // buffer=1: backpressure with signaling
		dead:  make(chan struct{}),
	}
	registry.workers[name] = bgw

	return bgw, false, nil
}

func (registry *backgroundWorkerRegistry) remove(name string, bgw *backgroundWorkerState) {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	if registry.workers[name] == bgw {
		delete(registry.workers, name)
	}

	// Signal waiting senders that this worker is gone
	bgw.deadOnce.Do(func() { close(bgw.dead) })
}

func startBackgroundWorker(thread *phpThread, bgWorkerName string) error {
	if bgWorkerName == "" {
		return fmt.Errorf("background worker name must not be empty")
	}

	lookup := getLookup(thread)
	if lookup == nil {
		return fmt.Errorf("no background worker configured in this php_server")
	}

	registry := lookup.Resolve(bgWorkerName)
	if registry == nil || registry.entrypoint == "" {
		return fmt.Errorf("no background worker configured in this php_server")
	}

	return startBackgroundWorkerWithRegistry(registry, bgWorkerName)
}

func startBackgroundWorkerWithRegistry(registry *backgroundWorkerRegistry, bgWorkerName string) error {
	bgw, exists, err := registry.reserve(bgWorkerName)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	numThreads := registry.MaxThreads()

	worker, err := newWorker(workerOpt{
		name:                   bgWorkerName,
		fileName:               registry.entrypoint,
		num:                    numThreads,
		isBackgroundWorker:     true,
		env:                    PrepareEnv(nil),
		watch:                  []string{},
		maxConsecutiveFailures: -1,
	})
	if err != nil {
		registry.remove(bgWorkerName, bgw)

		return fmt.Errorf("failed to create background worker: %w", err)
	}

	worker.isBackgroundWorker = true
	worker.backgroundWorker = bgw
	worker.backgroundRegistry = registry
	bgw.fds = &worker.backgroundFds

	for i := 0; i < numThreads; i++ {
		bgWorkerThread := getInactivePHPThread()
		if bgWorkerThread == nil {
			if i == 0 {
				registry.remove(bgWorkerName, bgw)
			}

			return fmt.Errorf("no available PHP thread for background worker (increase max_threads)")
		}

		scalingMu.Lock()
		workers = append(workers, worker)
		scalingMu.Unlock()

		convertToBackgroundWorkerThread(bgWorkerThread, worker)
	}

	if globalLogger.Enabled(globalCtx, slog.LevelInfo) {
		globalLogger.LogAttrs(globalCtx, slog.LevelInfo, "background worker started", slog.String("name", bgWorkerName), slog.Int("threads", numThreads))
	}

	return nil
}

func getLookup(thread *phpThread) *backgroundWorkerLookup {
	if handler, ok := thread.handler.(*workerThread); ok && handler.worker.backgroundLookup != nil {
		return handler.worker.backgroundLookup
	}
	if handler, ok := thread.handler.(*backgroundWorkerThread); ok && handler.worker.backgroundLookup != nil {
		return handler.worker.backgroundLookup
	}
	// Non-worker requests: resolve scope from context
	if fc, ok := fromContext(thread.context()); ok && fc.backgroundScope != "" {
		if backgroundLookups != nil {
			return backgroundLookups[fc.backgroundScope]
		}
	}
	// Fall back to global scope
	if backgroundLookups != nil {
		return backgroundLookups[""]
	}

	return nil
}

// go_frankenphp_worker_get_vars starts background workers if needed, waits for them
// to be ready, takes read locks, copies vars via C helper, and releases locks.
// All locking/unlocking happens within this single Go call.
//
// callerVersions/outVersions: if callerVersions is non-nil and all versions match,
// the copy is skipped entirely (returns 1). outVersions receives current versions.
//
//export go_frankenphp_worker_get_vars
func go_frankenphp_worker_get_vars(threadIndex C.uintptr_t, names **C.char, nameLens *C.size_t, nameCount C.int, timeoutMs C.int, returnValue *C.zval, callerVersions *C.uint64_t, outVersions *C.uint64_t) *C.char {
	thread := phpThreads[threadIndex]
	lookup := getLookup(thread)
	if lookup == nil {
		return C.CString("no background worker configured in this php_server")
	}

	n := int(nameCount)
	nameSlice := unsafe.Slice(names, n)
	nameLenSlice := unsafe.Slice(nameLens, n)

	sks := make([]*backgroundWorkerState, n)
	goNames := make([]string, n)
	for i := 0; i < n; i++ {
		goNames[i] = C.GoStringN(nameSlice[i], C.int(nameLenSlice[i]))

		// Start background worker if not already running
		if err := startBackgroundWorker(thread, goNames[i]); err != nil {
			return C.CString(err.Error())
		}

		registry := lookup.Resolve(goNames[i])
		if registry == nil {
			return C.CString("background worker not found: " + goNames[i])
		}
		registry.mu.Lock()
		sks[i] = registry.workers[goNames[i]]
		registry.mu.Unlock()
		if sks[i] == nil {
			return C.CString("background worker not found: " + goNames[i])
		}
	}

	// Wait for all workers to be ready (shared deadline across all workers)
	deadline := time.After(time.Duration(timeoutMs) * time.Millisecond)
	for i, sk := range sks {
		select {
		case <-sk.ready:
			// background worker has called set_vars
		case <-deadline:
			return C.CString(fmt.Sprintf("timeout waiting for background worker: %s", goNames[i]))
		}
	}

	// Fast path: if all caller versions match, skip lock + copy entirely.
	// Read each version once and write to outVersions for the C side to compare.
	if callerVersions != nil && outVersions != nil {
		callerVSlice := unsafe.Slice(callerVersions, n)
		outVSlice := unsafe.Slice(outVersions, n)
		allMatch := true
		for i, sk := range sks {
			v := sk.varsVersion.Load()
			outVSlice[i] = C.uint64_t(v)
			if uint64(callerVSlice[i]) != v {
				allMatch = false
			}
		}
		if allMatch {
			return nil // C side sees out == caller, uses cached value
		}
	}

	// Take all read locks, collect pointers, copy via C helper, then release
	ptrs := make([]unsafe.Pointer, n)
	for i, sk := range sks {
		sk.mu.RLock()
		ptrs[i] = sk.varsPtr
	}

	C.frankenphp_worker_copy_vars(returnValue, C.int(n), names, nameLens, (*unsafe.Pointer)(unsafe.Pointer(&ptrs[0])))

	// Write versions while locks are still held
	if outVersions != nil {
		outVSlice := unsafe.Slice(outVersions, n)
		for i, sk := range sks {
			outVSlice[i] = C.uint64_t(sk.varsVersion.Load())
		}
	}

	for _, sk := range sks {
		sk.mu.RUnlock()
	}

	return nil
}

//export go_frankenphp_worker_set_vars
func go_frankenphp_worker_set_vars(threadIndex C.uintptr_t, varsPtr unsafe.Pointer, oldPtr *unsafe.Pointer) *C.char {
	thread := phpThreads[threadIndex]

	bgHandler, ok := thread.handler.(*backgroundWorkerThread)
	if !ok || bgHandler.worker.backgroundWorker == nil {
		return C.CString("frankenphp_worker_set_vars() can only be called from a background worker")
	}

	sk := bgHandler.worker.backgroundWorker

	sk.mu.Lock()
	*oldPtr = sk.varsPtr
	sk.varsPtr = varsPtr
	sk.varsVersion.Add(1)
	sk.mu.Unlock()

	sk.readyOnce.Do(func() {
		bgHandler.markBackgroundReady()
		close(sk.ready)
	})

	return nil
}
