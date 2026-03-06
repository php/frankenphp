package state

import "C"
import (
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

type State int

const (
	// lifecycle States of a thread
	Reserved State = iota
	Booting
	BootRequested
	ShuttingDown
	Done

	// these States are 'stable' and safe to transition from at any time
	Inactive
	Ready

	// States necessary for restarting workers
	Restarting
	Yielding

	// States necessary for transitioning between different handlers
	TransitionRequested
	TransitionInProgress
	TransitionComplete
)

func (s State) String() string {
	switch s {
	case Reserved:
		return "reserved"
	case Booting:
		return "booting"
	case BootRequested:
		return "boot requested"
	case ShuttingDown:
		return "shutting down"
	case Done:
		return "done"
	case Inactive:
		return "inactive"
	case Ready:
		return "ready"
	case Restarting:
		return "restarting"
	case Yielding:
		return "yielding"
	case TransitionRequested:
		return "transition requested"
	case TransitionInProgress:
		return "transition in progress"
	case TransitionComplete:
		return "transition complete"
	default:
		return "unknown"
	}
}

type ThreadState struct {
	currentState State
	mu           sync.RWMutex
	subscribers  []stateSubscriber
	// how long threads have been waiting in stable states (unix ms, 0 = not waiting)
	waitingSince atomic.Int64
}

type stateSubscriber struct {
	states []State
	ch     chan struct{}
}

func NewThreadState() *ThreadState {
	return &ThreadState{
		currentState: Reserved,
		subscribers:  []stateSubscriber{},
		mu:           sync.RWMutex{},
	}
}

func (ts *ThreadState) Is(state State) bool {
	ts.mu.RLock()
	ok := ts.currentState == state
	ts.mu.RUnlock()

	return ok
}

func (ts *ThreadState) CompareAndSwap(compareTo State, swapTo State) bool {
	ts.mu.Lock()
	ok := ts.currentState == compareTo
	if ok {
		ts.currentState = swapTo
		ts.notifySubscribers(swapTo)
	}
	ts.mu.Unlock()

	return ok
}

func (ts *ThreadState) Name() string {
	return ts.Get().String()
}

func (ts *ThreadState) Get() State {
	ts.mu.RLock()
	id := ts.currentState
	ts.mu.RUnlock()

	return id
}

func (ts *ThreadState) Set(nextState State) {
	ts.mu.Lock()
	ts.currentState = nextState
	ts.notifySubscribers(nextState)
	ts.mu.Unlock()
}

func (ts *ThreadState) notifySubscribers(nextState State) {
	if len(ts.subscribers) == 0 {
		return
	}

	n := 0
	for _, sub := range ts.subscribers {
		if !slices.Contains(sub.states, nextState) {
			ts.subscribers[n] = sub
			n++
			continue
		}
		close(sub.ch)
	}

	ts.subscribers = ts.subscribers[:n]
}

// WaitFor blocks until the thread reaches a certain state
func (ts *ThreadState) WaitFor(states ...State) {
	ts.mu.Lock()
	if slices.Contains(states, ts.currentState) {
		ts.mu.Unlock()

		return
	}

	sub := stateSubscriber{
		states: states,
		ch:     make(chan struct{}),
	}
	ts.subscribers = append(ts.subscribers, sub)
	ts.mu.Unlock()
	<-sub.ch
}

// RequestSafeStateChange safely requests a state change from a different goroutine
func (ts *ThreadState) RequestSafeStateChange(nextState State) bool {
	ts.mu.Lock()
	switch ts.currentState {
	// disallow state changes if shutting down or done
	case ShuttingDown, Done, Reserved:
		ts.mu.Unlock()

		return false
	// ready and inactive are safe states to transition from
	case Ready, Inactive:
		ts.currentState = nextState
		ts.notifySubscribers(nextState)
		ts.mu.Unlock()

		return true
	}
	ts.mu.Unlock()

	// wait for the state to change to a safe state
	ts.WaitFor(Ready, Inactive, ShuttingDown)

	return ts.RequestSafeStateChange(nextState)
}

// MarkAsWaiting hints that the thread reached a stable state and is waiting for requests or shutdown
func (ts *ThreadState) MarkAsWaiting(isWaiting bool) {
	if isWaiting {
		ts.waitingSince.Store(time.Now().UnixMilli())
	} else {
		ts.waitingSince.Store(0)
	}
}

// IsInWaitingState returns true if a thread is waiting for a request or shutdown
func (ts *ThreadState) IsInWaitingState() bool {
	return ts.waitingSince.Load() != 0
}

// WaitTime returns the time since the thread is waiting in a stable state in ms
func (ts *ThreadState) WaitTime() int64 {
	since := ts.waitingSince.Load()
	if since == 0 {
		return 0
	}
	return time.Now().UnixMilli() - since
}

func (ts *ThreadState) SetWaitTime(t time.Time) {
	ts.waitingSince.Store(t.UnixMilli())
}
