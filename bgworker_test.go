package frankenphp_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// requireFileEventually asserts that `path` appears on disk before the
// deadline. Wraps require.Eventually so call sites stay short.
func requireFileEventually(t testing.TB, path string, msgAndArgs ...any) {
	t.Helper()
	require.Eventually(t, func() bool {
		_, err := os.Stat(path)
		return err == nil
	}, 5*time.Second, 25*time.Millisecond, msgAndArgs...)
}

// requireFileContentEventually waits for `path` to appear with content and
// returns it
func requireFileContentEventually(t *testing.T, path string) string {
	t.Helper()
	require.Eventually(t, func() bool {
		b, err := os.ReadFile(path)
		return err == nil && len(b) > 0
	}, 5*time.Second, 25*time.Millisecond, "file %q did not appear", path)
	b, err := os.ReadFile(path)
	require.NoError(t, err)

	return string(b)
}

// TestBackgroundWorkerLifecycle boots a background worker that touches a
// sentinel file then parks on its handle. It proves the bg worker runs
// (sentinel appears) and that Shutdown returns within a reasonable time.
// The test asserts on Shutdown timing, so it manages Shutdown itself
// instead of using initServers' t.Cleanup hook.
func TestBackgroundWorkerLifecycle(t *testing.T) {
	tmp := t.TempDir()
	sentinel := filepath.Join(tmp, "bg-lifecycle.sentinel")

	require.NoError(t, frankenphp.Init(
		frankenphp.WithWorkers("bg-lifecycle", "testdata/bgworker/basic.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerEnv(map[string]string{"BG_SENTINEL": sentinel}),
		),
		frankenphp.WithNumThreads(2),
	))

	requireFileEventually(t, sentinel, "background worker did not touch sentinel")

	done := make(chan struct{})
	go func() {
		frankenphp.Shutdown()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatalf("Shutdown did not return within 10s")
	}
}

// TestBackgroundWorkerCrashRestarts boots a worker that exit(1)s on its
// first run and touches a "restarted" sentinel on its second run. The
// sentinel proves the crash-restart loop fired.
func TestBackgroundWorkerCrashRestarts(t *testing.T) {
	tmp := t.TempDir()
	crashMarker := filepath.Join(tmp, "bg-crash.marker")
	restarted := filepath.Join(tmp, "bg-crash.restarted")

	initServers(t,
		frankenphp.WithWorkers("bg-crash", "testdata/bgworker/crash.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerEnv(map[string]string{
				"BG_CRASH_MARKER":       crashMarker,
				"BG_RESTARTED_SENTINEL": restarted,
			}),
		),
		frankenphp.WithNumThreads(2),
	)

	requireFileEventually(t, restarted, "background worker did not restart after crash")
}

// TestBackgroundWorkerOnServer scopes a background worker to a Server. It
// proves that the worker inherits the server env (the sentinel directory is
// declared on the server, not on the worker), that FRANKENPHP_WORKER holds
// the worker name, and that the worker does not intercept HTTP requests
// served by the same server.
func TestBackgroundWorkerOnServer(t *testing.T) {
	tmp := t.TempDir()

	server, err := frankenphp.NewServer(
		testDataDir,
		frankenphp.WithServerName("sidekick-server"),
		frankenphp.WithServerEnv(map[string]string{"BG_SENTINEL_DIR": tmp}),
	)
	require.NoError(t, err)

	globalSentinel := filepath.Join(tmp, "global.sentinel")
	initServers(t,
		frankenphp.WithServer(server),
		frankenphp.WithWorkers("jobs", "testdata/bgworker/named.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerServerScope(server),
		),
		// a global worker may reuse the name: names are scoped to their server
		frankenphp.WithWorkers("jobs", "testdata/bgworker/basic.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerEnv(map[string]string{"BG_SENTINEL": globalSentinel}),
		),
		frankenphp.WithNumThreads(3),
	)

	// named.php touches "<BG_SENTINEL_DIR>/<FRANKENPHP_WORKER>": the script sees
	// the declared name, not the server-qualified one used by metrics and logs
	requireFileEventually(t, filepath.Join(tmp, "jobs"), "background worker did not touch its per-name sentinel")
	requireFileEventually(t, globalSentinel, "the global worker sharing the name did not start")

	body := serverGet(t, server, "http://example.com/index.php")
	assert.Contains(t, body, "I am by birth a Genevese", "the server must still serve regular requests")
}

