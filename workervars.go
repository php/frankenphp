package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"errors"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"
)

// varsSlot holds the current snapshot of a vars namespace, published through
// WorkerHandle::setVars(): a persistent HashTable, copied into request memory
// by each frankenphp_get_vars() reader. The copy runs in C with no lock held,
// so the slot and the readers copying the table each hold a reference on it,
// and the last to let go frees it, see frankenphp_vars_release(). The slot
// belongs to the worker, not to a thread, so it survives script restarts and
// serves stale data meanwhile.
type varsSlot struct {
	mu      sync.RWMutex
	current *C.HashTable
}

var (
	// the publisher of each vars namespace, per scope
	varsOwners namespaceOwners

	// booting background workers blocked in frankenphp_get_vars() on other
	// workers, keyed by waiter: a cycle would deadlock Init(), refuse it
	varsWaitMu sync.Mutex
	varsWaitOn = map[*worker]map[*worker]int{}
)

// misuseError is an error PHP gets as a LogicException, a mistake in the
// code: the others depend on the configuration in place or on the moment, and
// are thrown as RuntimeException
type misuseError string

func (e misuseError) Error() string {
	return string(e)
}

// phpError hands an error over to the C side, with whether it is a misuse
func phpError(err error) (*C.char, C.bool) {
	var misuse misuseError

	return C.CString(err.Error()), C.bool(errors.As(err, &misuse))
}

// varsOwner resolves the publisher of a vars namespace: within the caller's
// server first, then among global workers
func varsOwner(thread *phpThread, ns string) (*worker, error) {
	w := varsOwners.resolve(namespaceScope(thread), ns)
	if w == nil {
		// what the configuration lacks, like a Mercure hub for frankenphp_mercure_publish()
		return nil, errors.New("frankenphp_get_vars(): no background worker publishes the vars namespace " + quoteNamespace(ns) + ", in this php_server or among the global workers")
	}

	return w, nil
}

// waitVarsReady blocks until target reached its ready point once. Requests
// cannot get here before that (activateServers() runs after initWorkers()),
// so a blocked caller is a worker still booting, a background one or the
// bootstrap of an HTTP one. Waits between background workers form a graph,
// and a cycle is refused instead of deadlocking Init(), as is any wait once
// the start failed
func waitVarsReady(target, caller *worker) error {
	select {
	case <-target.readyOnce:
		return nil
	default:
	}

	if caller != nil {
		varsWaitMu.Lock()
		if caller == target {
			varsWaitMu.Unlock()

			return misuseError("frankenphp_get_vars(): background worker " + strconv.Quote(caller.name) + " reads its own vars before its first tick, they are not published yet")
		}
		if varsWaitReaches(target, caller) {
			varsWaitMu.Unlock()

			return misuseError("frankenphp_get_vars(): circular dependency between background workers " + strconv.Quote(caller.name) + " and " + strconv.Quote(target.name))
		}
		if varsWaitOn[caller] == nil {
			varsWaitOn[caller] = map[*worker]int{}
		}
		varsWaitOn[caller][target]++
		varsWaitMu.Unlock()

		defer func() {
			varsWaitMu.Lock()
			if varsWaitOn[caller][target]--; varsWaitOn[caller][target] == 0 {
				delete(varsWaitOn[caller], target)
			}
			if len(varsWaitOn[caller]) == 0 {
				delete(varsWaitOn, caller)
			}
			varsWaitMu.Unlock()
		}()
	}

	select {
	case <-target.readyOnce:
		return nil
	case <-mainThread.done:
		return errors.New("frankenphp_get_vars(): FrankenPHP is shutting down")
	case <-startupFailed:
		return errors.New("frankenphp_get_vars(): FrankenPHP failed to start, " + strconv.Quote(target.name) + " is not ready")
	}
}

// varsWaitReaches reports whether from waits, transitively, on to; called
// with varsWaitMu held
func varsWaitReaches(from, to *worker) bool {
	for next := range varsWaitOn[from] {
		if next == to || varsWaitReaches(next, to) {
			return true
		}
	}

	return false
}

