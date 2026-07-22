package frankenphp_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// requireFileEventually asserts that `path` appears on disk before the
// deadline. Wraps require.Eventually so call sites stay short.
func requireFileEventually(t testing.TB, path string, msgAndArgs ...any) {
	t.Helper()
	require.Eventually(t, func() bool {
		_, err := os.Stat(path)
		return err == nil
	}, 5*time.Second, 25*time.Millisecond, msgAndArgs...)
}

// TestBackgroundWorkerLifecycle boots a background worker that touches a
// sentinel file then parks on the stop pipe. It proves the bg worker runs
// (sentinel appears) and that Shutdown returns within a reasonable time.
// The test asserts on Shutdown timing, so it manages Shutdown itself
// instead of using initServers' t.Cleanup hook.
func TestBackgroundWorkerLifecycle(t *testing.T) {
	tmp := t.TempDir()
	sentinel := filepath.Join(tmp, "bg-lifecycle.sentinel")

	require.NoError(t, frankenphp.Init(
		frankenphp.WithWorkers("bg-lifecycle", "testdata/bgworker/basic.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerEnv(map[string]string{"BG_SENTINEL": sentinel}),
		),
		frankenphp.WithNumThreads(2),
	))

	requireFileEventually(t, sentinel, "background worker did not touch sentinel")

	done := make(chan struct{})
	go func() {
		frankenphp.Shutdown()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatalf("Shutdown did not return within 10s")
	}
}

// TestBackgroundWorkerCrashRestarts boots a worker that exit(1)s on its
// first run and touches a "restarted" sentinel on its second run. The
// sentinel proves the crash-restart loop fired.
func TestBackgroundWorkerCrashRestarts(t *testing.T) {
	tmp := t.TempDir()
	crashMarker := filepath.Join(tmp, "bg-crash.marker")
	restarted := filepath.Join(tmp, "bg-crash.restarted")

	initServers(t,
		frankenphp.WithWorkers("bg-crash", "testdata/bgworker/crash.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerEnv(map[string]string{
				"BG_CRASH_MARKER":       crashMarker,
				"BG_RESTARTED_SENTINEL": restarted,
			}),
		),
		frankenphp.WithNumThreads(2),
	)

	requireFileEventually(t, restarted, "background worker did not restart after crash")
}

// TestBackgroundWorkerOnServer scopes a background worker to a Server. It
// proves that the worker inherits the server env (the sentinel directory is
// declared on the server, not on the worker), that FRANKENPHP_WORKER holds
// the worker name, and that the worker does not intercept HTTP requests
// served by the same server.
func TestBackgroundWorkerOnServer(t *testing.T) {
	tmp := t.TempDir()

	server, err := frankenphp.NewServer("sidekick-server", testDataDir, nil, map[string]string{"BG_SENTINEL_DIR": tmp}, nil)
	require.NoError(t, err)

	initServers(t,
		frankenphp.WithServer(server),
		frankenphp.WithWorkers("jobs", "testdata/bgworker/named.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerServerScope(server),
		),
		frankenphp.WithNumThreads(2),
	)

	// named.php touches "<BG_SENTINEL_DIR>/<FRANKENPHP_WORKER>"
	requireFileEventually(t, filepath.Join(tmp, "jobs"), "background worker did not touch its per-name sentinel")

	body := serverGet(t, server, "http://example.com/index.php")
	assert.Contains(t, body, "I am by birth a Genevese", "the server must still serve regular requests")
}

// TestBackgroundWorkerValidation covers the declaration-time errors.
func TestBackgroundWorkerValidation(t *testing.T) {
	t.Cleanup(frankenphp.Shutdown)

	t.Run("name is required", func(t *testing.T) {
		err := frankenphp.Init(
			frankenphp.WithWorkers("", "testdata/bgworker/basic.php", 1, frankenphp.WithWorkerBackground()),
			frankenphp.WithNumThreads(2),
		)
		require.ErrorContains(t, err, "must have an explicit name")
	})

	t.Run("num must be >= 1", func(t *testing.T) {
		err := frankenphp.Init(
			frankenphp.WithWorkers("bg-zero", "testdata/bgworker/basic.php", 0, frankenphp.WithWorkerBackground()),
			frankenphp.WithNumThreads(2),
		)
		require.ErrorContains(t, err, "must declare num >= 1")
	})

	t.Run("names are global", func(t *testing.T) {
		err := frankenphp.Init(
			frankenphp.WithWorkers("bg-shared", "testdata/bgworker/basic.php", 1, frankenphp.WithWorkerBackground()),
			frankenphp.WithWorkers("bg-shared", "testdata/bgworker/named.php", 1, frankenphp.WithWorkerBackground()),
			frankenphp.WithNumThreads(3),
		)
		require.ErrorContains(t, err, "two workers cannot have the same name")
	})

	t.Run("request matchers are rejected", func(t *testing.T) {
		err := frankenphp.Init(
			frankenphp.WithWorkers("bg-matched", "testdata/bgworker/basic.php", 1,
				frankenphp.WithWorkerBackground(),
				frankenphp.WithWorkerMatcher(func(*http.Request) bool { return true }),
			),
			frankenphp.WithNumThreads(2),
		)
		require.ErrorContains(t, err, "cannot match requests")
	})
}