// TestBackgroundWorkerValidation covers the declaration-time errors.
func TestBackgroundWorkerValidation(t *testing.T) {
	t.Cleanup(frankenphp.Shutdown)

	t.Run("name is required", func(t *testing.T) {
		err := frankenphp.Init(
			frankenphp.WithWorkers("", "testdata/bgworker/basic.php", 1, frankenphp.WithWorkerBackground()),
			frankenphp.WithNumThreads(2),
		)
		require.ErrorContains(t, err, "must have an explicit name")
	})

	t.Run("num must be >= 1", func(t *testing.T) {
		err := frankenphp.Init(
			frankenphp.WithWorkers("bg-zero", "testdata/bgworker/basic.php", 0, frankenphp.WithWorkerBackground()),
			frankenphp.WithNumThreads(2),
		)
		require.ErrorContains(t, err, "must declare num >= 1")
	})

	t.Run("names are unique within a server", func(t *testing.T) {
		// a global and a server-scoped worker may share a name (see
		// TestBackgroundWorkerOnServer), two workers of one server may not
		server, err := frankenphp.NewServer(testDataDir)
		require.NoError(t, err)
		err = frankenphp.Init(
			frankenphp.WithServer(server),
			frankenphp.WithWorkers("bg-shared", "testdata/bgworker/basic.php", 1,
				frankenphp.WithWorkerBackground(),
				frankenphp.WithWorkerServerScope(server),
			),
			frankenphp.WithWorkers("bg-shared", "testdata/bgworker/named.php", 1,
				frankenphp.WithWorkerBackground(),
				frankenphp.WithWorkerServerScope(server),
			),
			frankenphp.WithNumThreads(3),
		)
		require.ErrorContains(t, err, "two workers in a server cannot have the same name")
	})

	t.Run("early return without the handle fails startup", func(t *testing.T) {
		err := frankenphp.Init(
			frankenphp.WithWorkers("bg-early", "testdata/bgworker/early-return.php", 1,
				frankenphp.WithWorkerBackground(),
				frankenphp.WithWorkerMaxFailures(2),
			),
			frankenphp.WithNumThreads(2),
		)
		require.ErrorContains(t, err, "frankenphp_get_worker_handle")
	})

	t.Run("fetching the handle without waiting on it fails startup", func(t *testing.T) {
		err := frankenphp.Init(
			frankenphp.WithWorkers("bg-no-wait", "testdata/bgworker/fetch-no-wait.php", 1,
				frankenphp.WithWorkerBackground(),
				frankenphp.WithWorkerMaxFailures(2),
			),
			frankenphp.WithNumThreads(2),
		)
		require.ErrorContains(t, err, "waiting on its handle")
	})

	t.Run("max_threads is rejected", func(t *testing.T) {
		err := frankenphp.Init(
			frankenphp.WithWorkers("bg-scaled", "testdata/bgworker/basic.php", 1,
				frankenphp.WithWorkerBackground(),
				frankenphp.WithWorkerMaxThreads(2),
			),
			frankenphp.WithNumThreads(2),
		)
		require.ErrorContains(t, err, "cannot set max_threads")
	})

	t.Run("two workers cannot report under the same name", func(t *testing.T) {
		// scoping keeps names apart, except for a global name shaped like
		// the "<server>:<name>" of a scoped one
		server, err := frankenphp.NewServer(testDataDir, frankenphp.WithServerName("api"))
		require.NoError(t, err)
		err = frankenphp.Init(
			frankenphp.WithServer(server),
			frankenphp.WithWorkers("jobs", "testdata/bgworker/basic.php", 1,
				frankenphp.WithWorkerBackground(),
				frankenphp.WithWorkerServerScope(server),
			),
			frankenphp.WithWorkers("api:jobs", "testdata/bgworker/named.php", 1,
				frankenphp.WithWorkerBackground(),
			),
			frankenphp.WithNumThreads(3),
		)
		require.ErrorContains(t, err, `two workers cannot report under the same name: "api:jobs"`)
	})

	t.Run("an unregistered server scope is rejected", func(t *testing.T) {
		unregistered, err := frankenphp.NewServer(testDataDir)
		require.NoError(t, err)
		err = frankenphp.Init(
			frankenphp.WithWorkers("bg-orphan", "testdata/bgworker/basic.php", 1,
				frankenphp.WithWorkerBackground(),
				frankenphp.WithWorkerServerScope(unregistered),
			),
			frankenphp.WithNumThreads(2),
		)
		require.ErrorContains(t, err, "not passed to WithServer()")
	})

	t.Run("request matchers are rejected", func(t *testing.T) {
		err := frankenphp.Init(
			frankenphp.WithWorkers("bg-matched", "testdata/bgworker/basic.php", 1,
				frankenphp.WithWorkerBackground(),
				frankenphp.WithWorkerMatcher(func(*http.Request) bool { return true }),
			),
			frankenphp.WithNumThreads(2),
		)
		require.ErrorContains(t, err, "cannot match requests")
	})
}

