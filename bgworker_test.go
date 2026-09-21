package frankenphp_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// requireFileEventually asserts that `path` appears on disk before the deadline
func requireFileEventually(t testing.TB, path string, msgAndArgs ...any) {
	t.Helper()
	require.Eventually(t, func() bool {
		_, err := os.Stat(path)
		return err == nil
	}, 5*time.Second, 25*time.Millisecond, msgAndArgs...)
}

// requireFileContentEventually waits for `path` to appear with content
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

// asserts on the timing of Shutdown(), so it calls it itself rather than
// through initServers' t.Cleanup hook
func TestBackgroundWorkerLifecycle(t *testing.T) {
	tmp := t.TempDir()
	sentinel := filepath.Join(tmp, "bg-lifecycle.sentinel")

	t.Cleanup(frankenphp.Shutdown)
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

// the sentinel directory is declared on the server, not on the worker
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

	// named.php touches "<BG_SENTINEL_DIR>/<FRANKENPHP_WORKER_BACKGROUND>": the script sees
	// the declared name, not the server-qualified one used by metrics and logs
	requireFileEventually(t, filepath.Join(tmp, "jobs"), "background worker did not touch its per-name sentinel")
	requireFileEventually(t, globalSentinel, "the global worker sharing the name did not start")

	body := serverGet(t, server, "http://example.com/index.php")
	assert.Contains(t, body, "I am by birth a Genevese", "the server must still serve regular requests")
}

