package frankenphp

// #include <stdint.h>
// #include "frankenphp.h"
import "C"
import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

const (
	// defaultMaxBackgroundWorkers is the default safety cap for catch-all
	// background workers when the user doesn't set max_threads. Caps the
	// number of distinct lazy-started instances from a single catch-all.
	defaultMaxBackgroundWorkers = 16

	// defaultEnsureTimeout is the default deadline applied when ensure() is
	// called without an explicit timeout.
	defaultEnsureTimeout = 30 * time.Second
)

// backgroundWorkerExtras holds bg-only lifecycle state.
type backgroundWorkerExtras struct {
	// ready is shared by named workers and a catch-all's eager pool;
	// lazy-spawned catch-all instances each get their own slot in
	// catchAllNames.
	ready *backgroundWorkerState

	// catchAllNames != nil marks this *worker as a scope's catch-all
	// template. Lazy-spawned threads register here, up to catchAllCap.
	catchAllCap   int
	catchAllMu    sync.Mutex
	catchAllNames map[string]*backgroundWorkerState

	// lazyMu/lazyStarted gate the first thread spawn for a num=0 named
	// bg worker. Unused for eager (num > 0) or catch-all templates.
	lazyMu      sync.Mutex
	lazyStarted bool
}

// bootFailureInfo is the boot-phase crash metadata surfaced by ensure().
type bootFailureInfo struct {
	entrypoint   string
	exitStatus   int
	failureCount int
	phpError     string
}

// Scope isolates background workers between php_server blocks; the
// zero value is the global/embed scope. Obtain values via NextScope.
type Scope uint64

var scopeCounter atomic.Uint64

// NextScope returns a fresh scope value. Each php_server block should
// call this once during provisioning.
func NextScope() Scope {
	return Scope(scopeCounter.Add(1))
}

// scopeLabels maps Scope -> human-readable label registered by the
// embedder (e.g. the Caddy module).
var scopeLabels sync.Map

// SetScopeLabel attaches a human-readable label to a scope; the bg-worker
// metric emitter renders it as e.g. server="api.example.com" instead of
// an opaque numeric id. Empty labels are ignored.
func SetScopeLabel(s Scope, label string) {
	if label == "" {
		return
	}
	scopeLabels.Store(s, label)
}

// scopeLabelOrID returns the label registered for s, or the numeric id
// when none is set (including the zero/global scope), so callers always
// get a non-empty value.
func scopeLabelOrID(s Scope) string {
	if label, ok := lookupScopeLabel(s); ok {
		return label
	}
	return strconv.FormatUint(uint64(s), 10)
}

// lookupScopeLabel reports whether a label has been registered for s,
// returning ("", false) when none has. Distinguishes "unset" from
// "explicitly empty" without the numeric fallback.
func lookupScopeLabel(s Scope) (string, bool) {
	v, ok := scopeLabels.Load(s)
	if !ok {
		return "", false
	}
	return v.(string), true
}

// bgWorkerMetricName formats the metric label for a background worker:
// "m#<scopeLabel>:<runtimeName>". scopeLabel is empty when the scope
// has no registered label (embed/global, or before the embedder calls
// SetScopeLabel). The "m#" prefix mirrors the m# convention used for
// module workers; the colon keeps the format uniform so a single regex
// (m#([^:]*):(.+)) parses both labelled and unlabelled forms.
func bgWorkerMetricName(scope Scope, runtimeName string) string {
	label, _ := lookupScopeLabel(scope)
	return "m#" + label + ":" + runtimeName
}

// backgroundLookups maps scope -> lookup. Scope 0 is the global/embed scope.
var backgroundLookups map[Scope]*backgroundWorkerLookup

// backgroundWorkerLookup resolves a user-facing worker name to its *worker;
// catchAll is the fallback when byName misses.
type backgroundWorkerLookup struct {
	byName   map[string]*worker
	catchAll *worker
}