// TestBackgroundWorkerCannotHandleRequests checks that a request targeting a
// background worker by name is refused rather than dispatched to it.
func TestBackgroundWorkerCannotHandleRequests(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t,
		frankenphp.WithServer(server),
		frankenphp.WithWorkers("jobs", "testdata/bgworker/basic.php", 1, frankenphp.WithWorkerBackground(), frankenphp.WithWorkerServerScope(server)),
		frankenphp.WithNumThreads(2),
	)

	err = server.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "http://example.com/index.php", nil), frankenphp.WithWorkerName("jobs"))
	require.ErrorContains(t, err, `background worker "jobs" cannot handle requests`)
}

// TestBackgroundWorkerParksOnRead checks that a blocking read on the handle
// is a wait too: Init() returns only once the worker is ready, and the EOF
// of the drain unblocks the read so Shutdown() returns promptly.
func TestBackgroundWorkerParksOnRead(t *testing.T) {
	tmp := t.TempDir()
	sentinel := filepath.Join(tmp, "bg-read.sentinel")

	require.NoError(t, frankenphp.Init(
		frankenphp.WithWorkers("bg-read", "testdata/bgworker/read.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerEnv(map[string]string{"BG_SENTINEL": sentinel}),
		),
		frankenphp.WithNumThreads(2),
	))
	requireFileEventually(t, sentinel, "background worker parked on a read did not start")

	done := make(chan struct{})
	go func() {
		frankenphp.Shutdown()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Shutdown did not return within 10s: the read did not observe EOF")
	}
}

// TestBackgroundWorkerParksOnReceive checks that a blocking receive counts
// as a wait as well: it reaches the stream through the transport API rather
// than the read op, so Init() would hang if only reads reported readiness.
func TestBackgroundWorkerParksOnReceive(t *testing.T) {
	sentinel := filepath.Join(t.TempDir(), "bg-recv.sentinel")

	require.NoError(t, frankenphp.Init(
		frankenphp.WithWorkers("bg-recv", "testdata/bgworker/recv.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerEnv(map[string]string{"BG_SENTINEL": sentinel}),
		),
		frankenphp.WithNumThreads(2),
	))
	requireFileEventually(t, sentinel, "background worker parked on a receive did not start")

	done := make(chan struct{})
	go func() {
		frankenphp.Shutdown()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Shutdown did not return within 10s: the receive did not observe EOF")
	}
}

