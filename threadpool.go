package frankenphp

import (
	"sync"
	"time"
)

// threadPool manages a pool of PHP threads
// used for both worker and regular threads
type threadPool struct {
	threads        []*phpThread
	mu             sync.RWMutex
	ch             chan *frankenPHPContext
	lowLatencyChan chan *frankenPHPContext
}

func newThreadPool(capacity int) *threadPool {
	return &threadPool{
		threads:        make([]*phpThread, 0, capacity),
		mu:             sync.RWMutex{},
		ch:             make(chan *frankenPHPContext),
		lowLatencyChan: make(chan *frankenPHPContext),
	}
}

func (p *threadPool) attach(thread *phpThread) {
	p.mu.Lock()
	p.threads = append(p.threads, thread)
	p.mu.Unlock()
}

func (p *threadPool) detach(thread *phpThread) {
	p.mu.Lock()
	for i := len(p.threads) - 1; i >= 0; i-- {
		if thread == p.threads[i] {
			p.threads = append(p.threads[:i], p.threads[i+1:]...)
			break
		}
	}
	p.mu.Unlock()
}

// get the correct request chan for queued requests
func (p *threadPool) requestChan(thread *phpThread) chan *frankenPHPContext {
	if thread.isLowLatencyThread {
		return p.lowLatencyChan
	}
	return p.ch
}

// dispatch to all threads in order if any is available
// dispatching in order minimizes memory usage and external connections
func (p *threadPool) dispatchRequest(fc *frankenPHPContext) bool {
	p.mu.RLock()
	for _, thread := range p.threads {
		select {
		case thread.requestChan <- fc:
			p.mu.RUnlock()
			return true
		default:
			// thread is busy, continue
		}
	}
	p.mu.RUnlock()

	return false
}

// dispatch request to all threads, triggering scaling or timeouts as needed
func (p *threadPool) queueRequest(fc *frankenPHPContext, isLowLatencyRequest bool) bool {
	var lowLatencyChan chan *frankenPHPContext
	if isLowLatencyRequest {
		lowLatencyChan = p.lowLatencyChan
	}

	var timeoutChan <-chan time.Time
	if maxWaitTime > 0 {
		timeoutChan = time.After(maxWaitTime)
	}

	for {
		select {
		case p.ch <- fc:
			return true
		case lowLatencyChan <- fc:
			return true // 'low laten'
		case scaleChan <- fc:
			// the request has triggered scaling, continue to wait for a thread
		case <-timeoutChan:
			// the request has timed out stalling
			fc.reject(504, "Gateway Timeout")
			return false
		}
	}
}