func newBackgroundWorkerLookup() *backgroundWorkerLookup {
	return &backgroundWorkerLookup{
		byName: make(map[string]*worker),
	}
}

// resolve returns the worker for the given name, falling back to catchAll.
func (l *backgroundWorkerLookup) resolve(name string) *worker {
	if w, ok := l.byName[name]; ok {
		return w
	}
	return l.catchAll
}

// isCatchAllName reports whether (name, fileName) designates a catch-all
// (no user-supplied name; newWorker defaults the name to the absolute
// file path). m# is stripped because module workers carry the prefix.
func isCatchAllName(name, fileName string) bool {
	phpName := strings.TrimPrefix(name, "m#")
	return phpName == "" || phpName == fileName
}

func isCatchAllByName(w *worker) bool {
	return isCatchAllName(w.name, w.fileName)
}

// buildBackgroundWorkerLookups maps each declared bg worker into its scope's
// lookup. Per-scope name collisions are caught here because bg workers
// intentionally skip the global workersByName map (so two scopes can share
// a user-facing name).
func buildBackgroundWorkerLookups(workers []*worker, opts []workerOpt) (map[Scope]*backgroundWorkerLookup, error) {
	lookups := make(map[Scope]*backgroundWorkerLookup)

	for i, o := range opts {
		if !o.isBackgroundWorker {
			continue
		}

		scope := o.scope
		lookup, ok := lookups[scope]
		if !ok {
			lookup = newBackgroundWorkerLookup()
			lookups[scope] = lookup
		}

		w := workers[i]
		w.scope = scope

		if isCatchAllByName(w) {
			if lookup.catchAll != nil {
				return nil, fmt.Errorf("duplicate catch-all background worker in the same scope")
			}
			w.bg.catchAllCap = defaultMaxBackgroundWorkers
			if o.maxThreads > 0 {
				w.bg.catchAllCap = o.maxThreads
			}
			w.bg.catchAllNames = make(map[string]*backgroundWorkerState)
			lookup.catchAll = w
		} else {
			phpName := strings.TrimPrefix(w.name, "m#")
			if _, exists := lookup.byName[phpName]; exists {
				return nil, fmt.Errorf("duplicate background worker name %q in the same scope", phpName)
			}
			lookup.byName[phpName] = w
		}
	}

	if len(lookups) == 0 {
		return nil, nil
	}
	return lookups, nil
}

// reserveBackgroundWorkerThreads resolves max_threads defaults and
// returns the thread budget to add to the pool. Mutates opt.workers
// in place and pre-registers totalWorkers so a bg-only deployment
// has the metric initialised.
func reserveBackgroundWorkerThreads(opt *opt) int {
	reserved := 0
	for i, w := range opt.workers {
		if !w.isBackgroundWorker {
			continue
		}
		isCatchAll := isCatchAllName(w.name, w.fileName)

		if w.maxThreads == 0 {
			switch {
			case isCatchAll:
				// Lazy cap default for any catch-all.
				opt.workers[i].maxThreads = defaultMaxBackgroundWorkers
			case w.num == 0:
				// Single-thread budget for a lazy named worker.
				opt.workers[i].maxThreads = 1
			}
		}

		var extra int
		if isCatchAll {
			// eager pool + lazy cap (independent budgets)
			extra = w.num + opt.workers[i].maxThreads
		} else {
			extra = w.num
			if opt.workers[i].maxThreads > extra {
				extra = opt.workers[i].maxThreads
			}
		}
		if extra < 1 {
			extra = 1
		}
		reserved += extra
		metrics.TotalWorkers(bgWorkerMetricName(w.scope, w.name), extra)
	}
	return reserved
}

