package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// A worker script uses opcache's shared memory from its start until it ends.
// A restart of that memory either waits for the worker scripts, or reuses
// memory that they still reference. When opcache schedules a restart, worker
// scripts restart at their next request boundary, and idle ones right away.
// A worker script that starts while the restart is pending isn't cached by
// opcache, so it starts again once the restart is done.

// opcacheRestartCooldown is the minimum time between two rounds of worker
// restarts, so that a cache too small for the app doesn't restart the workers
// in a loop
var opcacheRestartCooldown = time.Minute

var opcacheRestarts struct {
	// round is incremented each time the worker scripts must restart
	round atomic.Uint64
	// wake is closed, then replaced, to wake up the idle worker threads
	wake atomic.Pointer[chan struct{}]

	mu        sync.Mutex
	lastRound time.Time
	delayed   *time.Timer
}

func init() {
	wake := make(chan struct{})
	opcacheRestarts.wake.Store(&wake)
}

// restartWorkersForOpcache starts a round of worker restarts, or delays it
// until the cooldown has elapsed
func restartWorkersForOpcache() {
	o := &opcacheRestarts
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.delayed != nil {
		return
	}

	wait := opcacheRestartCooldown - time.Since(o.lastRound)
	if wait <= 0 {
		startOpcacheRestartRound()

		return
	}

	if globalLogger.Enabled(globalCtx, slog.LevelWarn) {
		globalLogger.LogAttrs(globalCtx, slog.LevelWarn, "opcache restarts are too frequent, delaying the restart of the workers", slog.Duration("delay", wait))
	}

	o.delayed = time.AfterFunc(wait, func() {
		o.mu.Lock()
		defer o.mu.Unlock()

		if o.delayed != nil {
			o.delayed = nil
			startOpcacheRestartRound()
		}
	})
}

// startOpcacheRestartRound must be called with opcacheRestarts.mu held
func startOpcacheRestartRound() {
	opcacheRestarts.lastRound = time.Now()
	opcacheRestarts.round.Add(1)
	wakeWorkersForOpcache()
}

// wakeWorkersForOpcache wakes up the idle worker threads, so that they check
// whether they must restart their script
func wakeWorkersForOpcache() {
	wake := make(chan struct{})
	close(*opcacheRestarts.wake.Swap(&wake))
}

func opcacheWakeChan() <-chan struct{} {
	return *opcacheRestarts.wake.Load()
}

func resetOpcacheRestarts() {
	o := &opcacheRestarts
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.delayed != nil {
		o.delayed.Stop()
		o.delayed = nil
	}
	o.lastRound = time.Time{}
}

// opcacheStarted records how opcache treats the worker script that just started
func (handler *workerThread) opcacheStarted() {
	handler.opcacheStale = false
	handler.opcacheUncached = false
	if opcacheRestarts.round.Load() == 0 {
		// no restart was scheduled so far
		return
	}

	state := C.frankenphp_opcache_state()
	switch {
	case state < 0:
	case state&C.FRANKENPHP_OPCACHE_CACHED != 0:
		// the script started right before opcache marked the restart as
		// pending: it uses the memory the restart reuses
		handler.opcacheStale = state&C.FRANKENPHP_OPCACHE_RESTART_PENDING != 0
		handler.opcacheRetried = false
	case state&C.FRANKENPHP_OPCACHE_RESTART_PENDING != 0:
		// start again once the restart is done
		handler.opcacheUncached = true
	default:
		// the script started while the restart was running, or opcache
		// skips it for another reason: start again, but only once
		handler.opcacheUncached = !handler.opcacheRetried
	}
}

// opcacheRestartNeeded tells whether the worker script must restart because
// of an opcache restart
func (handler *workerThread) opcacheRestartNeeded() bool {
	if round := opcacheRestarts.round.Load(); handler.opcacheStale || handler.opcacheRound != round {
		// the script uses the memory the restart reuses; ask only once, in
		// case the script keeps calling frankenphp_handle_request()
		handler.opcacheStale = false
		handler.opcacheRound = round
		handler.opcacheRetried = false

		return true
	}

	if !handler.opcacheUncached {
		return false
	}

	state := C.frankenphp_opcache_state()
	if state >= 0 && state&C.FRANKENPHP_OPCACHE_RESTART_PENDING != 0 {
		// keep running without the cache until the restart is done
		return false
	}

	handler.opcacheUncached = false
	handler.opcacheRetried = true

	return state >= 0
}
