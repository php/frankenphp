package frankenphp

import (
	"sync"
	"time"
)

// stallSampleInterval is how often a worker's request queue is sampled
// to compute the rolling stall metric. 100ms gives 10 samples/s, which
// is enough resolution for the 1, 3, and 5 second windows while keeping
// overhead negligible.
const stallSampleInterval = 100 * time.Millisecond

// Window sizes are expressed in number of samples at stallSampleInterval.
const (
	stallWindow1 = int(time.Second / stallSampleInterval)
	stallWindow3 = int(3 * time.Second / stallSampleInterval)
	stallWindow5 = int(5 * time.Second / stallSampleInterval)
)

// stallSampler tracks whether a worker's request queue was non-empty
// across a fixed-size sliding window. Samples are recorded by the
// per-worker sampler goroutine and read out as rolling-window means
// covering 1, 3, and 5 second windows. Each mean is a fraction in
// [0,1] representing the share of the window during which the queue
// was non-empty — i.e. requests were stalling waiting for a worker
// thread.
//
// The sampler is intentionally decoupled from the request hot path:
// it observes worker.queuedRequests passively from a dedicated
// goroutine and never blocks request handling.
type stallSampler struct {
	mu      sync.Mutex
	samples [stallWindow5]uint8
	cursor  int // index of next write
	filled  int // number of valid samples (capped at stallWindow5)
}

// record appends a single sample to the ring buffer.
func (s *stallSampler) record(stalled bool) {
	var v uint8
	if stalled {
		v = 1
	}

	s.mu.Lock()
	s.samples[s.cursor] = v
	s.cursor = (s.cursor + 1) % stallWindow5
	if s.filled < stallWindow5 {
		s.filled++
	}
	s.mu.Unlock()
}

// snapshot returns rolling-window stall fractions for the 1, 3, and 5
// second windows. Each value is in [0,1]; 0 means the queue was empty
// for the whole window, 1 means the queue was non-empty for the whole
// window. When fewer than `windowSize` samples have been collected,
// the available samples are averaged (so the metric ramps up over the
// first 5 seconds rather than reporting a misleading low value).
func (s *stallSampler) snapshot() (win1s, win3s, win5s float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.windowLocked(stallWindow1),
		s.windowLocked(stallWindow3),
		s.windowLocked(stallWindow5)
}

// windowLocked computes the mean of the most recent `size` samples.
// The caller must hold s.mu.
func (s *stallSampler) windowLocked(size int) float64 {
	if s.filled == 0 {
		return 0
	}

	n := size
	if n > s.filled {
		n = s.filled
	}

	sum := 0
	for i := 1; i <= n; i++ {
		idx := (s.cursor - i + stallWindow5) % stallWindow5
		sum += int(s.samples[idx])
	}

	return float64(sum) / float64(n)
}
