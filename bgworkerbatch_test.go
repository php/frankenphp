package frankenphp_test

import (
	"os"
	"testing"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEnsureBackgroundWorkerBatch ensures multiple workers in one call,
// each publishing its own identity. Verifies the batch path (array arg)
// shares one deadline across all workers.
func TestEnsureBackgroundWorkerBatch(t *testing.T) {
	testDataDir := setupFrankenPHP(t,
		frankenphp.WithWorkers("worker-a", "testdata/bgworker/named.php", 0,
			frankenphp.WithWorkerBackground()),
		frankenphp.WithWorkers("worker-b", "testdata/bgworker/named.php", 0,
			frankenphp.WithWorkerBackground()),
		frankenphp.WithWorkers("worker-c", "testdata/bgworker/named.php", 0,
			frankenphp.WithWorkerBackground()),
		frankenphp.WithNumThreads(6),
	)

	body := serveBody(t, testDataDir, "bgworker/batch-ensure.php")
	assert.NotContains(t, body, "MISSING", "batch ensure should have started and published all workers:\n"+body)
	assert.Contains(t, body, "worker-a=worker-a")
	assert.Contains(t, body, "worker-b=worker-b")
	assert.Contains(t, body, "worker-c=worker-c")
}

// TestEnsureBackgroundWorkerBatchEmpty verifies that an empty array is
// rejected with a clear error rather than silently succeeding.
func TestEnsureBackgroundWorkerBatchEmpty(t *testing.T) {
	testDataDir := setupFrankenPHP(t,
		frankenphp.WithWorkers("bg", "testdata/bgworker/named.php", 0,
			frankenphp.WithWorkerBackground()),
		frankenphp.WithNumThreads(3),
	)

	php := `<?php
try {
    frankenphp_ensure_background_worker([], 1.0);
    echo "FAIL no error";
} catch (ValueError $e) {
    echo "OK ", $e->getMessage();
}`
	tmp := testDataDir + "bg-batch-empty.php"
	require.NoError(t, os.WriteFile(tmp, []byte(php), 0644))
	t.Cleanup(func() { _ = os.Remove(tmp) })

	body := serveBody(t, testDataDir, "bg-batch-empty.php")
	assert.Contains(t, body, "OK ")
	assert.Contains(t, body, "must not be empty")
	assert.NotContains(t, body, "FAIL")
}

// TestEnsureBackgroundWorkerBatchNonString verifies array-entry type
// validation: non-string elements produce a TypeError.
func TestEnsureBackgroundWorkerBatchNonString(t *testing.T) {
	testDataDir := setupFrankenPHP(t,
		frankenphp.WithWorkers("bg", "testdata/bgworker/named.php", 0,
			frankenphp.WithWorkerBackground()),
		frankenphp.WithNumThreads(3),
	)

	php := `<?php
try {
    frankenphp_ensure_background_worker(['bg', 42], 1.0);
    echo "FAIL no error";
} catch (TypeError $e) {
    echo "OK ", $e->getMessage();
}`
	tmp := testDataDir + "bg-batch-nonstring.php"
	require.NoError(t, os.WriteFile(tmp, []byte(php), 0644))
	t.Cleanup(func() { _ = os.Remove(tmp) })

	body := serveBody(t, testDataDir, "bg-batch-nonstring.php")
	assert.Contains(t, body, "OK ")
	assert.Contains(t, body, "must contain only strings")
	assert.NotContains(t, body, "FAIL")
}

// TestBackgroundWorkerServerFlag confirms that a bg worker sees
// FRANKENPHP_WORKER_BACKGROUND=true alongside FRANKENPHP_WORKER in
// $_SERVER, so scripts can branch without checking every function
// independently.
func TestBackgroundWorkerServerFlag(t *testing.T) {
	testDataDir := setupFrankenPHP(t,
		frankenphp.WithWorkers("flag-worker", "testdata/bgworker/bg-flag.php", 1,
			frankenphp.WithWorkerBackground()),
		frankenphp.WithNumThreads(3),
	)

	// ensure() removes the race between Init returning and the eager
	// bg-worker thread reaching its first set_vars.
	php := `<?php
frankenphp_ensure_background_worker('flag-worker');
$vars = frankenphp_get_vars('flag-worker');
echo 'name=', $vars['name'] ?? 'MISSING', "\n";
echo 'is_background=', var_export($vars['is_background'] ?? 'MISSING', true), "\n";
`
	tmp := testDataDir + "bg-flag-reader.php"
	require.NoError(t, os.WriteFile(tmp, []byte(php), 0644))
	t.Cleanup(func() { _ = os.Remove(tmp) })

	body := serveBody(t, testDataDir, "bg-flag-reader.php")
	assert.Contains(t, body, "name=flag-worker")
	assert.Contains(t, body, "is_background=true")
}