// freeWorkerVars releases the snapshots once no PHP thread can read them
// and before the engine goes away
func freeWorkerVars() {
	for _, w := range workers {
		for _, slot := range w.vars {
			slot.mu.Lock()
			if slot.current != nil {
				C.frankenphp_vars_release(slot.current)
				slot.current = nil
			}
			slot.mu.Unlock()
		}
	}
}

// go_frankenphp_set_vars stores a snapshot in a namespace of the calling
// worker and hands back the one it replaces, for the C side to let go of; on
// error, the C side frees the snapshot it passed
//
//export go_frankenphp_set_vars
func go_frankenphp_set_vars(threadIndex C.uintptr_t, namespace *C.char, namespaceLen C.size_t, table *C.HashTable, old **C.HashTable) *C.char {
	handler, ok := phpThreads[threadIndex].handler.(*backgroundWorkerThread)
	if !ok {
		// refused on the C side already
		return C.CString("FrankenPHP\\WorkerHandle::setVars() can only be called from a background worker")
	}

	// looked up first, and only kept once it is known
	ns := unsafe.String((*byte)(unsafe.Pointer(namespace)), int(namespaceLen))
	slot := handler.worker.vars[ns]
	if slot == nil {
		return C.CString("FrankenPHP\\WorkerHandle::setVars(): this worker does not publish the vars namespace " + quoteNamespace(ns))
	}
	ns = strings.Clone(ns)

	// the table comes with one reference, the slot's
	slot.mu.Lock()
	*old = slot.current
	slot.current = table
	slot.mu.Unlock()
	handler.publishedVars[ns] = true

	return nil
}

// go_frankenphp_get_vars hands the current snapshot of a namespace to the C
// side with a reference on it, to copy it into request memory and then let go
// of it. Nothing of the read is kept here: a thread that borrows the index of
// another, as the children of ext-parallel do, reads at the same time
//
//export go_frankenphp_get_vars
func go_frankenphp_get_vars(threadIndex C.uintptr_t, namespace *C.char, namespaceLen C.size_t, table **C.HashTable) (*C.char, C.bool) {
	thread := phpThreads[threadIndex]
	// looked up, never kept
	ns := unsafe.String((*byte)(unsafe.Pointer(namespace)), int(namespaceLen))
	target, err := varsOwner(thread, ns)
	if err != nil {
		return phpError(err)
	}

	var caller *worker
	if handler, ok := thread.handler.(*backgroundWorkerThread); ok && handler.isBootingScript {
		caller = handler.worker
	}
	if err := waitVarsReady(target, caller); err != nil {
		return phpError(err)
	}

	// a worker is ready only once every namespace it declares has a snapshot
	slot := target.vars[ns]
	slot.mu.RLock()
	// under the lock, so the slot still holds its own reference
	atomic.AddUint32((*uint32)(unsafe.Pointer(&slot.current.gc.refcount)), 1)
	*table = slot.current
	slot.mu.RUnlock()

	return nil, false
}

// go_frankenphp_worker_namespaces builds what WorkerHandle::getNamespaces()
// returns for the calling worker; the C side refuses the other threads
//
//export go_frankenphp_worker_namespaces
func go_frankenphp_worker_namespaces(threadIndex C.uintptr_t, namespaces *C.zval) {
	worker := phpThreads[threadIndex].handler.(*backgroundWorkerThread).worker

	phpValueInto(AssociativeArray[any]{
		Map: map[string]any{
			"vars":  namespaceList(worker.varsNamespaces),
			"tasks": namespaceList(worker.taskNamespaces),
		},
		Order: []string{"vars", "tasks"},
	}, namespaces)
}

// namespaceList is a list of namespaces as phpValueInto() takes it
func namespaceList(namespaces []string) []any {
	list := make([]any, len(namespaces))
	for i, ns := range namespaces {
		list[i] = ns
	}

	return list
}
