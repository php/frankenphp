package frankenphp

import (
	"math/rand"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// limit of tracked path children
const maxTrackedPaths = 1000

// path parts longer than this are considered a wildcard
const charLimitWildcard = 50

var (
	// requests taking longer than this are considered slow (var for tests)
	slowRequestThreshold = 1000 * time.Millisecond
	// % of autoscaled threads that are not marked as low latency (var for tests)
	slowThreadPercentile = 80

	latencyTrackingEnabled = atomic.Bool{}
	slowRequestsMu         = sync.RWMutex{}
	slowRequestPaths       map[string]time.Duration
	numRe                  = regexp.MustCompile(`^\d+$`)
	uuidRe                 = regexp.MustCompile(`^[a-f0-9-]{36}$`)
)

func initLatencyTracking() {
	latencyTrackingEnabled.Store(false)
	slowRequestPaths = make(map[string]time.Duration)
}

// trigger latency tracking while scaling threads
func triggerLatencyTrackingIfNeeded(thread *phpThread) {
	if isNearThreadLimit() {
		latencyTrackingEnabled.Store(true)
		thread.isLowLatencyThread = true
		logger.Debug("low latency thread spawned")
	}
}

func stopLatencyTrackingIfNeeded() {
	if latencyTrackingEnabled.Load() && !isNearThreadLimit() {
		latencyTrackingEnabled.Store(false)
		logger.Debug("latency tracking disabled")
	}
}

func isNearThreadLimit() bool {
	return len(autoScaledThreads) >= cap(autoScaledThreads)*slowThreadPercentile/100
}

// get a random thread that is not marked as low latency
func getRandomSlowThread(threads []*phpThread) *phpThread {
	slowThreadCount := 0
	for _, thread := range threads {
		if !thread.isLowLatencyThread {
			slowThreadCount++
		}
	}

	if slowThreadCount == 0 {
		panic("there must always be at least one slow thread")
	}

	slowThreadNum := rand.Intn(slowThreadCount)
	for _, thread := range threads {
		if !thread.isLowLatencyThread {
			if slowThreadNum == 0 {
				return thread
			}
			slowThreadNum--
		}
	}

	panic("there must always be at least one slow thread")
}

// record a slow request path
func trackRequestLatency(fc *frankenPHPContext, duration time.Duration, forceTracking bool) {
	if duration < slowRequestThreshold && !forceTracking {
		return
	}

	request := fc.getOriginalRequest()
	normalizedPath := normalizePath(request.URL.Path)
	logger.Debug("slow request detected", "path", normalizedPath, "duration", duration)
	slowRequestsMu.Lock()

	// if too many slow paths are tracked, clear the map
	if len(slowRequestPaths) > maxTrackedPaths {
		slowRequestPaths = make(map[string]time.Duration)
	}

	// record the latency as a moving average
	recordedLatency := slowRequestPaths[normalizedPath]
	slowRequestPaths[normalizedPath] = duration/2 + recordedLatency/2
	slowRequestsMu.Unlock()
}

// determine if a request is likely to be high latency based on the request path
func isHighLatencyRequest(fc *frankenPHPContext) bool {
	request := fc.getOriginalRequest()
	normalizedPath := normalizePath(request.URL.Path)

	slowRequestsMu.RLock()
	latency, exists := slowRequestPaths[normalizedPath]
	slowRequestsMu.RUnlock()

	if exists {
		return latency > slowRequestThreshold
	}

	return false
}

// TODO: query?
func normalizePath(path string) string {
	pathLen := len(path)
	if pathLen > 1 && path[pathLen-1] == '/' {
		pathLen-- // ignore trailing slash for processing
	}

	var b strings.Builder
	b.Grow(len(path)) // pre-allocate at least original size
	start := 0
	for i := 0; i <= pathLen; i++ {
		if i == pathLen || path[i] == '/' {
			if i > start {
				seg := path[start:i]
				b.WriteString(normalizePathPart(seg))
			}
			if i < pathLen {
				b.WriteByte('/')
			}
			start = i + 1
		}
	}
	return b.String()
}

// determine if a path part is a wildcard
func normalizePathPart(part string) string {
	if len(part) > charLimitWildcard {
		return ":slug"
	}

	if numRe.MatchString(part) {
		return ":id"
	}

	if uuidRe.MatchString(part) {
		return ":uuid"
	}

	return part
}
