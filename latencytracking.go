package frankenphp

import (
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// hard limit of tracked paths
const maxTrackedPaths = 1000

// path parts longer than this are considered a wildcard
const maxPathPartChars = 50

// max amount of requests being drained when a new slow path is recorded
const maxRequestDrainage = 100

var (
	// requests taking longer than this are considered slow (var for tests)
	slowRequestThreshold = 2000 * time.Millisecond
	// % of autoscaled threads that are  marked as low latency threads(var for tests)
	lowLatencyPercentile = 20

	slowRequestPaths       map[string]time.Duration
	latencyTrackingEnabled = false
	latencyTrackingActive  = atomic.Bool{}
	slowRequestsMu         = sync.RWMutex{}
	numRe                  = regexp.MustCompile(`^\d+$`)
	uuidRe                 = regexp.MustCompile(`^[a-f0-9-]{36}$`)
)

func initLatencyTracking(enabled bool) {
	latencyTrackingActive.Store(false)
	slowRequestPaths = make(map[string]time.Duration)
	latencyTrackingEnabled = enabled
}

func triggerLatencyTracking(thread *phpThread, threadAmount int, threadLimit int) {
	if !latencyTrackingEnabled || !isCloseToThreadLimit(threadAmount, threadLimit) {
		return
	}

	thread.isLowLatencyThread = true

	if !latencyTrackingActive.Load() {
		latencyTrackingActive.Store(true)
		logger.Info("latency tracking enabled")
	}
}

func stopLatencyTracking(threadAmount int, threadLimit int) {
	if latencyTrackingActive.Load() && !isCloseToThreadLimit(threadAmount, threadLimit) {
		latencyTrackingActive.Store(false)
		logger.Info("latency tracking disabled")
	}
}

func isCloseToThreadLimit(threadAmount int, threadLimit int) bool {
	return threadAmount >= threadLimit*(100-lowLatencyPercentile)/100
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

	recordedLatency := slowRequestPaths[normalizedPath]
	if recordedLatency == 0 && latencyTrackingActive.Load() {
		// a new path that is known to be slow is recorded,
		// drain some requests to free up low-latency threads
		// TODO: make sure this overhead is acceptable
	out:
		for i := 0; i < maxRequestDrainage; i++ {
			select {
			case scaleChan <- fc:
				_ = isHighLatencyRequest(fc)
			default:
				// no more queued requests
				//break outer loop
				break out
			}
		}
	}

	movingAverage := duration/2 + recordedLatency/2
	slowRequestPaths[normalizedPath] = movingAverage

	// remove the path if it is no longer considered slow
	if forceTracking && movingAverage < slowRequestThreshold {
		delete(slowRequestPaths, normalizedPath)
	}

	slowRequestsMu.Unlock()
}

// determine if a request is likely to be high latency based on previous requests with the same path
func isHighLatencyRequest(fc *frankenPHPContext) bool {
	if len(slowRequestPaths) == 0 {
		return false
	}

	normalizedPath := normalizePath(fc.getOriginalRequest().URL.Path)

	slowRequestsMu.RLock()
	latency := slowRequestPaths[normalizedPath]
	slowRequestsMu.RUnlock()

	fc.isLowLatencyRequest = latency < slowRequestThreshold

	return !fc.isLowLatencyRequest
}

// normalize a path by replacing variable parts with wildcards
// e.g. /user/123/profile -> /user/:id/profile
//
//	/post/550e8400-e29b-41d4-a716-446655440000 -> /post/:uuid
//	/category/very-long-category-name -> /category/:slug
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
	if len(part) > maxPathPartChars {
		// TODO: better slug detection?
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
