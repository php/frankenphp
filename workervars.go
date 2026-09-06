package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"errors"
	"strconv"
	"sync"
)

// varsSlot holds the snapshot a background worker published through
// WorkerHandle::setVars(): a persistent HashTable, copied into request memory
// by each frankenphp_get_vars() reader. It belongs to the worker, not to a
// thread, so it survives script restarts and serves stale data meanwhile.
type varsSlot struct {
	mu    sync.RWMutex
	table *C.HashTable
}

var (
	// booting background workers blocked in frankenphp_get_vars() on other
	// workers, keyed by waiter: a cycle would deadlock Init(), refuse it
	varsWaitMu sync.Mutex
	varsWaitOn = map[*worker]map[*worker]int{}
)

// varsWorker resolves the name of a background worker
func varsWorker(name string) (*worker, error) {
	w := workersByName[name]
	if w == nil || !w.isBackgroundWorker {
		return nil, errors.New("frankenphp_get_vars(): unknown background worker " + strconv.Quote(name))
	}

	return w, nil
}

// waitVarsReady blocks until target reached its ready point once. Requests
// cannot get here before that (activateServers() runs after initWorkers()),
// so a blocked caller is a background worker still booting: waits between
// workers form a graph, and a cycle is refused instead of deadlocking Init()
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
		w.vars.mu.Lock()
		if w.vars.table != nil {
			C.frankenphp_vars_free(w.vars.table)
			w.vars.table = nil
		}
		w.vars.mu.Unlock()
	}
}

//export go_frankenphp_set_vars
func go_frankenphp_set_vars(threadIndex C.uintptr_t, table *C.HashTable) *C.HashTable {
	handler, ok := phpThreads[threadIndex].handler.(*backgroundWorkerThread)
	if !ok {
		// only a background worker can hold a handle; hand the table back so
		// it is freed
		return table
	}

	slot := &handler.worker.vars
	slot.mu.Lock()
	old := slot.table
	slot.table = table
	slot.mu.Unlock()

	return old
}

//export go_frankenphp_get_vars
func go_frankenphp_get_vars(threadIndex C.uintptr_t, name *C.char, nameLen C.size_t, returnValue *C.zval) *C.char {
	thread := phpThreads[threadIndex]
	target, err := varsWorker(C.GoStringN(name, C.int(nameLen)))
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

	slot := &target.vars
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.table == nil {
		return C.CString("frankenphp_get_vars(): background worker " + strconv.Quote(target.name) + " has not published any vars yet")
	}
	C.frankenphp_vars_to_request(returnValue, slot.table)

	return nil
}
