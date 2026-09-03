package frankenphp

import (
	"bytes"
	"net/http/httptest"
	"runtime/pprof"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// leakedGoroutines runs a leak-detection GC cycle and returns how many goroutines
// it found blocked forever on a channel or mutex that no running goroutine can
// still reach, along with their stacks.
//
// Profile.Count only reports the result of the previous detection cycle, so the
// profile has to be written first even when only the count is wanted.
func leakedGoroutines(t *testing.T) (int, string) {
	t.Helper()

	profile := pprof.Lookup("goroutineleak")
	require.NotNil(t, profile, "the goroutineleak profile is only available since Go 1.27")

	var stacks bytes.Buffer
	require.NoError(t, profile.WriteTo(&stacks, 1))

	return profile.Count(), stacks.String()
}

// TestNoGoroutinesAreLeakedByAFullServerLifecycle boots PHP threads, serves requests
// through a worker and a regular thread, then shuts everything down.
//
// Shutdown has to unblock every goroutine it started, so a goroutine still parked on
// an unreachable channel afterwards means a thread, a scaling ticker or a watcher
// outlived Shutdown with nothing left to wake it. The count is compared against a
// baseline rather than against zero: tests share a process, so earlier tests may have
// left leaks of their own behind.
func TestNoGoroutinesAreLeakedByAFullServerLifecycle(t *testing.T) {
	before, _ := leakedGoroutines(t)

	require.NoError(t, Init(
		WithNumThreads(2),
		WithMaxThreads(4),
		WithWorkers("worker", testDataPath+"/index.php", 1, WithWorkerMaxFailures(0)),
	))

	for range 5 {
		r := httptest.NewRequest("GET", "http://localhost/index.php", nil)
		req, err := NewRequestWithContext(r, WithRequestDocumentRoot(testDataPath, false))
		require.NoError(t, err)
		require.NoError(t, ServeHTTP(httptest.NewRecorder(), req))
	}

	Shutdown()

	after, stacks := leakedGoroutines(t)
	t.Logf("leaked goroutines: %d before, %d after", before, after)

	assert.LessOrEqual(t, after, before, "goroutines leaked across an Init/Shutdown cycle:\n%s", stacks)
}