// getLookup returns the background-worker lookup for the calling thread,
// resolving the scope via worker handler -> request context -> global. A
// scope with no declared workers falls through to scope 0 so embed-mode
// workers stay reachable; declared scopes stay strictly isolated.
func getLookup(thread *phpThread) *backgroundWorkerLookup {
	if backgroundLookups == nil {
		return nil
	}
	var scope Scope
	if thread != nil {
		if w := thread.handler.scopedWorker(); w != nil {
			scope = w.scope
		} else if fc, ok := fromContext(thread.context()); ok {
			scope = fc.scope
		}
	}
	if scope != 0 {
		if l := backgroundLookups[scope]; l != nil {
			return l
		}
	}
	return backgroundLookups[0]
}

// startBackgroundWorker resolves `name` via lookup.byName / lookup.catchAll,
// lazy-starting the thread if needed, and returns the per-instance state
// slot the caller should wait on. Safe to call concurrently.
func startBackgroundWorker(thread *phpThread, bgWorkerName string) (*backgroundWorkerState, error) {
	if bgWorkerName == "" {
		return nil, fmt.Errorf("background worker name must not be empty")
	}
	lookup := getLookup(thread)
	if lookup == nil {
		return nil, fmt.Errorf("no background worker configured")
	}

	// byName is keyed by the user-facing (m#-stripped) name.
	if w, ok := lookup.byName[bgWorkerName]; ok {
		sk, err := lazyStartNamedWorker(w)
		if err != nil {
			return nil, err
		}
		return sk, nil
	}

	catchAll := lookup.catchAll
	if catchAll == nil {
		return nil, fmt.Errorf("no background worker configured for name %q", bgWorkerName)
	}

	// Reject so behavior doesn't silently split-brain across the eager
	// pool and a lazy-spawned instance. m#-strip matches
	// buildBackgroundWorkerLookups: module catch-alls carry the prefix,
	// bgWorkerName from PHP never does.
	if bgWorkerName == strings.TrimPrefix(catchAll.name, "m#") {
		return nil, fmt.Errorf(`cannot ensure() against "%s": it matches the catch-all's own name; use a distinct user-facing name`, bgWorkerName)
	}

	// Hold catchAllMu across thread reservation + entry publication so a
	// failed allocation can't leave a phantom registration visible to
	// concurrent callers.
	bg := catchAll.bg
	bg.catchAllMu.Lock()

	if sk, ok := bg.catchAllNames[bgWorkerName]; ok {
		bg.catchAllMu.Unlock()
		return sk, nil
	}

	if bg.catchAllCap > 0 && len(bg.catchAllNames) >= bg.catchAllCap {
		bg.catchAllMu.Unlock()
		return nil, fmt.Errorf("cannot start background worker %q: limit of %d reached (increase max threads or declare it as a named worker)", bgWorkerName, bg.catchAllCap)
	}

	sk := newBackgroundWorkerState()
	bg.catchAllNames[bgWorkerName] = sk
	bg.catchAllMu.Unlock()

	if _, err := addBackgroundWorkerThread(catchAll, bgWorkerName, sk); err != nil {
		// Wake any concurrent waiter that picked up sk from catchAllNames
		// between our publish and this rollback so they see the start
		// failure instead of timing out.
		sk.abort(err)
		bg.catchAllMu.Lock()
		delete(bg.catchAllNames, bgWorkerName)
		bg.catchAllMu.Unlock()
		return nil, fmt.Errorf("cannot start background worker %q: %w (increase max threads)", bgWorkerName, err)
	}

	if globalLogger.Enabled(globalCtx, slog.LevelInfo) {
		globalLogger.LogAttrs(globalCtx, slog.LevelInfo, "background worker started",
			slog.String("name", bgWorkerName))
	}

	return sk, nil
}

// lazyStartNamedWorker returns the readiness slot the caller should
// wait on. For num=0 workers it spawns the first thread under
// bg.lazyMu (idempotent); the snapshot captured under the lock stays
// consistent with any concurrent invalidateBackgroundEntry.
func lazyStartNamedWorker(w *worker) (*backgroundWorkerState, error) {
	if w.num > 0 {
		return w.bg.ready, nil
	}
	w.bg.lazyMu.Lock()
	defer w.bg.lazyMu.Unlock()
	if w.bg.lazyStarted {
		return w.bg.ready, nil
	}
	r := w.bg.ready
	if _, err := addBackgroundWorkerThread(w, w.name, r); err != nil {
		return nil, fmt.Errorf("cannot start background worker %q: %w (increase max threads)", w.name, err)
	}
	w.bg.lazyStarted = true
	return r, nil
}

