package frankenphp

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupBgWorker boots FrankenPHP with the given options (internal-package
// variant), registers Shutdown as a t.Cleanup, and returns the absolute
// path to the testdata directory.
func setupBgWorker(t *testing.T, opts ...Option) (testDataDir string) {
	t.Helper()
	cwd, err := os.Getwd()
	require.NoError(t, err)
	testDataDir = cwd + "/testdata/"
	require.NoError(t, Init(opts...))
	t.Cleanup(Shutdown)
	return
}

// requireSentinelEventually asserts that `path` appears on disk before the
// deadline. Wraps require.Eventually so call sites stay short.
func requireSentinelEventually(t testing.TB, path string, msgAndArgs ...any) {
	t.Helper()
	require.Eventually(t, func() bool {
		_, err := os.Stat(path)
		return err == nil
	}, 5*time.Second, 10*time.Millisecond, msgAndArgs...)
}

// TestNextScopeIsDistinct verifies the scope counter
// hands out unique values on consecutive calls.
func TestNextScopeIsDistinct(t *testing.T) {
	a := NextScope()
	b := NextScope()
	assert.NotEqual(t, a, b, "consecutive scopes must differ")
	assert.NotZero(t, a, "scopes must be non-zero (zero is the global scope)")
	assert.NotZero(t, b, "scopes must be non-zero (zero is the global scope)")
}

// TestBackgroundWorkerSameNameDifferentScope declares two named bg
// workers with the same user-facing name in distinct scopes. Both must
// Init successfully (the global workersByName collision check must
// recognize bg workers as scope-isolated), each must produce its own
// sentinel under its scope-specific directory.
func TestBackgroundWorkerSameNameDifferentScope(t *testing.T) {
	scopeA := NextScope()
	scopeB := NextScope()

	tmp := t.TempDir()
	dirA := filepath.Join(tmp, "a")
	dirB := filepath.Join(tmp, "b")
	require.NoError(t, os.MkdirAll(dirA, 0o755))
	require.NoError(t, os.MkdirAll(dirB, 0o755))

	setupBgWorker(t,
		WithWorkers("shared", "testdata/bgworker/named.php", 1,
			WithWorkerBackground(),
			WithWorkerScope(scopeA),
			WithWorkerEnv(map[string]string{"BG_SENTINEL_DIR": dirA}),
		),
		WithWorkers("shared", "testdata/bgworker/named.php", 1,
			WithWorkerBackground(),
			WithWorkerScope(scopeB),
			WithWorkerEnv(map[string]string{"BG_SENTINEL_DIR": dirB}),
		),
		WithNumThreads(4),
	)

	// Both lookups must exist and resolve "shared" to a *worker.
	require.NotNil(t, backgroundLookups[scopeA], "scope A lookup missing")
	require.NotNil(t, backgroundLookups[scopeB], "scope B lookup missing")
	assert.NotNil(t, backgroundLookups[scopeA]["shared"], "scope A must resolve 'shared'")
	assert.NotNil(t, backgroundLookups[scopeB]["shared"], "scope B must resolve 'shared'")
	// And they must be distinct *worker instances (not the same pointer).
	assert.NotSame(t,
		backgroundLookups[scopeA]["shared"],
		backgroundLookups[scopeB]["shared"],
		"each scope must own a distinct *worker for the same name")

	// Each scope's worker writes its sentinel under its own dir.
	requireSentinelEventually(t, filepath.Join(dirA, "shared"), "scope A worker did not touch sentinel")
	requireSentinelEventually(t, filepath.Join(dirB, "shared"), "scope B worker did not touch sentinel")
}
