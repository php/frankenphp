package frankenphp_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/require"
)

// TestBackgroundWorkerPool declares a named bg worker with num=3 (pool of
// three threads). Each thread touches a unique sentinel under
// BG_SENTINEL_DIR via tempnam(), so the test can assert that all three
// pool threads booted independently. Covers the lifted num>1 +
// max_threads>1 constraints and the per-thread stop pipe.
func TestBackgroundWorkerPool(t *testing.T) {
	tmp := t.TempDir()
	setupFrankenPHP(t,
		frankenphp.WithWorkers("pool-worker", "testdata/bgworker/pool.php", 3,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerMaxThreads(3),
			frankenphp.WithWorkerEnv(map[string]string{"BG_SENTINEL_DIR": tmp}),
		),
		frankenphp.WithNumThreads(6),
	)

	require.Eventually(t, func() bool {
		entries, err := os.ReadDir(tmp)
		return err == nil && len(entries) == 3
	}, 5*time.Second, 25*time.Millisecond,
		"expected 3 distinct pool sentinels under %s", tmp)
}

// TestBackgroundWorkerMultiEntrypoint declares two named bg workers that
// share the same entrypoint file. Each gets its own *worker, so both Init
// successfully (no filename-collision rejection) and both produce
// sentinels.
func TestBackgroundWorkerMultiEntrypoint(t *testing.T) {
	tmp := t.TempDir()
	setupFrankenPHP(t,
		frankenphp.WithWorkers("shared-a", "testdata/bgworker/named.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerEnv(map[string]string{"BG_SENTINEL_DIR": tmp}),
		),
		frankenphp.WithWorkers("shared-b", "testdata/bgworker/named.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerEnv(map[string]string{"BG_SENTINEL_DIR": tmp}),
		),
		frankenphp.WithNumThreads(5),
	)

	for _, name := range []string{"shared-a", "shared-b"} {
		requireFileEventually(t, filepath.Join(tmp, name),
			"shared bg worker %q did not touch its sentinel", name)
	}
}
