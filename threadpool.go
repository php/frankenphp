package frankenphp

import (
	"math/rand/v2"
	"sync"
)

// TODO: dynamic splitting?
type threadPool struct {
	threads     []*phpThread
	slowThreads []*phpThread
	mu          sync.RWMutex
	ch          chan *frankenPHPContext
}

func newThreadPool(capacity int) *threadPool {
	return &threadPool{
		threads: make([]*phpThread, 0, capacity),
		mu:      sync.RWMutex{},
		ch:      make(chan *frankenPHPContext),
	}
}

func (p *threadPool) attach(thread *phpThread) {
	p.mu.Lock()
	p.threads = append(p.threads, thread)
	if !thread.isLowLatencyThread {
		p.slowThreads = append(p.slowThreads, thread)
	}
	p.mu.Unlock()
}

func (p *threadPool) detach(thread *phpThread) {
	p.mu.Lock()
	for i, t := range p.threads {
		if t == thread {
			p.threads = append(p.threads[:i], p.threads[i+1:]...)
			break
		}
	}
	if !thread.isLowLatencyThread {
		for i, t := range p.slowThreads {
			if t == thread {
				p.slowThreads = append(p.slowThreads[:i], p.slowThreads[i+1:]...)
				break
			}
		}
	}
	p.mu.Unlock()
}

func (p *threadPool) getRandomSlowThread() *phpThread {
	p.mu.RLock()
	thread := p.slowThreads[rand.IntN(len(p.slowThreads))]
	p.mu.RUnlock()

	return thread
}

func (p *threadPool) len() int {
	p.mu.RLock()
	l := len(p.threads)
	p.mu.RUnlock()
	return l
}
