package frankenphp

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEnsureBackgroundWorkerNamedLazy declares a num=0 named worker, then
// calls ensure() to lazy-start it. The fixture writes a sentinel named
// after FRANKENPHP_WORKER so we can confirm the right instance ran.
func TestEnsureBackgroundWorkerNamedLazy(t *testing.T) {
	tmp := t.TempDir()
	setupBgWorker(t,
		WithWorkers("bg-lazy", "testdata/bgworker/named.php", 0,
			WithWorkerBackground(),
			WithWorkerEnv(map[string]string{"BG_SENTINEL_DIR": tmp}),
		),
		WithNumThreads(2),
	)

	// num=0 means no eager start: the sentinel should not exist yet.
	require.NoFileExists(t, filepath.Join(tmp, "bg-lazy"), "lazy worker should not have started yet")

	require.NoError(t, ensureBackgroundWorker(nil, "bg-lazy", 5*time.Second))
	requireSentinelEventually(t, filepath.Join(tmp, "bg-lazy"),
		"ensure() should have lazy-started the named bg worker")
}

// TestEnsureBackgroundWorkerCatchAll declares a single catch-all (no name)
// and invokes ensure() with two distinct names. Each name should spawn an
// independent instance from the same entrypoint and write its own sentinel.
func TestEnsureBackgroundWorkerCatchAll(t *testing.T) {
	tmp := t.TempDir()
	setupBgWorker(t,
		// Name-less bg worker = catch-all.
		WithWorkers("", "testdata/bgworker/named.php", 0,
			WithWorkerBackground(),
			WithWorkerEnv(map[string]string{"BG_SENTINEL_DIR": tmp}),
		),
		WithNumThreads(4),
	)

	for _, name := range []string{"job-a", "job-b"} {
		require.NoError(t, ensureBackgroundWorker(nil, name, 5*time.Second), "ensure(%s)", name)
	}

	for _, name := range []string{"job-a", "job-b"} {
		requireSentinelEventually(t, filepath.Join(tmp, name),
			"catch-all instance %q should have written its sentinel", name)
	}
}

// TestEnsureBackgroundWorkerCatchAllCap exercises max_threads on the
// catch-all: third distinct name beyond the cap should error.
func TestEnsureBackgroundWorkerCatchAllCap(t *testing.T) {
	tmp := t.TempDir()
	setupBgWorker(t,
		WithWorkers("", "testdata/bgworker/named.php", 0,
			WithWorkerBackground(),
			WithWorkerMaxThreads(2),
			WithWorkerEnv(map[string]string{"BG_SENTINEL_DIR": tmp}),
		),
		WithNumThreads(4),
	)

	require.NoError(t, ensureBackgroundWorker(nil, "cap-a", 5*time.Second))
	require.NoError(t, ensureBackgroundWorker(nil, "cap-b", 5*time.Second))

	require.ErrorContains(t,
		ensureBackgroundWorker(nil, "cap-c", 5*time.Second),
		"limit of 2 reached",
		"third ensure must hit the catch-all cap")
}

// TestEnsureBackgroundWorkerUndeclared confirms ensure() on an undeclared
// name with no catch-all returns the configuration error.
func TestEnsureBackgroundWorkerUndeclared(t *testing.T) {
	setupBgWorker(t,
		WithWorkers("bg-known", "testdata/bgworker/named.php", 0,
			WithWorkerBackground(),
		),
		WithNumThreads(2),
	)

	require.ErrorContains(t,
		ensureBackgroundWorker(nil, "other-name", 5*time.Second),
		"no background worker configured for name")
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

// TestEnsureBackgroundWorkerConcurrent confirms the doc claim that ensure()
// is safe to call concurrently: 16 goroutines hitting the same lazy-named
// declaration produce exactly one spawned thread. Without serialising the
// lazy-start gate (bgLazyStartMu), the second caller could observe the
// flag before the first caller has completed thread reservation, leaving
// the worker in an inconsistent state.
func TestEnsureBackgroundWorkerConcurrent(t *testing.T) {
	setupBgWorker(t,
		WithWorkers("bg-concurrent", "testdata/bgworker/named.php", 0,
			WithWorkerBackground(),
		),
		WithNumThreads(8),
	)

	const goroutines = 16
	var wg sync.WaitGroup
	wg.Add(goroutines)
	errs := make([]error, goroutines)
	start := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			<-start
			errs[idx] = ensureBackgroundWorker(nil, "bg-concurrent", 5*time.Second)
		}(i)
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		require.NoError(t, err, "goroutine %d", i)
	}

	// The lazy-named declaration should resolve to its single *worker,
	// and exactly one thread should have been spawned by the lazy-start
	// path despite the 16 concurrent ensure() callers.
	lookup := backgroundLookups[0]
	require.NotNil(t, lookup)
	w := lookup.byName["bg-concurrent"]
	require.NotNil(t, w)
	assert.Equal(t, 1, w.countThreads(), "exactly one worker thread expected")
}

// TestEnsureBackgroundWorkerTimeout proves ensure() blocks until either
// the worker hits its readiness boundary (frankenphp_get_worker_handle())
// or the timeout expires. The fixture sleep()s without ever calling the
// readiness function, so the second branch must fire.
func TestEnsureBackgroundWorkerTimeout(t *testing.T) {
	setupBgWorker(t,
		WithWorkers("bg-no-handle", "testdata/bgworker/no-handle.php", 0,
			WithWorkerBackground(),
		),
		WithNumThreads(2),
	)

	start := time.Now()
	err := ensureBackgroundWorker(nil, "bg-no-handle", 1*time.Second)
	deadline := start.Add(1 * time.Second)

	require.ErrorContains(t, err,
		"did not call frankenphp_get_worker_handle()",
		"ensure() must time out at the readiness boundary")
	// Lower bound: timer must actually have run; allow a little slop for
	// timer scheduling.
	assert.GreaterOrEqual(t, time.Since(start), 900*time.Millisecond, "ensure() must wait the full timeout")
	// Upper bound: ensure() returned close to the deadline (didn't hang).
	assert.WithinDuration(t, deadline, time.Now(), 4*time.Second, "ensure() must return within a small slack window after the timeout")
}

// TestEnsureBackgroundWorkerBootFailure declares a worker whose entrypoint
// throws on its very first line. ensure() should surface the boot crash
// metadata (entrypoint, exit status, attempt count) instead of just
// reporting a generic timeout. We use a max_consecutive_failures cap so
// the worker stops respawning and the abort path fires deterministically.
func TestEnsureBackgroundWorkerBootFailure(t *testing.T) {
	setupBgWorker(t,
		WithWorkers("bg-boot-fail", "testdata/bgworker/boot-fail.php", 0,
			WithWorkerBackground(),
			WithWorkerMaxFailures(2),
		),
		WithNumThreads(2),
	)

	err := ensureBackgroundWorker(nil, "bg-boot-fail", 5*time.Second)
	require.Error(t, err, "ensure() must surface the boot failure")
	msg := err.Error()
	// One of two paths: either the abort fired (cap reached) and the error
	// mentions max_consecutive_failures, or the timeout fired with the
	// bootFailureInfo attached so the message mentions exit status.
	assert.True(t,
		strings.Contains(msg, "exit status") || strings.Contains(msg, "max_consecutive_failures") || strings.Contains(msg, "failed to start"),
		"ensure() error must reflect the boot crash, got: %s", msg)
}
