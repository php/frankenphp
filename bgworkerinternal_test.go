package frankenphp

import (
	"context"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBackgroundWorkerRestartForceKillsStuckThread actually exercises the
// force-kill path: the fixture sleep()s without watching the stop pipe,
// so handler.drain() cannot wake it. RestartWorkers must go through the
// grace-period timeout and the force-kill primitive (pthread_kill on
// Linux/FreeBSD) to finish within the budget. Skips platforms where
// force-kill cannot interrupt a blocking syscall (macOS has no realtime
// signals, Windows non-alertable Sleep stays uninterruptible).
func TestBackgroundWorkerRestartForceKillsStuckThread(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "freebsd" {
		t.Skipf("force-kill cannot interrupt blocking syscalls on %s", runtime.GOOS)
	}

	prev := drainGracePeriod
	drainGracePeriod = 2 * time.Second
	t.Cleanup(func() { drainGracePeriod = prev })

	cwd, _ := os.Getwd()
	testDataDir := cwd + "/testdata/"

	require.NoError(t, Init(
		WithWorkers("bg-stuck", testDataDir+"bgworker/stuck.php", 1,
			WithWorkerBackground()),
		WithNumThreads(2),
	))
	t.Cleanup(Shutdown)

	// Wait until the bg worker published 'ready' (the line right before
	// sleep(60)) so we know it is actually parked in the blocking
	// syscall when the drain fires - that's the only way to prove the
	// force-kill code path was exercised, not the stop-pipe EOF path.
	readerPHP := `<?php
try {
    $vars = frankenphp_get_vars('bg-stuck');
    echo 'ready=', $vars['ready'] ?? 'MISSING';
} catch (\Throwable $e) {
    echo 'err=', $e->getMessage();
}`
	tmp := testDataDir + "bg-stuck-reader.php"
	require.NoError(t, os.WriteFile(tmp, []byte(readerPHP), 0644))
	t.Cleanup(func() { _ = os.Remove(tmp) })

	require.Eventually(t, func() bool {
		req := httptest.NewRequest("GET", "http://example.com/bg-stuck-reader.php", nil)
		fr, err := NewRequestWithContext(req, WithRequestDocumentRoot(testDataDir, false))
		if err != nil {
			return false
		}
		w := httptest.NewRecorder()
		_ = ServeHTTP(w, fr)
		body, _ := io.ReadAll(w.Result().Body)
		return strings.Contains(string(body), "ready=1")
	}, 5*time.Second, 25*time.Millisecond, "bg worker never entered sleep()")

	start := time.Now()
	RestartWorkers()
	// Drain budget = grace period (2s) + slack for signal dispatch and
	// drain completion.
	assert.WithinDuration(t, start, time.Now(), 5*time.Second,
		"drain must force-kill the stuck bg worker within the grace period")
}

// TestEnsureBackgroundWorkerCatchAllNumPlusLazy declares a catch-all with
// an eager pool (num=1) and no explicit max_threads. The previous
// reservation logic only counted max(num, max_threads) and would
// undercount when num>0 with max_threads unset, leaving lazy ensure()
// callers without inactive thread slots even though the cap (16) said
// they were allowed. With the fix in place, all four lazy ensure()s
// must succeed alongside the eager pool thread.
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

// TestBackgroundWorkerCatchAllEagerInstance proves that the eager
// num>0 pool of a catch-all worker actually boots and runs to
// readiness, alongside any lazy ensure() instances. The previous
// reservation logic was sufficient for the eager thread to land in a
// PHP slot but did not assert it actually executed; this guards
// against a regression where catch-all eager num is silently treated
// as a no-op.
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
		"eager catch-all instance must reach readiness and write its sentinel")
}

