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
const childLimitPerNode = 10

// path parts longer than this are considered a wildcard
const charLimitWildcard = 50

var (
	// requests taking longer than this are considered slow (var for tests)
	slowRequestThreshold = 1000 * time.Millisecond
	// % of autoscaled threads that are not marked as low latency (var for tests)
	slowThreadPercentile = 80

	rootNode              = &pathNode{children: make(map[string]*pathNode), mu: sync.RWMutex{}}
	enableLatencyTracking = atomic.Bool{}
	numRe                 = regexp.MustCompile(`^\d+$`)
	uuidRe                = regexp.MustCompile(`^[a-f0-9-]{36}$`)
)

type pathNode struct {
	children map[string]*pathNode
	latency  time.Duration
	mu       sync.RWMutex
}

func (n *pathNode) commitLatency(time time.Duration) {
	n.mu.Lock()
	n.latency = n.latency/2 + time/2
	n.mu.Unlock()
}

func (n *pathNode) getLatency() time.Duration {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.latency
}

func (n *pathNode) addChild(child *pathNode, path string) {
	n.mu.Lock()
	if len(n.children) <= childLimitPerNode {
		n.children[path] = child
	}
	n.mu.Unlock()
}

func (n *pathNode) getChild(path string) (*pathNode, bool) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	node, ok := n.children[path]
	return node, ok
}

func initLatencyTracking() {
	enableLatencyTracking.Store(false)
}

// get a random thread that is not marked as low latency
func getRandomSlowThread(worker *worker) *phpThread {
	worker.threadMutex.RLock()
	defer worker.threadMutex.RUnlock()

	slowThreads := []*phpThread{}
	for _, thread := range worker.threads {
		if !thread.isLowLatencyThread {
			slowThreads = append(slowThreads, thread)
		}
	}

	// if there are no slow threads, return a random thread
	if len(slowThreads) == 0 {
		return worker.threads[rand.Intn(len(worker.threads))]
	}

	return slowThreads[rand.Intn(len(slowThreads))]
}

func recordSlowRequest(fc *frankenPHPContext, duration time.Duration) {
	if duration > slowRequestThreshold {
		recordRequestLatency(fc, duration)
	}
}

// record a slow request in the path tree
func recordRequestLatency(fc *frankenPHPContext, duration time.Duration) {
	request := fc.getOriginalRequest()
	parts := strings.Split(request.URL.Path, "/")
	node := rootNode

	logger.Debug("slow request detected", "path", request.URL.Path, "duration", duration)

	for _, part := range parts {
		if part == "" {
			continue
		}
		if isWildcardPathPart(part) {
			part = "*"
		}

		childNode, exists := node.getChild(part)
		if !exists {
			childNode = &pathNode{
				children: make(map[string]*pathNode),
				mu:       sync.RWMutex{},
			}
			node.addChild(childNode, part)
		}
		node = childNode
		node.commitLatency(duration)
	}
}

// determine if a request is likely to be high latency based on the request path
func isHighLatencyRequest(fc *frankenPHPContext) bool {
	request := fc.getOriginalRequest()
	parts := strings.Split(request.URL.Path, "/")
	node := rootNode

	for _, part := range parts {
		if part == "" {
			continue
		}

		childNode, exists := node.getChild(part)
		if exists {
			node = childNode
			continue
		}
		childNode, exists = node.getChild("*")
		if exists {
			node = childNode
			continue
		}

		return false
	}

	return node.getLatency() > slowRequestThreshold
}

// determine if a path part is a wildcard
// a path part is a wildcard if:
// - it is longer than charLimitWildcard
// - it is a number
// - it is a uuid
func isWildcardPathPart(part string) bool {
	return len(part) > charLimitWildcard || numRe.MatchString(part) || uuidRe.MatchString(part)
}
