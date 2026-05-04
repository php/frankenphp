package frankenphp_test

import (
	"os"
	"testing"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBackgroundWorkerPool declares a named bg worker with num=3 (pool
// of three threads). All three threads should boot, share the same
// registered backgroundWorkerState, and the reader can see the pool's
// vars. This covers the lifted num>1 + max_threads>1 constraint.
func TestBackgroundWorkerPool(t *testing.T) {
	testDataDir := setupFrankenPHP(t,
		frankenphp.WithWorkers("pool-worker", "testdata/bgworker/pool.php", 3,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerMaxThreads(3)),
		frankenphp.WithNumThreads(6),
	)

	// Read the pool worker's vars via get_vars; all three threads share
	// the same state so we don't need to target a specific one. ensure()
	// waits for at least one pool thread's first set_vars so the eager
	// start can't race the reader.
	php := `<?php
frankenphp_ensure_background_worker('pool-worker');
$vars = frankenphp_get_vars('pool-worker');
echo 'name=', $vars['name'] ?? 'MISSING', "\n";
echo 'has-pid=', isset($vars['pid']) ? '1' : '0', "\n";
`
	tmp := testDataDir + "pool-reader.php"
	require.NoError(t, os.WriteFile(tmp, []byte(php), 0644))
	t.Cleanup(func() { _ = os.Remove(tmp) })

	body := serveBody(t, testDataDir, "pool-reader.php")
	assert.NotContains(t, body, "MISSING", "pool worker should have published its vars:\n"+body)
	assert.Contains(t, body, "name=pool-worker")
	assert.Contains(t, body, "has-pid=1")
}

// TestBackgroundWorkerMultiEntrypoint declares two named bg workers that
// share the same entrypoint file. Each registry is independent, so ensure()
// + get_vars resolve to the correct instance by name.
func TestBackgroundWorkerMultiEntrypoint(t *testing.T) {
	testDataDir := setupFrankenPHP(t,
		frankenphp.WithWorkers("shared-a", "testdata/bgworker/named.php", 1,
			frankenphp.WithWorkerBackground()),
		frankenphp.WithWorkers("shared-b", "testdata/bgworker/named.php", 1,
			frankenphp.WithWorkerBackground()),
		frankenphp.WithNumThreads(5),
	)

	read := func(name string) string {
		php := `<?php
frankenphp_ensure_background_worker(` + "'" + name + "'" + `);
$vars = frankenphp_get_vars(` + "'" + name + "'" + `);
echo $vars['FRANKENPHP_WORKER'] ?? 'MISSING';
`
		script := "multi-entry-reader-" + name + ".php"
		require.NoError(t, os.WriteFile(testDataDir+script, []byte(php), 0644))
		t.Cleanup(func() { _ = os.Remove(testDataDir + script) })
		return serveBody(t, testDataDir, script)
	}

	assert.Equal(t, "shared-a", read("shared-a"), "shared-a name must resolve to its own worker")
	assert.Equal(t, "shared-b", read("shared-b"), "shared-b name must resolve to its own worker")
}