func TestBackgroundWorkerValidation(t *testing.T) {
	t.Cleanup(frankenphp.Shutdown)

	t.Run("name is required", func(t *testing.T) {
		err := frankenphp.Init(
			frankenphp.WithWorkers("", "testdata/bgworker/basic.php", 1, frankenphp.WithWorkerBackground()),
			frankenphp.WithNumThreads(2),
		)
		require.ErrorContains(t, err, "must have an explicit name")
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
		require.ErrorContains(t, err, "without calling WorkerHandle::tick()")
	})

	t.Run("fetching the handle without ticking fails startup", func(t *testing.T) {
		err := frankenphp.Init(
			frankenphp.WithWorkers("bg-no-tick", "testdata/bgworker/fetch-no-tick.php", 1,
				frankenphp.WithWorkerBackground(),
				frankenphp.WithWorkerMaxFailures(2),
			),
			frankenphp.WithNumThreads(2),
		)
		require.ErrorContains(t, err, "without calling WorkerHandle::tick()")
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

// a blocking read is a wait too: it reports the worker ready, and the EOF of
// the drain unblocks it
func TestBackgroundWorkerParksOnRead(t *testing.T) {
	tmp := t.TempDir()
	sentinel := filepath.Join(tmp, "bg-read.sentinel")

	t.Cleanup(frankenphp.Shutdown)
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

// a blocking receive reaches the stream through the transport API rather than
// the read op, so Init() would hang if only reads reported readiness
func TestBackgroundWorkerParksOnReceive(t *testing.T) {
	sentinel := filepath.Join(t.TempDir(), "bg-recv.sentinel")

	t.Cleanup(frankenphp.Shutdown)
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

func TestWorkerHandleOutsideBackgroundWorker(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t, frankenphp.WithServer(server), frankenphp.WithNumThreads(1))

	body := serverGet(t, server, "http://example.com/handle-outside.php")

	assert.Contains(t, body, `FrankenPHP\WorkerHandle can only be created from a background worker`)
}

// without the wake-up sent at start, a script that only ticks when its handle
// is readable would keep Init() waiting until the drain
func TestBackgroundWorkerLoopTicksOnItsOwn(t *testing.T) {
	sentinel := filepath.Join(t.TempDir(), "loop.sentinel")
	initServers(t,
		frankenphp.WithWorkers("bg-loop", "testdata/bgworker/loop.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerEnv(map[string]string{"BG_SENTINEL": sentinel}),
		),
		frankenphp.WithNumThreads(2),
	)
	requireFileEventually(t, sentinel, "background worker did not start")
}

// the handle is readable at start and quiet once a tick returned, so a loop
// selecting on it blocks instead of spinning
func TestBackgroundWorkerTickLeavesTheHandleQuiet(t *testing.T) {
	sentinel := filepath.Join(t.TempDir(), "readable.txt")
	initServers(t,
		frankenphp.WithWorkers("bg-readable", "testdata/bgworker/readable.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerEnv(map[string]string{"BG_SENTINEL": sentinel}),
		),
		frankenphp.WithNumThreads(2),
	)

	assert.Equal(t, "start:readable after tick:quiet after second tick:quiet", requireFileContentEventually(t, sentinel))
}

// true while the worker runs, false once it is drained, and still false on the
// next call
func TestBackgroundWorkerTick(t *testing.T) {
	sentinel := filepath.Join(t.TempDir(), "ticks.txt")

	t.Cleanup(frankenphp.Shutdown)
	require.NoError(t, frankenphp.Init(
		frankenphp.WithWorkers("bg-tick", "testdata/bgworker/tick.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerEnv(map[string]string{"BG_SENTINEL": sentinel}),
		),
		frankenphp.WithNumThreads(2),
	))
	assert.Equal(t, "true true", requireFileContentEventually(t, sentinel))

	// Shutdown() waits for the script to leave, so the file is final after it
	frankenphp.Shutdown()
	b, err := os.ReadFile(sentinel)
	require.NoError(t, err)
	assert.Equal(t, "true true false false", string(b))
}

// an HTTP worker sees FRANKENPHP_WORKER as it always did, a background one its
// declared name in FRANKENPHP_WORKER_BACKGROUND and no FRANKENPHP_WORKER
func TestWorkerNameInServerVars(t *testing.T) {
	sentinel := filepath.Join(t.TempDir(), "flag.txt")
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t,
		frankenphp.WithServer(server),
		frankenphp.WithWorkers("web", testDataDir+"worker-name.php", 1, frankenphp.WithWorkerServerScope(server)),
		frankenphp.WithWorkers("jobs", "testdata/bgworker/flag.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerServerScope(server),
			frankenphp.WithWorkerEnv(map[string]string{"BG_SENTINEL": sentinel}),
		),
		frankenphp.WithNumThreads(3),
	)

	assert.Equal(t, "1 http", serverGet(t, server, "http://example.com/worker-name.php"))

	flag := requireFileContentEventually(t, sentinel)
	assert.Contains(t, flag, "'worker' => 'unset'")
	assert.Contains(t, flag, "'background' => 'jobs'")
}

// the threads of a pool share the name, park on a handle each, and one drain
// wakes them all
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

// two named background workers of one server may share a script, they are not
// matched by path
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

// background threads are reserved on top of num_threads
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

// a background worker serves no requests, so num defaults to one thread rather
// than to the CPU count of an HTTP worker
func TestBackgroundWorkerDefaultsToOneThread(t *testing.T) {
	sentinel := filepath.Join(t.TempDir(), "bg-default.sentinel")
	initServers(t,
		frankenphp.WithWorkers("bg-default", "testdata/bgworker/basic.php", 0,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerEnv(map[string]string{"BG_SENTINEL": sentinel}),
		),
		frankenphp.WithNumThreads(1),
	)
	requireFileEventually(t, sentinel, "background worker did not start without an explicit num")

	background := 0
	for _, thread := range frankenphp.DebugState().ThreadDebugStates {
		if strings.Contains(thread.Name, "Background Worker") {
			background++
		}
	}
	assert.Equal(t, 1, background)
}

// the automatic limit is an HTTP one too: the reservation is added to what it
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

// max_execution_time applies until the first tick, a setup that outlives it
// fails Init(). The limit is PHP's, so the test only runs where its timers are
// known to fire under FrankenPHP: the max execution timers of Linux ZTS builds
func TestBackgroundWorkerBootstrapIsBounded(t *testing.T) {
	if !frankenphp.Config().ZendMaxExecutionTimers {
		t.Skip("max_execution_time is only reliable with Zend max execution timers")
	}

	countFile := filepath.Join(t.TempDir(), "boots")
	err := frankenphp.Init(
		frankenphp.WithWorkers("bg-slow", "testdata/bgworker/slow-boot.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerMaxFailures(0),
			frankenphp.WithWorkerEnv(map[string]string{"BG_COUNT_FILE": countFile}),
		),
		frankenphp.WithNumThreads(2),
		frankenphp.WithPhpIni(map[string]string{"max_execution_time": "1"}),
	)
	if err == nil {
		frankenphp.Shutdown()
	}
	require.ErrorContains(t, err, "keeps crashing")

	b, _ := os.ReadFile(countFile)
	assert.Equal(t, 1, bytes.Count(b, []byte("\n")), "the limit should have ended the one and only boot")
}

// past its first tick, a parked script is cut short by neither of the two
// limits it never disables itself: max_execution_time, disarmed by the tick,
// and default_socket_timeout, which the handle's stream overrides
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

// the same stream every time, a fresh one once the script closed it, one per
// handle, and the drain reaches the script through all of them
func TestBackgroundWorkerStreamClosedAndFetchedAgain(t *testing.T) {
	sentinel := filepath.Join(t.TempDir(), "refetch.txt")

	t.Cleanup(frankenphp.Shutdown)
	require.NoError(t, frankenphp.Init(
		frankenphp.WithWorkers("bg-refetch", "testdata/bgworker/refetch.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerEnv(map[string]string{"BG_SENTINEL": sentinel}),
		),
		frankenphp.WithNumThreads(2),
	))
	assert.Equal(t, "same then fresh then its own", requireFileContentEventually(t, sentinel))

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

// boot failures below max_consecutive_failures are retried with the backoff,
// and Init() still succeeds once a run reaches its ready point
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

// a crash past the ready point restarts without counting toward
// max_consecutive_failures, and a drain cuts the backoff short
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
		return bytes.Count(b, []byte("\n")) >= 5
	}, 5*time.Second, 25*time.Millisecond, "the worker was not restarted after crashing past its ready point")

	// past four crashes the wait is at its 1s cap
	start := time.Now()
	frankenphp.Shutdown()
	assert.Less(t, time.Since(start), 500*time.Millisecond, "Shutdown() waited for the backoff")
}

// a script returning right after its tick is re-run with the crash backoff
func TestBackgroundWorkerCleanExitIsPaced(t *testing.T) {
	countFile := filepath.Join(t.TempDir(), "runs")
	initServers(t,
		frankenphp.WithWorkers("bg-exit", "testdata/bgworker/exit-after-tick.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerEnv(map[string]string{"BG_COUNT_FILE": countFile}),
		),
		frankenphp.WithNumThreads(2),
	)

	time.Sleep(time.Second)
	b, err := os.ReadFile(countFile)
	require.NoError(t, err)
	runs := bytes.Count(b, []byte("\n"))
	assert.GreaterOrEqual(t, runs, 2, "the script was not re-run")
	assert.LessOrEqual(t, runs, 8, "the re-runs were not paced")
}

// the other side of the pacing: a worker that does some work and returns is
// re-run at its own pace
func TestBackgroundWorkerCyclingIsNotThrottled(t *testing.T) {
	countFile := filepath.Join(t.TempDir(), "runs")
	initServers(t,
		frankenphp.WithWorkers("bg-cycle", "testdata/bgworker/cycle.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerEnv(map[string]string{"BG_COUNT_FILE": countFile}),
		),
		frankenphp.WithNumThreads(2),
	)

	// the interval between two runs is the work plus whatever FrankenPHP
	// waits: comparing the last to the first cancels how fast the machine
	// is, and a backoff counting these runs would have grown it by a second
	starts := func() []float64 {
		b, _ := os.ReadFile(countFile)
		var times []float64
		for _, line := range strings.Fields(string(b)) {
			if t, err := strconv.ParseFloat(line, 64); err == nil {
				times = append(times, t)
			}
		}

		return times
	}
	require.Eventually(t, func() bool { return len(starts()) >= 6 }, 15*time.Second, 50*time.Millisecond, "the worker did not run six times")

	times := starts()
	first, last := times[1]-times[0], times[5]-times[4]
	assert.Less(t, last-first, 0.5, "the interval between runs grew, the worker was throttled")
}

// a script ignoring its handle does not stall RestartWorkers() past the reboot
// grace period, the force-kill ends it
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

// the poll API of PHP 8.6 waits on the handle itself, no stream involved
func TestBackgroundWorkerPollHandle(t *testing.T) {
	if frankenphp.Version().VersionID < 80600 {
		t.Skip("the poll API needs PHP 8.6")
	}

	sentinel := filepath.Join(t.TempDir(), "poll.backend")
	initServers(t,
		frankenphp.WithWorkers("bg-poll", "testdata/bgworker/poll.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerEnv(map[string]string{"BG_SENTINEL": sentinel}),
		),
		frankenphp.WithNumThreads(2),
	)

	// the server only starts once the worker ticked, which it does through
	// the context: a backend name means the wait came back on its own
	assert.NotEmpty(t, requireFileContentEventually(t, sentinel))
}

// the handle implements Io\Poll\Handle on every version: PHP 8.6 declares the
// interface with the poll API, FrankenPHP declares it below that
func TestBackgroundWorkerHandleIsAPollHandle(t *testing.T) {
	sentinel := filepath.Join(t.TempDir(), "poll.json")
	initServers(t,
		frankenphp.WithWorkers("bg-iface", "testdata/bgworker/poll-interface.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerEnv(map[string]string{"BG_SENTINEL": sentinel}),
		),
		frankenphp.WithNumThreads(2),
	)

	assert.JSONEq(t, `{"handle":true,"internal":true}`, requireFileContentEventually(t, sentinel))
}

// the methods do not trust the constructor's guard: unserialize() is refused
// outright, and a handle Reflection built on a request thread throws instead
// of reaching the Go side, which would panic and take the process with it
func TestWorkerHandleBuiltBehindTheConstructor(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t, frankenphp.WithServer(server), frankenphp.WithNumThreads(1))

	body := serverGet(t, server, "http://example.com/handle-bypass.php")
	assert.Contains(t, body, "unserialize: Exception: Unserialization of 'FrankenPHP\\WorkerHandle' is not allowed")
	assert.Contains(t, body, "reflection: ReflectionException:")
	assert.Contains(t, body, "construct: RuntimeException: FrankenPHP\\WorkerHandle can only be created from a background worker")
}

// a script that parks without ever ticking fails Init() instead of hanging it,
// the only failure mode available where Zend max execution timers are not
func TestBackgroundWorkerBootTimeout(t *testing.T) {
	err := frankenphp.Init(
		frankenphp.WithWorkers("bg-mute", "testdata/bgworker/never-ticks.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerBootTimeout(300*time.Millisecond),
		),
		frankenphp.WithNumThreads(2),
	)
	if err == nil {
		frankenphp.Shutdown()
	}
	require.ErrorContains(t, err, `background worker "bg-mute" did not call WorkerHandle::tick() within 300ms`)
}
