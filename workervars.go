package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"errors"
	"runtime/cgo"
	"strconv"
	"sync"
	"sync/atomic"
)

// varsSnapshot is one publication of a vars namespace: a persistent
// HashTable, copied into request memory by each frankenphp_get_vars()
// reader. The copy runs in C with no lock held, a bailout may end it, so refs
// counts the slot and the readers holding the snapshot, and the last one to
// let go frees it.
type varsSnapshot struct {
	table *C.HashTable
	refs  atomic.Int32
}

// release lets go of a reference, and hands back the table to free once
// nobody holds it anymore
func (s *varsSnapshot) release() *C.HashTable {
	if s.refs.Add(-1) == 0 {
		return s.table
	}

	return nil
}

// varsSlot holds the current snapshot of a vars namespace, published through
// WorkerHandle::setVars(). It belongs to the worker, not to a thread, so it
// survives script restarts and serves stale data meanwhile.
type varsSlot struct {
	mu      sync.RWMutex
	current *varsSnapshot
}

var (
	// the publisher of each vars namespace, per scope
	varsOwners namespaceOwners

	// booting background workers blocked in frankenphp_get_vars() on other
	// workers, keyed by waiter: a cycle would deadlock Init(), refuse it
	varsWaitMu sync.Mutex
	varsWaitOn = map[*worker]map[*worker]int{}
)

// varsOwner resolves the publisher of a vars namespace: within the caller's
// server first, then among global workers
func varsOwner(fc *frankenPHPContext, ns string) (*worker, error) {
	var server *Server
	if fc != nil {
		server = fc.server
	}
	w := varsOwners.resolve(server, ns)
	if w == nil {
		return nil, errors.New("frankenphp_get_vars(): no background worker publishes the vars namespace " + strconv.Quote(ns))
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
		if caller == target || varsWaitReaches(target, caller) {
			varsWaitMu.Unlock()

			return errors.New("frankenphp_get_vars(): circular dependency between background workers " + strconv.Quote(caller.name) + " and " + strconv.Quote(target.name))
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
				if table := slot.current.release(); table != nil {
					C.frankenphp_vars_free(table)
				}
				slot.current = nil
			}
			slot.mu.Unlock()
		}
	}
}

// go_frankenphp_set_vars stores a snapshot in a namespace of the calling
// worker and hands back the one it replaces, for the C side to free unless a
// reader still copies it; on error, the C side frees the snapshot it passed
//
//export go_frankenphp_set_vars
func go_frankenphp_set_vars(threadIndex C.uintptr_t, namespace *C.char, namespaceLen C.size_t, table *C.HashTable, old **C.HashTable) *C.char {
	handler, ok := phpThreads[threadIndex].handler.(*backgroundWorkerThread)
	if !ok {
		// refused on the C side already
		return C.CString("FrankenPHP\\WorkerHandle::setVars() can only be called from a background worker")
	}

	ns := C.GoStringN(namespace, C.int(namespaceLen))
	slot := handler.worker.vars[ns]
	if slot == nil {
		return C.CString("FrankenPHP\\WorkerHandle::setVars(): this worker does not publish the vars namespace " + strconv.Quote(ns))
	}

	s := &varsSnapshot{table: table}
	s.refs.Store(1)

	slot.mu.Lock()
	previous := slot.current
	slot.current = s
	slot.mu.Unlock()
	if previous != nil {
		*old = previous.release()
	}
	handler.publishedVars[ns] = true

	return nil
}

// go_frankenphp_get_vars hands the current snapshot of a namespace to the C
// side, which copies it into request memory and then lets go of it through
// go_frankenphp_vars_done
//
//export go_frankenphp_get_vars
func go_frankenphp_get_vars(threadIndex C.uintptr_t, namespace *C.char, namespaceLen C.size_t, table **C.HashTable, snapshot *C.uintptr_t) *C.char {
	thread := phpThreads[threadIndex]
	ns := C.GoStringN(namespace, C.int(namespaceLen))
	target, err := varsOwner(thread.handler.frankenPHPContext(), ns)
	if err != nil {
		return C.CString(err.Error())
	}

	var caller *worker
	if handler, ok := thread.handler.(*backgroundWorkerThread); ok && handler.isBootingScript {
		caller = handler.worker
	}
	if err := waitVarsReady(target, caller); err != nil {
		return C.CString(err.Error())
	}

	// a worker is ready only once every namespace it declares has a snapshot
	slot := target.vars[ns]
	slot.mu.RLock()
	s := slot.current
	s.refs.Add(1)
	slot.mu.RUnlock()

	*table = s.table
	*snapshot = C.uintptr_t(cgo.NewHandle(s))

	return nil
}

// go_frankenphp_vars_done lets go of the snapshot a frankenphp_get_vars()
// call copied, and hands back its table for the C side to free when its slot
// moved on meanwhile
//
//export go_frankenphp_vars_done
func go_frankenphp_vars_done(snapshot C.uintptr_t) *C.HashTable {
	h := cgo.Handle(snapshot)
	s := h.Value().(*varsSnapshot)
	h.Delete()

	return s.release()
}

// go_frankenphp_worker_vars lists the vars namespaces of the calling worker,
// for WorkerHandle::getNamespaces()
//
//export go_frankenphp_worker_vars
func go_frankenphp_worker_vars(threadIndex C.uintptr_t, array *C.zval) {
	if handler, ok := phpThreads[threadIndex].handler.(*backgroundWorkerThread); ok {
		for _, ns := range handler.worker.varsNamespaces {
			C.frankenphp_array_append_string(array, toUnsafeChar(ns), C.size_t(len(ns)))
		}
	}
}

// go_frankenphp_worker_tasks lists the task namespaces of the calling worker,
// for WorkerHandle::getNamespaces()
//
//export go_frankenphp_worker_tasks
func go_frankenphp_worker_tasks(threadIndex C.uintptr_t, array *C.zval) {
	if handler, ok := phpThreads[threadIndex].handler.(*backgroundWorkerThread); ok {
		for _, ns := range handler.worker.taskNamespaces {
			C.frankenphp_array_append_string(array, toUnsafeChar(ns), C.size_t(len(ns)))
		}
	}
}