// TestBackgroundWorkerRestartDrainsParkedScript checks that RestartWorkers()
// wakes a parked background script through the drain and re-runs it.
func TestBackgroundWorkerRestartDrainsParkedScript(t *testing.T) {
	tmp := t.TempDir()
	countFile := filepath.Join(tmp, "bg-count.log")

	initServers(t,
		frankenphp.WithWorkers("bg-count", "testdata/bgworker/count.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerEnv(map[string]string{"BG_COUNT_FILE": countFile}),
		),
		frankenphp.WithNumThreads(2),
	)
	runs := func() int {
		b, _ := os.ReadFile(countFile)
		return bytes.Count(b, []byte("\n"))
	}
	require.Eventually(t, func() bool { return runs() == 1 }, 5*time.Second, 25*time.Millisecond, "background worker did not start")

	frankenphp.RestartWorkers()

	require.Eventually(t, func() bool { return runs() == 2 }, 5*time.Second, 25*time.Millisecond, "background worker was not re-run after the restart")
}

// TestGetWorkerHandleOutsideBackgroundWorker checks the function throws on a
// regular request thread instead of handing out a stream.
func TestGetWorkerHandleOutsideBackgroundWorker(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t, frankenphp.WithServer(server), frankenphp.WithNumThreads(1))

	body := serverGet(t, server, "http://example.com/handle-outside.php")

	assert.Contains(t, body, "can only be called from a background worker")
}

// TestWorkerNameInServerVars checks that every worker sees its declared name
// in FRANKENPHP_WORKER and that only background workers get the
// FRANKENPHP_WORKER_BACKGROUND flag.
func TestWorkerNameInServerVars(t *testing.T) {
	sentinel := filepath.Join(t.TempDir(), "flag.txt")
	t.Setenv("FRANKENPHP_WORKER_BACKGROUND", "1")
	server, err := frankenphp.NewServer(testDataDir, frankenphp.WithServerEnv(map[string]string{"FRANKENPHP_WORKER_BACKGROUND": "1"}))
	require.NoError(t, err)
	initServers(t,
		frankenphp.WithServer(server),
		// both names are reserved: neither the worker env here, nor the
		// server env, nor the process environment set below may make an
		// HTTP worker look like a background one
		frankenphp.WithWorkers("web", testDataDir+"worker-name.php", 1,
			frankenphp.WithWorkerServerScope(server),
			frankenphp.WithWorkerEnv(map[string]string{"FRANKENPHP_WORKER_BACKGROUND": "1"}),
		),
		frankenphp.WithWorkers("jobs", "testdata/bgworker/flag.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerServerScope(server),
			frankenphp.WithWorkerEnv(map[string]string{"BG_SENTINEL": sentinel}),
		),
		frankenphp.WithNumThreads(3),
	)

	assert.Equal(t, "web http", serverGet(t, server, "http://example.com/worker-name.php"))

	flag := requireFileContentEventually(t, sentinel)
	assert.Contains(t, flag, "'worker' => 'jobs'")
	assert.Contains(t, flag, "'background' => 'set'")
}

// TestBackgroundWorkerPool checks that num > 1 threads share the name, each
// parks on its own handle, and one drain wakes them all.
func TestBackgroundWorkerPool(t *testing.T) {
	dir := t.TempDir()
	initServers(t,
		frankenphp.WithWorkers("pool", "testdata/bgworker/pool.php", 3,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerEnv(map[string]string{"BG_SENTINEL_DIR": dir}),
		),
		frankenphp.WithNumThreads(4),
	)

	require.Eventually(t, func() bool {
		entries, _ := os.ReadDir(dir)
		return len(entries) == 3
	}, 5*time.Second, 25*time.Millisecond, "the three pool threads did not all start")

	done := make(chan struct{})
	go func() {
		frankenphp.Shutdown()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Shutdown did not drain the whole pool within 10s")
	}
}

