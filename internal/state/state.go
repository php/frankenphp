package frankenphp

import (
	"slices"
	"sync"
	"time"
)

type StateID uint8

const (
	// livecycle States of a thread
	StateReserved StateID = iota
	StateBooting
	StateBootRequested
	StateShuttingDown
	StateDone

	// these States are 'stable' and safe to transition from at any time
	StateInactive
	StateReady

	// States necessary for restarting workers
	StateRestarting
	StateYielding

	// States necessary for transitioning between different handlers
	StateTransitionRequested
	StateTransitionInProgress
	StateTransitionComplete
)

var stateNames = map[StateID]string{
	StateReserved:             "reserved",
	StateBooting:              "booting",
	StateInactive:             "inactive",
	StateReady:                "ready",
	StateShuttingDown:         "shutting down",
	StateDone:                 "done",
	StateRestarting:           "restarting",
	StateYielding:             "yielding",
	StateTransitionRequested:  "transition requested",
	StateTransitionInProgress: "transition in progress",
	StateTransitionComplete:   "transition complete",
}

type ThreadState struct {
	currentState StateID
	mu           sync.RWMutex
	subscribers  []stateSubscriber
	// how long threads have been waiting in stable states
	waitingSince time.Time
	isWaiting    bool
}

type stateSubscriber struct {
	states []StateID
	ch     chan struct{}
}

func NewThreadState() *ThreadState {
	return &ThreadState{
		currentState: StateReserved,
		subscribers:  []stateSubscriber{},
		mu:           sync.RWMutex{},
	}
}

func (ts *ThreadState) Is(state StateID) bool {
	ts.mu.RLock()
	ok := ts.currentState == state
	ts.mu.RUnlock()

	return ok
}

func (ts *ThreadState) CompareAndSwap(compareTo StateID, swapTo StateID) bool {
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
	return stateNames[ts.Get()]
}

func (ts *ThreadState) Get() StateID {
	ts.mu.RLock()
	id := ts.currentState
	ts.mu.RUnlock()

	return id
}

func (ts *ThreadState) Set(nextState StateID) {
	ts.mu.Lock()
	ts.currentState = nextState
	ts.notifySubscribers(nextState)
	ts.mu.Unlock()
}

func (ts *ThreadState) notifySubscribers(nextState StateID) {
	if len(ts.subscribers) == 0 {
		return
	}
	newSubscribers := []stateSubscriber{}
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
func (ts *ThreadState) WaitFor(states ...StateID) {
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
func (ts *ThreadState) RequestSafeStateChange(nextState StateID) bool {
	ts.mu.Lock()
	switch ts.currentState {
	// disallow state changes if shutting down or done
	case StateShuttingDown, StateDone, StateReserved:
		ts.mu.Unlock()
		return false
	// ready and inactive are safe states to transition from
	case StateReady, StateInactive:
		ts.currentState = nextState
		ts.notifySubscribers(nextState)
		ts.mu.Unlock()
		return true
	}
	ts.mu.Unlock()

	// wait for the state to change to a safe state
	ts.WaitFor(StateReady, StateInactive, StateShuttingDown)
	return ts.RequestSafeStateChange(nextState)
}

// markAsWaiting hints that the thread reached a stable state and is waiting for requests or shutdown
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

// isWaitingState returns true if a thread is waiting for a request or shutdown
func (ts *ThreadState) IsInWaitingState() bool {
	ts.mu.RLock()
	isWaiting := ts.isWaiting
	ts.mu.RUnlock()
	return isWaiting
}

// waitTime returns the time since the thread is waiting in a stable state in ms
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
