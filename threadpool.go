package frankenphp

import (
	"sync"
	"time"
)

type threadPool struct {
	threads  []*phpThread
	mu       sync.RWMutex
	ch       chan *frankenPHPContext
	fastChan chan *frankenPHPContext
}

func newThreadPool(capacity int) *threadPool {
	return &threadPool{
		threads:  make([]*phpThread, 0, capacity),
		mu:       sync.RWMutex{},
		ch:       make(chan *frankenPHPContext),
		fastChan: make(chan *frankenPHPContext),
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

func (p *threadPool) len() int {
	p.mu.RLock()
	l := len(p.threads)
	p.mu.RUnlock()
	return l
}

// get the correct request chan for queued requests
func (p *threadPool) requestChan(thread *phpThread) chan *frankenPHPContext {
	if thread.isLowLatencyThread {
		return p.fastChan
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
func (p *threadPool) queueRequest(fc *frankenPHPContext, isFastRequest bool) bool {
	var fastChan chan *frankenPHPContext
	if isFastRequest {
		fastChan = p.fastChan
	}

	for {
		select {
		case p.ch <- fc:
			return true
		case fastChan <- fc:
			return true
		case scaleChan <- fc:
			// the request has triggered scaling, continue to wait for a thread
		case <-timeoutChan(maxWaitTime):
			// the request has timed out stalling
			fc.reject(504, "Gateway Timeout")
			return false
		}
	}
}

func timeoutChan(timeout time.Duration) <-chan time.Time {
	if timeout == 0 {
		return nil
	}

	return time.After(timeout)
}
