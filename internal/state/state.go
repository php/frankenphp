package state

import "C"
import (
	"slices"
	"sync"
	"time"
)

type StateID uint8

const (
	// livecycle States of a thread
	Reserved StateID = iota
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

var stateNames = map[StateID]string{
	Reserved:             "reserved",
	Booting:              "booting",
	Inactive:             "inactive",
	Ready:                "ready",
	ShuttingDown:         "shutting down",
	Done:                 "done",
	Restarting:           "restarting",
	Yielding:             "yielding",
	TransitionRequested:  "transition requested",
	TransitionInProgress: "transition in progress",
	TransitionComplete:   "transition complete",
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
		currentState: Reserved,
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
