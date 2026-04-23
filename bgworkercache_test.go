package frankenphp_test

import (
	"os"
	"strings"
	"testing"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetVarsCacheIdentity verifies that two get_vars calls within one
// request return the *same* zval (pointer identity via ===) when the
// worker hasn't published a new version in between. This is the user-
// visible guarantee that proves the per-request cache is wired.
func TestGetVarsCacheIdentity(t *testing.T) {
	testDataDir := setupFrankenPHP(t,
		frankenphp.WithWorkers("cache-worker", "testdata/background-worker-cache-fixture.php", 1,
			frankenphp.WithWorkerBackground()),
		frankenphp.WithNumThreads(3),
	)

	body := serveBody(t, testDataDir, "background-worker-cache-identity.php")
	assert.Contains(t, body, "first=cached-value")
	assert.Contains(t, body, "second=cached-value")
	assert.Contains(t, body, "identical=true", "cached zvals must be === across repeated reads:\n"+body)
}

// TestGetVarsCacheManyReads exercises the cache path under load: one
// request calls get_vars 500 times against a nested-array worker. The
// second call onward is a cache hit; the test just asserts the script
// completes without corruption.
func TestGetVarsCacheManyReads(t *testing.T) {
	testDataDir := setupFrankenPHP(t,
		frankenphp.WithWorkers("cache-worker", "testdata/background-worker-cache-fixture.php", 1,
			frankenphp.WithWorkerBackground()),
		frankenphp.WithNumThreads(3),
	)

	// ensure() first so the eager-start race doesn't surface before the
	// 500-read loop even begins.
	php := `<?php
frankenphp_ensure_background_worker('cache-worker');
for ($i = 0; $i < 500; $i++) {
    $vars = frankenphp_get_vars('cache-worker');
}
echo 'ok=', $vars['marker'] ?? 'MISSING', "\n";`
	tmp := testDataDir + "bg-cache-many.php"
	require.NoError(t, os.WriteFile(tmp, []byte(php), 0644))
	t.Cleanup(func() { _ = os.Remove(tmp) })

	body := serveBody(t, testDataDir, "bg-cache-many.php")
	assert.Contains(t, body, "ok=cached-value", "500 cached reads should all succeed:\n"+body)
	assert.False(t, strings.Contains(body, "Fatal error") || strings.Contains(body, "corrupted"), "no corruption expected:\n"+body)
}
