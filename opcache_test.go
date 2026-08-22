package frankenphp_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOpcacheRestartKeepsWorkerThreadsAlive guards the opcache restart hook.
//
// A worker holds references into opcache shared memory for its whole life. An
// opcache restart rewinds that memory to a startup watermark while the worker
// is still executing, because opcache defers a restart until
// accel_is_inactive(), which is an fcntl probe that cannot see threads of the
// calling process. The hook reboots every thread before that happens.
//
// Without the hook this test does not fail, it takes the process down with
// SIGSEGV, which is the regression being guarded.
func TestOpcacheRestartKeepsWorkerThreadsAlive(t *testing.T) {
	t.Cleanup(func() {
		_ = os.RemoveAll(filepath.Join(os.TempDir(), "frankenphp-opcache-restart-test"))
	})

	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, _ int) {
		get := func(path string) string {
			w := httptest.NewRecorder()
			handler(w, httptest.NewRequest("GET", "http://example.com"+path, nil))

			return strings.TrimSpace(w.Body.String())
		}

		if body := get("/opcache/trigger.php?b=0"); body == "NOOPCACHE" {
			t.Skip("opcache is not available in this build")
		}

		require.Equal(t, "OK", get("/opcache/worker.php"), "worker must be healthy before the restart")

		// Compile until opcache performs a restart. The counters only move
		// once a restart has actually been carried out, not merely scheduled.
		restarted := false
		for b := 1; b <= 20 && !restarted; b++ {
			body := get("/opcache/trigger.php?b=" + strconv.Itoa(b))
			if n, err := strconv.Atoi(body); err == nil && n > 0 {
				restarted = true
			}
		}

		if !restarted {
			t.Skip("could not force an opcache restart in this environment")
		}

		assert.Equal(t, "OK", get("/opcache/worker.php"), "worker must survive an opcache restart")
	}, &testOptions{
		workerScript:       "opcache/worker.php",
		nbWorkers:          1,
		nbParallelRequests: 1,
		phpIni: map[string]string{
			"opcache.enable":                  "1",
			"opcache.enable_cli":              "1",
			"opcache.memory_consumption":      "8",
			"opcache.interned_strings_buffer": "1",
			"opcache.max_accelerated_files":   "200",
			"opcache.file_update_protection":  "0",
			"opcache.validate_timestamps":     "1",
			"opcache.revalidate_freq":         "0",
		},
	})
}
