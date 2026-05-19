package frankenphp_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEnsureBackgroundWorkerNamedLazy drives ensure() against a declared
// named worker with num=0 (lazy). First request lazy-starts it; set_vars
// publishes; get_vars reads the published vars.
func TestEnsureBackgroundWorkerNamedLazy(t *testing.T) {
	testDataDir := setupFrankenPHP(t,
		frankenphp.WithWorkers("bg-lazy", "testdata/bgworker/named.php", 0,
			frankenphp.WithWorkerBackground()),
		frankenphp.WithNumThreads(3),
	)

	body := serveBody(t, testDataDir, "bgworker/ensure-from-handler.php")
	assert.NotContains(t, body, "MISSING", "ensure() should have lazy-started the worker and published vars:\n"+body)
	assert.Contains(t, body, "ensured-name=bg-lazy")
}

// TestEnsureBackgroundWorkerCatchAll declares a single catch-all (no name)
// and invokes ensure() twice with distinct names. Each name should start
// its own instance from the same entrypoint and publish its own vars.
func TestEnsureBackgroundWorkerCatchAll(t *testing.T) {
	testDataDir := setupFrankenPHP(t,
		// Name-less bg worker = catch-all. max_threads on a catch-all is
		// the cap on lazy-started instances; it also drives the thread
		// budget that calculateMaxThreads reserves for the catch-all.
		frankenphp.WithWorkers("", "testdata/bgworker/named.php", 0,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerMaxThreads(4)),
		frankenphp.WithNumThreads(5),
	)

	for _, name := range []string{"job-a", "job-b"} {
		body := serveBody(t, testDataDir, "bgworker/ensure-reader.php?name="+name)
		assert.Contains(t, body, "name="+name, "catch-all instance %s did not publish its name:\n%s", name, body)
	}
}

// TestEnsureBackgroundWorkerCatchAllCap sets max_threads on a catch-all so
// the third distinct name ensure() hits the cap error.
func TestEnsureBackgroundWorkerCatchAllCap(t *testing.T) {
	testDataDir := setupFrankenPHP(t,
		frankenphp.WithWorkers("", "testdata/bgworker/named.php", 0,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerMaxThreads(2)),
		frankenphp.WithNumThreads(5),
	)

	for _, name := range []string{"cap-a", "cap-b"} {
		body := serveBody(t, testDataDir, "bgworker/ensure-reader.php?name="+name)
		require.NotContains(t, body, "limit of", "first two ensures should succeed, got:\n"+body)
	}

	// Third should fail with a cap error.
	body := serveBody(t, testDataDir, "bgworker/ensure-reader.php?name=cap-c")
	assert.Contains(t, body, "limit of 2 reached", "third ensure must hit the catch-all cap:\n"+body)
}

// TestEnsureBackgroundWorkerUndeclared checks that ensure() on a name that
// is neither declared nor covered by a catch-all returns an error.
func TestEnsureBackgroundWorkerUndeclared(t *testing.T) {
	testDataDir := setupFrankenPHP(t,
		frankenphp.WithWorkers("bg-lazy", "testdata/bgworker/named.php", 0,
			frankenphp.WithWorkerBackground()),
		frankenphp.WithNumThreads(2),
	)

	// Script tries to ensure('other-name') which is neither named nor catch-all.
	php := `<?php
try {
    frankenphp_ensure_background_worker('other-name', 2.0);
    echo "FAIL no error";
} catch (RuntimeException $e) {
    echo "OK ", $e->getMessage();
}`
	tmp := testDataDir + "bg-ensure-undeclared.php"
	require.NoError(t, os.WriteFile(tmp, []byte(php), 0644))
	t.Cleanup(func() { _ = os.Remove(tmp) })

	body := serveBody(t, testDataDir, "bg-ensure-undeclared.php")
	assert.Contains(t, body, "OK no background worker configured for name", "ensure of undeclared name should error:\n"+body)
	assert.NotContains(t, body, "FAIL")
}

// TestBackgroundWorkerBootFailureError confirms that an entrypoint which
// throws during boot surfaces the captured details through ensure()'s
// timeout error message: entrypoint path, attempt count, and the PHP
// RuntimeException message. Runs as a non-worker request so ensure uses
// the tolerant lazy-start path (no fail-fast).
func TestBackgroundWorkerBootFailureError(t *testing.T) {
	testDataDir := setupFrankenPHP(t,
		frankenphp.WithWorkers("boot-fail-worker", "testdata/bgworker/boot-fail.php", 0,
			frankenphp.WithWorkerBackground()),
		frankenphp.WithNumThreads(3),
	)

	php := `<?php
try {
    frankenphp_ensure_background_worker('boot-fail-worker', 1.0);
    echo "FAIL no error";
} catch (\RuntimeException $e) {
    echo $e->getMessage();
}`
	tmp := testDataDir + "bg-boot-fail.php"
	require.NoError(t, os.WriteFile(tmp, []byte(php), 0644))
	t.Cleanup(func() { _ = os.Remove(tmp) })

	body := serveBody(t, testDataDir, "bg-boot-fail.php")
	assert.NotContains(t, body, "FAIL", "ensure should have thrown:\n"+body)
	assert.Contains(t, body, `"boot-fail-worker"`)
	assert.Contains(t, body, filepath.Join("bgworker", "boot-fail.php"), "entrypoint path must appear in the error:\n"+body)
	assert.Contains(t, body, "attempt", "attempt count must appear:\n"+body)
	assert.Contains(t, body, "intentional boot failure for test", "PHP exception message must be captured:\n"+body)
}
