package frankenphp

import (
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBackgroundWorkerRestartForceKillsStuckThread exercises the force-kill
// path: the fixture sleep()s without watching the stop pipe, so
// handler.drain() cannot wake it. RestartWorkers must go through the
// grace-period timeout and the force-kill primitive (pthread_kill on
// Linux/FreeBSD) to finish within the budget. Skips platforms where
// force-kill cannot interrupt a blocking syscall.
func TestBackgroundWorkerRestartForceKillsStuckThread(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "freebsd" {
		t.Skipf("force-kill cannot interrupt blocking syscalls on %s", runtime.GOOS)
	}

	prev := drainGracePeriod
	drainGracePeriod = 2 * time.Second
	t.Cleanup(func() { drainGracePeriod = prev })

	tmp := t.TempDir()
	sentinel := filepath.Join(tmp, "bg-stuck.sentinel")

	setupBgWorker(t,
		WithWorkers("bg-stuck", "testdata/bgworker/stuck.php", 1,
			WithWorkerBackground(),
			WithWorkerEnv(map[string]string{"BG_SENTINEL": sentinel}),
		),
		WithNumThreads(2),
	)

	// Wait until the bg worker touched the sentinel (the line right
	// before sleep(60)) so we know it is parked in the blocking syscall
	// when the drain fires - that's the only way to prove the
	// force-kill code path was exercised, not the stop-pipe EOF path
	// (the fixture doesn't open the stop pipe at all).
	requireSentinelEventually(t, sentinel, "bg worker never entered sleep()")

	start := time.Now()
	RestartWorkers()
	// Drain budget = grace period (2s) + slack for signal dispatch and
	// drain completion.
	assert.WithinDuration(t, start, time.Now(), 5*time.Second,
		"drain must force-kill the stuck bg worker within the grace period")
}

// TestEnsureBackgroundWorkerCatchAllNumPlusLazy exercises a catch-all
// with both an eager pool (num=1) and lazy ensures: every ensure() must
// succeed alongside the eager thread, since num and the catch-all cap
// reserve independent thread budgets.
func TestEnsureBackgroundWorkerCatchAllNumPlusLazy(t *testing.T) {
	tmp := t.TempDir()
	setupBgWorker(t,
		WithWorkers("", "testdata/bgworker/named.php", 1,
			WithWorkerBackground(),
			WithWorkerEnv(map[string]string{"BG_SENTINEL_DIR": tmp}),
		),
		WithNumThreads(2),
	)

	for _, name := range []string{"a", "b", "c", "d"} {
		require.NoError(t, ensureBackgroundWorker(nil, name, 5*time.Second), "ensure(%s)", name)
	}
	for _, name := range []string{"a", "b", "c", "d"} {
		requireSentinelEventually(t, filepath.Join(tmp, name),
			"lazy catch-all instance %q should have written its sentinel", name)
	}
}

// TestBackgroundWorkerCatchAllEagerInstance verifies that a catch-all
// declared with num>0 actually boots and runs the script — the
// catch-all flag must not silently disable the eager pool.
func TestBackgroundWorkerCatchAllEagerInstance(t *testing.T) {
	tmp := t.TempDir()
	sentinel := filepath.Join(tmp, "eager")
	setupBgWorker(t,
		WithWorkers("", "testdata/bgworker/eager-catchall.php", 1,
			WithWorkerBackground(),
			WithWorkerEnv(map[string]string{"BG_EAGER_SENTINEL": sentinel}),
		),
		WithNumThreads(2),
	)
	requireSentinelEventually(t, sentinel,
		"eager catch-all instance must reach readiness")
}

// TestEnsureBackgroundWorkerPostBootCrashLoopAborts verifies that a
// worker which reaches readiness once and then keeps crashing aborts
// ensure() callers, instead of leaving them stuck on the stale "ready"
// signal.
func TestEnsureBackgroundWorkerPostBootCrashLoopAborts(t *testing.T) {
	setupBgWorker(t,
		WithWorkers("flapper", "testdata/bgworker/readiness-then-crash.php", 0,
			WithWorkerBackground(),
			WithWorkerMaxFailures(2),
		),
		WithNumThreads(2),
	)

	// Returns once the fixture signals readiness via frankenphp_get_worker_handle().
	require.NoError(t, ensureBackgroundWorker(nil, "flapper", 5*time.Second))

	// Subsequent ensure()s must surface the failure: prior abort or a
	// fresh thread that crashes again. Never nil.
	require.Eventually(t, func() bool {
		return ensureBackgroundWorker(nil, "flapper", 1*time.Second) != nil
	}, 10*time.Second, 100*time.Millisecond,
		"ensure() must surface post-boot crash-loop")
}

// TestEnsureBackgroundWorkerCatchAllRespawnAfterCap verifies that once
// a catch-all instance trips the failure cap, the slot is released so
// a retried ensure() can lazy-spawn a fresh thread under the same name.
func TestEnsureBackgroundWorkerCatchAllRespawnAfterCap(t *testing.T) {
	tmp := t.TempDir()
	setupBgWorker(t,
		WithWorkers("", "testdata/bgworker/fail-then-succeed.php", 0,
			WithWorkerBackground(),
			WithWorkerMaxThreads(2),
			WithWorkerMaxFailures(2),
			WithWorkerEnv(map[string]string{
				"BG_MARKER_DIR": tmp,
				// Crash boots 1..3, succeed on boot 4. The cap fires on
				// the 3rd crash because failureCount is checked before
				// the increment in afterScriptExecution.
				"BG_FAIL_UNTIL": "3",
			}),
		),
		WithNumThreads(3),
	)

	err := ensureBackgroundWorker(nil, "respawn", 5*time.Second)
	require.Error(t, err, "first ensure() must hit the cap")

	require.Eventually(t, func() bool {
		return ensureBackgroundWorker(nil, "respawn", 5*time.Second) == nil
	}, 15*time.Second, 100*time.Millisecond,
		"retry must lazy-spawn a fresh thread after the cap")

	requireSentinelEventually(t, filepath.Join(tmp, "ready"),
		"recovered worker must reach readiness")
}
