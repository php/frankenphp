//go:build !nowatcher

package frankenphp_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// we have to wait a few milliseconds for the watcher debounce to take effect
const pollingTime = 250

// in tests checking for no reload: we will poll 3x250ms = 0.75s
const minTimesToPollForChanges = 3

// in tests checking for a reload: we will poll a maximum of 60x250ms = 15s
const maxTimesToPollForChanges = 60

func TestWorkersShouldReloadOnMatchingPattern(t *testing.T) {
	watch := []string{"./testdata/**/*.txt"}

	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		requestBodyHasReset := pollForWorkerReset(t, handler, maxTimesToPollForChanges)
		assert.True(t, requestBodyHasReset)
	}, &testOptions{nbParallelRequests: 1, nbWorkers: 1, workerScript: "worker-with-counter.php", watch: watch})
}

func TestWorkersShouldNotReloadOnExcludingPattern(t *testing.T) {
	watch := []string{"./testdata/**/*.php"}

	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		requestBodyHasReset := pollForWorkerReset(t, handler, minTimesToPollForChanges)
		assert.False(t, requestBodyHasReset)
	}, &testOptions{nbParallelRequests: 1, nbWorkers: 1, workerScript: "worker-with-counter.php", watch: watch})
}

func pollForWorkerReset(t *testing.T, handler func(http.ResponseWriter, *http.Request), limit int) bool {
	t.Helper()

	// first we make an initial request to start the request counter
	body, _ := testGet("http://example.com/worker-with-counter.php", handler, t)
	assert.Equal(t, "requests:1", body)

	// now we spam file updates and check if the request counter resets
	for range limit {
		updateTestFile(t, filepath.Join(".", "testdata", "files", "test.txt"), "updated")
		time.Sleep(pollingTime * time.Millisecond)
		body, _ := testGet("http://example.com/worker-with-counter.php", handler, t)
		if body == "requests:1" {
			return true
		}
	}

	return false
}

func updateTestFile(t *testing.T, fileName, content string) {
	absFileName, err := filepath.Abs(fileName)
	require.NoError(t, err)

	require.NoError(t, os.MkdirAll(filepath.Dir(absFileName), 0700))
	require.NoError(t, os.WriteFile(absFileName, []byte(content), 0644))
}