// isBootstrapEnsure reports whether the calling thread is inside an HTTP
// worker's boot phase. Bootstrap callers take the fail-fast path; runtime
// callers (bg workers, classic requests) take the tolerant lazy-start path.
func isBootstrapEnsure(thread *phpThread) bool {
	handler, ok := thread.handler.(*workerThread)
	return ok && handler.isBootingScript
}

// formatBackgroundWorkerTimeoutError produces the timeout-error message
// for an ensure() that didn't reach combined readiness in time. Mentions
// the boot failure if one was recorded; otherwise reports which half of
// the readiness signal the worker missed.
func formatBackgroundWorkerTimeoutError(name string, sk *backgroundWorkerState, timeout time.Duration) string {
	if info := sk.bootFailure.Load(); info != nil {
		msg := fmt.Sprintf("background worker %q did not become ready within %s; last attempt %d failed (exit status %d, entrypoint %s)",
			name, timeout, info.failureCount, info.exitStatus, info.entrypoint)
		if info.phpError != "" {
			msg += ": " + info.phpError
		}
		return msg
	}
	missing := []string{}
	if !sk.hasHandle.Load() {
		missing = append(missing, "frankenphp_get_worker_handle()")
	}
	if !sk.hasVars.Load() {
		missing = append(missing, "frankenphp_set_vars()")
	}
	if len(missing) == 0 {
		return fmt.Sprintf("background worker %q did not become ready within %s", name, timeout)
	}
	return fmt.Sprintf("background worker %q did not call %s within %s", name, strings.Join(missing, " and "), timeout)
}

// errBackgroundWorkerNotInsideBgThread is returned by set_vars when called
// outside a bg-worker thread. Package-level so tests can match it.
var errBackgroundWorkerNotInsideBgThread = errors.New("frankenphp_set_vars() can only be called from a background worker")

// go_frankenphp_ensure_background_worker lazy-starts each named bg worker
// (C side has validated names are non-empty + unique) and blocks until
// each reaches combined readiness (get_worker_handle + set_vars), aborts,
// or timeoutMs elapses (<=0 = default). Bootstrap callers (HTTP worker
// pre-handle_request) fail fast on boot failures; runtime callers wait
// out the restart/backoff cycle.
//
//export go_frankenphp_ensure_background_worker
func go_frankenphp_ensure_background_worker(threadIndex C.uintptr_t, names **C.char, nameLens *C.size_t, nameCount C.int, timeoutMs C.int64_t) *C.char {
	thread := phpThreads[threadIndex]
	timeout := time.Duration(int64(timeoutMs)) * time.Millisecond
	if timeout <= 0 {
		timeout = defaultEnsureTimeout
	}

	n := int(nameCount)
	nameSlice := unsafe.Slice(names, n)
	nameLenSlice := unsafe.Slice(nameLens, n)
	bootstrap := isBootstrapEnsure(thread)

	// Start each named worker first. Reserve their states so a shared
	// deadline applies across the whole group (the caller gets one
	// timeout value, not one per worker).
	sks := make([]*backgroundWorkerState, n)
	goNames := make([]string, n)
	for i := 0; i < n; i++ {
		goNames[i] = C.GoStringN(nameSlice[i], C.int(nameLenSlice[i]))
		sk, err := startBackgroundWorker(thread, goNames[i])
		if err != nil {
			return C.CString(err.Error())
		}
		sks[i] = sk
	}

	deadline := time.After(timeout)
	if bootstrap {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for i, sk := range sks {
		wait:
			for {
				select {
				case <-sk.ready:
					break wait
				case <-sk.aborted:
					return C.CString(sk.abortErr.Error())
				case <-deadline:
					return C.CString(formatBackgroundWorkerTimeoutError(goNames[i], sk, timeout))
				case <-globalCtx.Done():
					return C.CString("frankenphp is shutting down")
				case <-ticker.C:
					if sk.bootFailure.Load() != nil {
						return C.CString(formatBackgroundWorkerTimeoutError(goNames[i], sk, timeout))
					}
				}
			}
		}
		return nil
	}

	for i, sk := range sks {
		select {
		case <-sk.ready:
		case <-sk.aborted:
			return C.CString(sk.abortErr.Error())
		case <-deadline:
			return C.CString(formatBackgroundWorkerTimeoutError(goNames[i], sk, timeout))
		case <-globalCtx.Done():
			return C.CString("frankenphp is shutting down")
		}
	}
	return nil
}