// TestEnsureBackgroundWorkerPostBootCrashLoopAborts proves that a worker
// which reaches readiness once (closing r.ready) and then crashes
// repeatedly post-readiness eventually surfaces the failure to ensure()
// callers via r.aborted, instead of leaving them with a stale "ready"
// signal pointing at a dead worker. Regression test for the silent-
// failure mode in the cap branch of afterScriptExecution.
func TestEnsureBackgroundWorkerPostBootCrashLoopAborts(t *testing.T) {
	setupBgWorker(t,
		WithWorkers("flapper", "testdata/bgworker/readiness-then-crash.php", 0,
			WithWorkerBackground(),
			WithWorkerMaxFailures(2),
		),
		WithNumThreads(2),
	)

	// First call returns once the worker reaches readiness — the
	// fixture's frankenphp_get_worker_handle() fires markReady before
	// the script exit(1)s.
	require.NoError(t, ensureBackgroundWorker(nil, "flapper", 5*time.Second))

	// After at most max_consecutive_failures = 2 post-readiness crashes
	// (each <1s with quadratic backoff capped at 1s), the cap branch
	// fires abort() + invalidates the registry slot. Subsequent
	// ensure() calls either see the prior abort, or lazy-spawn a fresh
	// thread that ALSO crashes and aborts. Either way: error, not nil.
	require.Eventually(t, func() bool {
		return ensureBackgroundWorker(nil, "flapper", 1*time.Second) != nil
	}, 10*time.Second, 100*time.Millisecond,
		"ensure() must surface post-boot crash-loop instead of silently returning nil")
}

// TestEnsureBackgroundWorkerCatchAllRespawnAfterCap proves that
// invalidateBackgroundEntry actually reclaims the catch-all slot after
// the cap fires: a fixture that crashes its first N boots and succeeds
// on boot N+1 must end up reaching readiness via a retried ensure(),
// rather than being permanently stuck behind the cap.
func TestEnsureBackgroundWorkerCatchAllRespawnAfterCap(t *testing.T) {
	tmp := t.TempDir()
	setupBgWorker(t,
		WithWorkers("", "testdata/bgworker/fail-then-succeed.php", 0,
			WithWorkerBackground(),
			WithWorkerMaxThreads(2),
			WithWorkerMaxFailures(2),
			WithWorkerEnv(map[string]string{
				"BG_MARKER_DIR": tmp,
				// Fail boots 1..3, succeed boot 4+. afterScriptExecution
				// in threadbackgroundworker.go checks failureCount >= max
				// before the failureCount++ at the bottom, so max=2 means
				// the cap fires on the 3rd crash.
				"BG_FAIL_UNTIL": "3",
			}),
		),
		WithNumThreads(3),
	)

	// First ensure() drives boots 1..3 (all crash, cap hits, abort
	// fires + catch-all entry invalidated).
	err := ensureBackgroundWorker(nil, "respawn", 5*time.Second)
	require.Error(t, err, "first ensure() must hit the cap")

	// A retry must lazy-spawn a fresh thread (boot 4) that reaches
	// readiness. Without invalidate, catchAllNames["respawn"] would
	// still hold the prior aborted r and ensure() would short-circuit
	// with the stale error.
	require.Eventually(t, func() bool {
		return ensureBackgroundWorker(nil, "respawn", 5*time.Second) == nil
	}, 15*time.Second, 100*time.Millisecond,
		"ensure() must lazy-spawn a fresh thread after the cap invalidates the slot")

	requireSentinelEventually(t, filepath.Join(tmp, "ready"),
		"recovered worker must reach readiness")
}

// TestEnsureBackgroundWorkerCatchAllSelfIdentityRejected verifies
// ensure() rejects the catch-all's own filepath uniformly across
// num=0 and num=1.
func TestEnsureBackgroundWorkerCatchAllSelfIdentityRejected(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err)
	// The catch-all's user-facing name is its absolute file path
	// (set by newWorker when the declaration leaves name empty).
	absPath := filepath.Join(cwd, "testdata/bgworker/named.php")

	for _, tc := range []struct {
		label string
		num   int
	}{
		{"lazy num=0", 0},
		{"eager num=1", 1},
	} {
		t.Run(tc.label, func(t *testing.T) {
			setupBgWorker(t,
				WithWorkers("", "testdata/bgworker/named.php", tc.num,
					WithWorkerBackground(),
				),
				WithNumThreads(2),
			)

			err := ensureBackgroundWorker(nil, absPath, 1*time.Second)
			require.Error(t, err, "ensure() with the catch-all's file path must be rejected")
			// Echoes the rejected input so a PHP caller can see what
			// they passed; "catch-all's own name" surfaces the cause.
			assert.Contains(t, err.Error(), absPath, "error must echo the rejected input")
			assert.Contains(t, err.Error(), "catch-all's own name", "error must explain why the input was rejected")
		})
	}
}