// TestBackgroundWorkerMultiEntrypoint checks that two named background
// workers of one server may share a script, since they are not matched by path.
func TestBackgroundWorkerMultiEntrypoint(t *testing.T) {
	tmp := t.TempDir()
	first, second := filepath.Join(tmp, "first"), filepath.Join(tmp, "second")
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t,
		frankenphp.WithServer(server),
		frankenphp.WithWorkers("first", "testdata/bgworker/basic.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerServerScope(server),
			frankenphp.WithWorkerEnv(map[string]string{"BG_SENTINEL": first}),
		),
		frankenphp.WithWorkers("second", "testdata/bgworker/basic.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerServerScope(server),
			frankenphp.WithWorkerEnv(map[string]string{"BG_SENTINEL": second}),
		),
		frankenphp.WithNumThreads(3),
	)

	requireFileEventually(t, first, "the first worker on the shared script did not start")
	requireFileEventually(t, second, "the second worker on the shared script did not start")
}

// TestBackgroundWorkerThreadsComeOnTop checks that background threads are
// reserved on top of num_threads: one HTTP thread plus one background worker
// starts with num_threads 1.
func TestBackgroundWorkerThreadsComeOnTop(t *testing.T) {
	sentinel := filepath.Join(t.TempDir(), "bg-only.sentinel")
	initServers(t,
		frankenphp.WithWorkers("bg-only", "testdata/bgworker/basic.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerEnv(map[string]string{"BG_SENTINEL": sentinel}),
		),
		frankenphp.WithNumThreads(1),
	)

	requireFileEventually(t, sentinel, "background worker did not start with num_threads 1")
}

// TestBackgroundWorkerThreadsComeOnTopOfAutoMaxThreads checks that the
// automatic limit is an HTTP one too: the reservation is added to what it
// resolves, instead of eating into it
func TestBackgroundWorkerThreadsComeOnTopOfAutoMaxThreads(t *testing.T) {
	sentinel := filepath.Join(t.TempDir(), "bg-auto.sentinel")
	initServers(t,
		frankenphp.WithWorkers("bg-auto", "testdata/bgworker/basic.php", 2,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerEnv(map[string]string{"BG_SENTINEL": sentinel}),
		),
		frankenphp.WithNumThreads(2),
		frankenphp.WithMaxThreads(-1),
		frankenphp.WithPhpIni(map[string]string{"memory_limit": "-1"}),
	)
	requireFileEventually(t, sentinel, "background worker did not start")

	// an unlimited memory_limit falls back to twice the HTTP threads, so
	// 2*2 HTTP plus the 2 reserved, not (2+2)*2
	state := frankenphp.DebugState()
	assert.Equal(t, 6, len(state.ThreadDebugStates)+state.ReservedThreadCount)
}

// TestBackgroundWorkerParkingIsNotInterrupted checks that a script parked
// on its handle is not cut short by the two limits it never disables
// itself: max_execution_time, which php_execute_script() re-arms from the
// ini when max_input_time is set, and default_socket_timeout, which the
// handle overrides with an infinite read timeout.
func TestBackgroundWorkerParkingIsNotInterrupted(t *testing.T) {
	countFile := filepath.Join(t.TempDir(), "runs")
	initServers(t,
		frankenphp.WithWorkers("bg-timeout", "testdata/bgworker/no-time-limit.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerEnv(map[string]string{"BG_COUNT_FILE": countFile}),
		),
		frankenphp.WithNumThreads(2),
		frankenphp.WithPhpIni(map[string]string{"max_execution_time": "1", "max_input_time": "1", "default_socket_timeout": "1"}),
	)

	runs := func() int {
		b, _ := os.ReadFile(countFile)

		return bytes.Count(b, []byte("\n"))
	}
	require.Eventually(t, func() bool { return runs() == 1 }, 5*time.Second, 25*time.Millisecond, "background worker did not start")
	// well past both limits
	time.Sleep(2500 * time.Millisecond)
	assert.Equal(t, 1, runs(), "the worker was restarted, so a limit interrupted its park")
}

