package frankenphp_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskRoundTrip(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t, frankenphp.WithServer(server), bgWorker("echo", "task-worker.php", nil, server), frankenphp.WithNumThreads(2))

	body := serverGet(t, server, "http://example.com/task.php?input=hello")
	assert.Contains(t, body, `"result":"processed:hello"`)
	assert.Contains(t, body, `"worker":"echo"`)
	assert.True(t, strings.HasSuffix(body, "\ndone"), body)

	// the worker loops: a second task on the same thread
	assert.Contains(t, serverGet(t, server, "http://example.com/task.php?input=again"), `"result":"processed:again"`)

	// progress updates come in order, before the result
	lines := strings.Split(serverGet(t, server, "http://example.com/task.php?input=steps&steps=2"), "\n")
	require.Len(t, lines, 4)
	assert.Equal(t, `{"step":1,"of":2}`, lines[0])
	assert.Equal(t, `{"step":2,"of":2}`, lines[1])
	assert.Contains(t, lines[2], `"result":"processed:steps"`)
	assert.Equal(t, "done", lines[3])
}

func TestTaskScopedToServer(t *testing.T) {
	server1, _ := frankenphp.NewServer(testDataDir, frankenphp.WithServerName("one"))
	server2, _ := frankenphp.NewServer(testDataDir, frankenphp.WithServerName("two"))
	initServers(t,
		frankenphp.WithServer(server1),
		frankenphp.WithServer(server2),
		bgWorker("jobs", "task-worker.php", map[string]string{"BG_TAG": "one"}, server1),
		bgWorker("jobs", "task-worker.php", map[string]string{"BG_TAG": "two"}, server2),
		frankenphp.WithNumThreads(3),
	)

	assert.Contains(t, serverGet(t, server1, "http://example.com/task.php?name=jobs"), `"tag":"one"`)
	assert.Contains(t, serverGet(t, server2, "http://example.com/task.php?name=jobs"), `"tag":"two"`)
}

// a background worker may send tasks too, here while booting
func TestTaskFromBackgroundWorker(t *testing.T) {
	sentinel := filepath.Join(t.TempDir(), "relay.json")
	initServers(t,
		bgWorker("relay", "task-relay.php", map[string]string{"BG_TARGET": "echo", "BG_SENTINEL": sentinel}, nil),
		bgWorker("echo", "task-worker.php", nil, nil),
		frankenphp.WithNumThreads(3),
	)

	assert.Contains(t, requireFileContentEventually(t, sentinel), `"result":"processed:relayed"`)
}

// a sender waits for a thread to pick its task up, up to the timeout
func TestTaskPickupTimeout(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t, frankenphp.WithServer(server), bgWorker("echo", "task-worker.php", nil, server), frankenphp.WithNumThreads(2))

	body := serverGet(t, server, "http://example.com/task-busy.php")
	assert.Contains(t, body, `picked up the task in time`)
	assert.Contains(t, body, `"result":"processed:slow"`)
}

// a worker exiting with a task open fails the sender's read, then restarts
func TestTaskCrashMidTask(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t, frankenphp.WithServer(server), bgWorker("echo", "task-worker.php", nil, server), frankenphp.WithNumThreads(2))

	assert.Contains(t, serverGet(t, server, "http://example.com/task.php?crash=1"), "exited without completing the task")
	assert.Contains(t, serverGet(t, server, "http://example.com/task.php?input=after"), `"result":"processed:after"`)
}

// closing the stream abandons the task: the worker sees it on its own stream
func TestTaskAbandoned(t *testing.T) {
	sentinel := filepath.Join(t.TempDir(), "abandoned.txt")
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t, frankenphp.WithServer(server), bgWorker("echo", "task-worker.php", map[string]string{"BG_SENTINEL": sentinel}, server), frankenphp.WithNumThreads(2))

	assert.Equal(t, "closed", serverGet(t, server, "http://example.com/task.php?sleep_ms=200&close_early=1"))
	assert.Contains(t, requireFileContentEventually(t, sentinel), "the sender closed the task before the update")
}