// TestBackgroundWorkerCatchAllPerScope declares a catch-all in two
// distinct scopes and verifies that ensure() in scope A consumes a slot
// from scope A's catch-all only, leaving scope B's catch-all untouched.
func TestBackgroundWorkerCatchAllPerScope(t *testing.T) {
	scopeA := NextScope()
	scopeB := NextScope()

	tmp := t.TempDir()
	dirA := filepath.Join(tmp, "a")
	dirB := filepath.Join(tmp, "b")
	require.NoError(t, os.MkdirAll(dirA, 0o755))
	require.NoError(t, os.MkdirAll(dirB, 0o755))

	setupBgWorker(t,
		WithWorkers("", "testdata/bgworker/named.php", 0,
			WithWorkerBackground(),
			WithWorkerScope(scopeA),
			WithWorkerEnv(map[string]string{"BG_SENTINEL_DIR": dirA}),
		),
		WithWorkers("", "testdata/bgworker/named.php", 0,
			WithWorkerBackground(),
			WithWorkerScope(scopeB),
			WithWorkerEnv(map[string]string{"BG_SENTINEL_DIR": dirB}),
		),
		WithNumThreads(4),
	)

	// Pre-conditions: each scope's catch-all is empty.
	require.Empty(t, backgroundLookups[scopeA].catchAll.bg.catchAllNames, "scope A catch-all must start empty")
	require.Empty(t, backgroundLookups[scopeB].catchAll.bg.catchAllNames, "scope B catch-all must start empty")

	// Drive ensure() through a fake "request" context tagged to scope A.
	fc := newFrankenPHPContext()
	fc.scope = scopeA
	ctx := context.WithValue(context.Background(), contextKey, fc)
	thread := &phpThread{}
	thread.handler = &fakeContextThread{ctx: ctx}
	require.NoError(t, ensureBackgroundWorker(thread, "job-a", 5*time.Second))

	// The lazy-started instance must land in scope A's catch-all only.
	require.Eventually(t, func() bool {
		_, ok := backgroundLookups[scopeA].catchAll.bg.catchAllNames["job-a"]
		return ok
	}, 2*time.Second, 10*time.Millisecond, "ensure() must populate scope A's catch-all")
	assert.Empty(t, backgroundLookups[scopeB].catchAll.bg.catchAllNames, "scope B catch-all must remain untouched")

	assert.Equal(t, 1, backgroundLookups[scopeA].catchAll.countThreads(), "scope A catch-all must host exactly one thread")
	assert.Equal(t, 0, backgroundLookups[scopeB].catchAll.countThreads(), "scope B catch-all must host zero threads")

	requireSentinelEventually(t, filepath.Join(dirA, "job-a"),
		"scope A catch-all instance must write its sentinel under scope A's BG_SENTINEL_DIR")
}

// fakeContextThread is a threadHandler stub for the request-context
// fallback path in getLookup.
type fakeContextThread struct {
	ctx context.Context
}

func (h *fakeContextThread) name() string                          { return "fake" }
func (h *fakeContextThread) beforeScriptExecution() string         { return "" }
func (h *fakeContextThread) afterScriptExecution(int)              {}
func (h *fakeContextThread) frankenPHPContext() *frankenPHPContext { return nil }
func (h *fakeContextThread) drain()                                {}
func (h *fakeContextThread) context() context.Context              { return h.ctx }
func (h *fakeContextThread) scopedWorker() *worker                 { return nil }
