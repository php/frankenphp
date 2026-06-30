package frankenphp

import (
	"os"
	"testing"
	"time"

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