// a task queued while the only thread is busy reaches it when it reads its
// handle again, even with a loop taking one task per wake-up
func TestTaskQueuedWhileBusy(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t, frankenphp.WithServer(server), bgWorker("echo", "task-worker.php", map[string]string{"BG_LOOP": "if"}, server), frankenphp.WithNumThreads(3))

	bodies := make(chan string, 2)
	for _, input := range []string{"first", "second"} {
		go func() {
			w := httptest.NewRecorder()
			_ = server.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "http://example.com/task.php?sleep_ms=100&input="+input, nil))
			b, _ := io.ReadAll(w.Result().Body)
			bodies <- string(b)
		}()
	}
	results := <-bodies + <-bodies
	assert.Contains(t, results, `"result":"processed:first"`)
	assert.Contains(t, results, `"result":"processed:second"`)
}

// the threads of a pool share the queue, and stream_select() works on the
// sender's streams
func TestTaskPool(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t,
		frankenphp.WithServer(server),
		frankenphp.WithWorkers("pool", "testdata/bgworker/task-worker.php", 2, frankenphp.WithWorkerBackground(), frankenphp.WithWorkerServerScope(server)),
		frankenphp.WithNumThreads(3),
	)

	assert.Equal(t, "[\"processed:a\",\"processed:b\"]\ntwo threads", serverGet(t, server, "http://example.com/task-pool.php"))
}

func TestTaskErrors(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t, frankenphp.WithServer(server), bgWorker("echo", "task-worker.php", nil, server), frankenphp.WithNumThreads(2))

	body := serverGet(t, server, "http://example.com/task-errors.php")
	assert.Contains(t, body, `unknown: RuntimeException: frankenphp_send_task(): unknown background worker "nope"`)
	assert.Contains(t, body, "payload: ValueError: frankenphp_send_task(): payload values must be null, scalars, arrays or enums")
	assert.Contains(t, body, "timeout: ValueError: frankenphp_send_task(): Argument #3 ($timeout) must be greater than or equal to 0")
	assert.Contains(t, body, "receive: RuntimeException: frankenphp_receive_task() can only be called from a background worker")
	assert.Contains(t, body, "update: TypeError: frankenphp_update_task(): Argument #1 ($stream) must be a stream returned by frankenphp_receive_task()")
	assert.Contains(t, body, "read: TypeError: frankenphp_read_task(): Argument #1 ($stream) must be a stream returned by frankenphp_send_task()")
}

// a sender waiting for a busy worker to pick its task up is released by
// Shutdown() instead of holding it
func TestTaskSenderUnblockedOnShutdown(t *testing.T) {
	mark := filepath.Join(t.TempDir(), "picked")
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	t.Cleanup(frankenphp.Shutdown)
	require.NoError(t, frankenphp.Init(frankenphp.WithServer(server), bgWorker("echo", "task-worker.php", nil, server), frankenphp.WithNumThreads(2)))

	body := sendWhileWorkerBusy(t, server, mark)
	done := make(chan struct{})
	go func() {
		frankenphp.Shutdown()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Shutdown did not return within 10s")
	}
	assert.Contains(t, <-body, "FrankenPHP is shutting down")
}

// a restart drains the sender's thread too: the wait for a pickup ends
// instead of stalling the restart until the timeout
func TestTaskSenderUnblockedOnRestart(t *testing.T) {
	mark := filepath.Join(t.TempDir(), "picked")
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t, frankenphp.WithServer(server), bgWorker("echo", "task-worker.php", nil, server), frankenphp.WithNumThreads(2))

	body := sendWhileWorkerBusy(t, server, mark)
	start := time.Now()
	frankenphp.RestartWorkers()
	assert.WithinDuration(t, start, time.Now(), 10*time.Second, "the restart must not wait for the sender's timeout")
	assert.Contains(t, <-body, "the calling thread is restarting or shutting down")

	// the restarted worker serves tasks again
	assert.Contains(t, serverGet(t, server, "http://example.com/task.php?input=after"), `"result":"processed:after"`)
}

// sendWhileWorkerBusy runs task-shutdown.php in the background: its first
// task keeps the only thread of the worker busy, its second one has no
// timeout; returns the channel carrying the response body once the first
// task was picked up
func sendWhileWorkerBusy(t *testing.T, server *frankenphp.Server, mark string) <-chan string {
	t.Helper()
	body := make(chan string, 1)
	go func() {
		w := httptest.NewRecorder()
		_ = server.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "http://example.com/task-shutdown.php?mark="+url.QueryEscape(mark), nil))
		b, _ := io.ReadAll(w.Result().Body)
		body <- string(b)
	}()
	requireFileEventually(t, mark, "the worker did not pick the first task up")

	return body
}
