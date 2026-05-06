package frankenphp_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEnsureBackgroundWorkerBatch declares a single catch-all bg worker
// and ensures three distinct names from a single ensure() call. Each
// catch-all instance touches a per-name sentinel; the test asserts that
// all three appear, proving the array form started one worker per name.
func TestEnsureBackgroundWorkerBatch(t *testing.T) {
	tmp := t.TempDir()
	testDataDir := setupFrankenPHP(t,
		frankenphp.WithWorkers("", "testdata/bgworker/named.php", 0,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerMaxThreads(8),
			frankenphp.WithWorkerEnv(map[string]string{"BG_SENTINEL_DIR": tmp}),
		),
		frankenphp.WithNumThreads(8),
	)

	body := serveBody(t, testDataDir, "bgworker/batch-ensure.php")
	assert.Contains(t, body, "ok", "batch ensure script should echo ok, got: %q", body)

	for _, name := range []string{"batch-a", "batch-b", "batch-c"} {
		requireFileEventually(t, filepath.Join(tmp, name),
			"catch-all instance %q should have written its sentinel", name)
	}
}

// TestEnsureBackgroundWorkerBatchEmpty exercises the C-side validation
// that an empty array raises a ValueError before any worker is started.
// The fixture catches the throwable and echoes its class.
func TestEnsureBackgroundWorkerBatchEmpty(t *testing.T) {
	testDataDir := setupFrankenPHP(t,
		frankenphp.WithWorkers("", "testdata/bgworker/named.php", 0,
			frankenphp.WithWorkerBackground(),
		),
		frankenphp.WithNumThreads(2),
	)

	body := serveBody(t, testDataDir, "bgworker/batch-errors.php?mode=empty")
	assert.Contains(t, body, "ValueError", "empty array should raise ValueError, got: %q", body)
	assert.Contains(t, body, "must not be empty")
}

// TestEnsureBackgroundWorkerBatchNonString verifies a non-string element
// raises a TypeError (PHP's standard for argument-type mismatches inside
// our parsed array).
func TestEnsureBackgroundWorkerBatchNonString(t *testing.T) {
	testDataDir := setupFrankenPHP(t,
		frankenphp.WithWorkers("", "testdata/bgworker/named.php", 0,
			frankenphp.WithWorkerBackground(),
		),
		frankenphp.WithNumThreads(2),
	)

	body := serveBody(t, testDataDir, "bgworker/batch-errors.php?mode=nonstring")
	assert.Contains(t, body, "TypeError", "non-string element should raise TypeError, got: %q", body)
}

// TestEnsureBackgroundWorkerBatchDuplicate verifies that duplicate names
// in the same batch are rejected as a ValueError, matching the e17577e
// reference behavior (no silent dedup).
func TestEnsureBackgroundWorkerBatchDuplicate(t *testing.T) {
	testDataDir := setupFrankenPHP(t,
		frankenphp.WithWorkers("", "testdata/bgworker/named.php", 0,
			frankenphp.WithWorkerBackground(),
		),
		frankenphp.WithNumThreads(2),
	)

	body := serveBody(t, testDataDir, "bgworker/batch-errors.php?mode=duplicate")
	assert.Contains(t, body, "ValueError", "duplicate name should raise ValueError, got: %q", body)
	assert.Contains(t, body, "duplicate")
}

// TestBackgroundWorkerBgFlag asserts that a bg worker script sees
// $_SERVER['FRANKENPHP_WORKER_BACKGROUND'] === true. The fixture writes
// var_export() of the value to a sentinel so the test can read the exact
// PHP-level representation.
func TestBackgroundWorkerBgFlag(t *testing.T) {
	tmp := t.TempDir()
	sentinel := filepath.Join(tmp, "bg-flag.sentinel")

	setupFrankenPHP(t,
		frankenphp.WithWorkers("bg-flag", "testdata/bgworker/bg-flag.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerEnv(map[string]string{"BG_SENTINEL": sentinel}),
		),
		frankenphp.WithNumThreads(2),
	)

	requireFileEventually(t, sentinel,
		"bg worker should have written the FRANKENPHP_WORKER_BACKGROUND sentinel")

	contents, err := os.ReadFile(sentinel)
	require.NoError(t, err)
	assert.Equal(t, "true", string(contents),
		"$_SERVER['FRANKENPHP_WORKER_BACKGROUND'] should be the bool true")
}
