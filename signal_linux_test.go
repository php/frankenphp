//go:build linux

package frankenphp_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A SIGURG landing in PHP would interrupt the system call the thread sleeps in
func TestPreemptionSignalIsBlocked_module(t *testing.T) { testPreemptionSignalIsBlocked(t, nil) }
func TestPreemptionSignalIsBlocked_worker(t *testing.T) {
	testPreemptionSignalIsBlocked(t, &testOptions{workerScript: "signal-mask.php"})
}
func testPreemptionSignalIsBlocked(t *testing.T, opts *testOptions) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, _ int) {
		body, _ := testGet("http://example.com/signal-mask.php", handler, t)

		mask, err := strconv.ParseUint(body, 16, 64)
		require.NoError(t, err)
		assert.NotZero(t, mask&(1<<(syscall.SIGURG-1)), "SIGURG is not blocked")
	}, opts)
}
