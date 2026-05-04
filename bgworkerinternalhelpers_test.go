package frankenphp

import (
	"fmt"
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

// ensureBackgroundWorker starts the named bg worker and waits for
// readiness, abort, or timeout. Test-only adapter mirroring the C
// entry-point's wait without the formatted-error machinery.
func ensureBackgroundWorker(thread *phpThread, name string, timeout time.Duration) error {
	sk, err := startBackgroundWorker(thread, name)
	if err != nil {
		return err
	}
	if timeout <= 0 {
		timeout = defaultEnsureTimeout
	}
	select {
	case <-sk.ready:
		return nil
	case <-sk.aborted:
		return sk.abortErr
	case <-time.After(timeout):
		return fmt.Errorf("background worker %q failed to start within %v", name, timeout)
	}
}