// TestBackgroundWorkerHandleClosedAndFetchedAgain checks the handle cache:
// a run gets one stream, closing it yields a fresh one on the next fetch,
// and the drain still reaches the script through it.
func TestBackgroundWorkerHandleClosedAndFetchedAgain(t *testing.T) {
	sentinel := filepath.Join(t.TempDir(), "refetch.txt")

	require.NoError(t, frankenphp.Init(
		frankenphp.WithWorkers("bg-refetch", "testdata/bgworker/refetch.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerEnv(map[string]string{"BG_SENTINEL": sentinel}),
		),
		frankenphp.WithNumThreads(2),
	))
	assert.Equal(t, "same then fresh", requireFileContentEventually(t, sentinel))

	done := make(chan struct{})
	go func() {
		frankenphp.Shutdown()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Shutdown did not return within 10s: the re-fetched handle missed the drain")
	}
}

// TestBackgroundWorkerBootFailuresThenSucceeds checks that boot failures below
// max_consecutive_failures are retried with the backoff and Init() still
// succeeds once a run reaches its ready point.
func TestBackgroundWorkerBootFailuresThenSucceeds(t *testing.T) {
	tmp := t.TempDir()
	countFile, sentinel := filepath.Join(tmp, "boots"), filepath.Join(tmp, "ready")
	initServers(t,
		frankenphp.WithWorkers("bg-flaky", "testdata/bgworker/fail-then-succeed.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerEnv(map[string]string{"BG_COUNT_FILE": countFile, "BG_SENTINEL": sentinel, "BG_FAIL_UNTIL": "2"}),
		),
		frankenphp.WithNumThreads(2),
	)

	requireFileEventually(t, sentinel, "background worker did not recover from its boot failures")
	boots, err := os.ReadFile(countFile)
	require.NoError(t, err)
	assert.Equal(t, "3", string(boots), "two boot failures then a success")
}

// TestBackgroundWorkerCrashAfterReadyRestarts checks that a crash after the
// ready point restarts right away without counting toward
// max_consecutive_failures, and that a zero-timeout stream_select() counts
// as the wait.
func TestBackgroundWorkerCrashAfterReadyRestarts(t *testing.T) {
	countFile := filepath.Join(t.TempDir(), "runs")
	initServers(t,
		frankenphp.WithWorkers("bg-crashy", "testdata/bgworker/crash-after-ready.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerMaxFailures(2),
			frankenphp.WithWorkerEnv(map[string]string{"BG_COUNT_FILE": countFile}),
		),
		frankenphp.WithNumThreads(2),
	)

	require.Eventually(t, func() bool {
		b, _ := os.ReadFile(countFile)
		return bytes.Count(b, []byte("\n")) >= 4
	}, 5*time.Second, 25*time.Millisecond, "the worker was not restarted after crashing past its ready point")
}

// TestBackgroundWorkerRebootForceKillsStuckScript checks that a script
// ignoring its handle does not stall RestartWorkers() past the reboot grace
// period: the force-kill ends it and the next run parks normally.
func TestBackgroundWorkerRebootForceKillsStuckScript(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "freebsd" {
		t.Skipf("force-kill cannot interrupt a blocking syscall on %s", runtime.GOOS)
	}

	tmp := t.TempDir()
	once, sentinel := filepath.Join(tmp, "once"), filepath.Join(tmp, "parked")
	initServers(t,
		frankenphp.WithWorkers("bg-stuck", "testdata/bgworker/stuck.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerEnv(map[string]string{"BG_ONCE": once, "BG_SENTINEL": sentinel}),
		),
		frankenphp.WithNumThreads(2),
	)
	requireFileEventually(t, once, "background worker never entered its sleep")

	start := time.Now()
	frankenphp.RestartWorkers()
	assert.WithinDuration(t, start, time.Now(), 10*time.Second, "the reboot must force-kill the stuck script within its grace period")

	requireFileEventually(t, sentinel, "the re-run script did not park")
}