// go_frankenphp_worker_ready marks the handle half of the combined
// readiness signal on the per-thread state slot. The slot's ready channel
// closes only once both handle and set_vars have fired. Idempotent.
//
//export go_frankenphp_worker_ready
func go_frankenphp_worker_ready(threadIndex C.uintptr_t) {
	thread := phpThreads[threadIndex]
	if thread == nil {
		return
	}
	handler, ok := thread.handler.(*backgroundWorkerThread)
	if !ok || handler == nil {
		return
	}
	if sk := handler.backgroundWorker; sk != nil {
		sk.markHandle()
	}
}

// go_frankenphp_set_vars is called from PHP when a background worker
// publishes its shared vars. The caller has already deep-copied the vars
// into persistent memory; here we swap the pointer under the state lock
// and hand back the old pointer so the C side can free it after the call.
//
//export go_frankenphp_set_vars
func go_frankenphp_set_vars(threadIndex C.uintptr_t, varsPtr unsafe.Pointer, oldPtr *unsafe.Pointer) *C.char {
	thread := phpThreads[threadIndex]

	bgHandler, ok := thread.handler.(*backgroundWorkerThread)
	if !ok || bgHandler.backgroundWorker == nil {
		return C.CString(errBackgroundWorkerNotInsideBgThread.Error())
	}

	sk := bgHandler.backgroundWorker

	sk.mu.Lock()
	*oldPtr = sk.varsPtr
	sk.varsPtr = varsPtr
	sk.varsVersion.Add(1)
	sk.mu.Unlock()

	bgHandler.markBackgroundReady()

	return nil
}

// go_frankenphp_get_vars resolves the named worker through the lookup
// (named or catch-all), checks its sk.ready without starting the worker,
// and copies its vars into the return value. If the caller hasn't called
// ensure() first, this returns a "not ready" error.
//
//export go_frankenphp_get_vars
func go_frankenphp_get_vars(threadIndex C.uintptr_t, name *C.char, nameLen C.size_t, returnValue *C.zval) *C.char {
	thread := phpThreads[threadIndex]
	lookup := getLookup(thread)
	if lookup == nil {
		return C.CString("no background worker configured")
	}

	goName := C.GoStringN(name, C.int(nameLen))
	var sk *backgroundWorkerState
	if w, ok := lookup.byName[goName]; ok {
		sk = w.bg.ready
	} else if ca := lookup.catchAll; ca != nil {
		ca.bg.catchAllMu.Lock()
		sk = ca.bg.catchAllNames[goName]
		ca.bg.catchAllMu.Unlock()
	}
	if sk == nil {
		return C.CString("background worker not found: " + goName + " (call frankenphp_ensure_background_worker first)")
	}

	select {
	case <-sk.ready:
	default:
		return C.CString("background worker not ready: " + goName + " (no set_vars call yet)")
	}

	sk.mu.RLock()
	C.frankenphp_copy_persistent_vars(returnValue, sk.varsPtr)
	sk.mu.RUnlock()

	return nil
}
