package state

import (
	"slices"
	"sync"
	"time"
)

type State string

const (
	// livecycle States of a thread
	Reserved      State = "reserved"
	Booting       State = "booting"
	BootRequested State = "boot requested"
	ShuttingDown  State = "shutting down"
	Done          State = "done"

	// these States are 'stable' and safe to transition from at any time
	Inactive State = "inactive"
	Ready    State = "ready"

	// States necessary for restarting workers
	Restarting State = "restarting"
	Yielding   State = "yielding"

	// States necessary for transitioning between different handlers
	TransitionRequested  State = "transition requested"
	TransitionInProgress State = "transition in progress"
	TransitionComplete   State = "transition complete"
)

type ThreadState struct {
	currentState State
	mu           sync.RWMutex
	subscribers  []stateSubscriber
	// how long threads have been waiting in stable states
	waitingSince time.Time
	isWaiting    bool
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
	return string(ts.Get())
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

	var newSubscribers []stateSubscriber

	// notify subscribers to the state change
	for _, sub := range ts.subscribers {
		if !slices.Contains(sub.states, nextState) {
			newSubscribers = append(newSubscribers, sub)

			continue
		}

		close(sub.ch)
	}

	ts.subscribers = newSubscribers
}

// block until the thread reaches a certain state
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

// safely request a state change from a different goroutine
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
	ts.mu.Lock()
	if isWaiting {
		ts.isWaiting = true
		ts.waitingSince = time.Now()
	} else {
		ts.isWaiting = false
	}
	ts.mu.Unlock()
}

// IsInWaitingState returns true if a thread is waiting for a request or shutdown
func (ts *ThreadState) IsInWaitingState() bool {
	ts.mu.RLock()
	isWaiting := ts.isWaiting
	ts.mu.RUnlock()

	return isWaiting
}

// WaitTime returns the time since the thread is waiting in a stable state in ms
func (ts *ThreadState) WaitTime() int64 {
	ts.mu.RLock()
	waitTime := int64(0)
	if ts.isWaiting {
		waitTime = time.Now().UnixMilli() - ts.waitingSince.UnixMilli()
	}
	ts.mu.RUnlock()

	return waitTime
}

func (ts *ThreadState) SetWaitTime(t time.Time) {
	ts.mu.Lock()
	ts.waitingSince = t
	ts.mu.Unlock()
}
