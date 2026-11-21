package frankenphp_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dunglas/frankenphp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// taskWorker declares a background worker from testdata/bgworker receiving
// the task namespace named like it, scoped to server when one is given
func taskWorker(name, file string, env map[string]string, server *frankenphp.Server, extra ...frankenphp.WorkerOption) frankenphp.Option {
	opts := []frankenphp.WorkerOption{frankenphp.WithWorkerBackground(), frankenphp.WithWorkerEnv(env), frankenphp.WithWorkerTasks(name)}
	if server != nil {
		opts = append(opts, frankenphp.WithWorkerServerScope(server))
	}

	return frankenphp.WithWorkers(name, "testdata/bgworker/"+file, 1, append(opts, extra...)...)
}

func TestTaskRoundTrip(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t, frankenphp.WithServer(server), taskWorker("echo", "task-worker.php", nil, server), frankenphp.WithNumThreads(2))

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
	// worker names are global, the namespace is what the two share
	jobs := func(name, tag string, server *frankenphp.Server) frankenphp.Option {
		return frankenphp.WithWorkers(name, "testdata/bgworker/task-worker.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerTasks("jobs"),
			frankenphp.WithWorkerEnv(map[string]string{"BG_TAG": tag}),
			frankenphp.WithWorkerServerScope(server),
		)
	}
	initServers(t,
		frankenphp.WithServer(server1),
		frankenphp.WithServer(server2),
		jobs("jobs-one", "one", server1),
		jobs("jobs-two", "two", server2),
		frankenphp.WithNumThreads(3),
	)

	assert.Contains(t, serverGet(t, server1, "http://example.com/task.php?ns=jobs"), `"tag":"one"`)
	assert.Contains(t, serverGet(t, server2, "http://example.com/task.php?ns=jobs"), `"tag":"two"`)

	// a name nobody declares may come from the request: the message shows its start
	long := serverGet(t, server1, "http://example.com/task.php?ns="+strings.Repeat("x", 5000))
	assert.Contains(t, long, `the task namespace "`+strings.Repeat("x", 64)+`"...`)
	assert.Less(t, len(long), 300)
}

// between two requests, a worker serving any server still sends its tasks
// to the workers of the one it just served
func TestTaskBetweenTwoRequests(t *testing.T) {
	server1, _ := frankenphp.NewServer(testDataDir, frankenphp.WithServerName("one"))
	server2, _ := frankenphp.NewServer(testDataDir, frankenphp.WithServerName("two"))
	jobs := func(name, tag string, server *frankenphp.Server) frankenphp.Option {
		return frankenphp.WithWorkers(name, "testdata/bgworker/task-worker.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerTasks("jobs"),
			frankenphp.WithWorkerEnv(map[string]string{"BG_TAG": tag}),
			frankenphp.WithWorkerServerScope(server),
		)
	}
	initServers(t,
		frankenphp.WithServer(server1),
		frankenphp.WithServer(server2),
		jobs("jobs-one", "one", server1),
		jobs("jobs-two", "two", server2),
		frankenphp.WithWorkers("sender", testDataDir+"task-between-requests.php", 1),
		frankenphp.WithNumThreads(4),
	)

	url := "http://example.com/task-between-requests.php"
	assert.JSONEq(t, `{"request":"one","after":null}`, serverGet(t, server1, url))
	assert.JSONEq(t, `{"request":"two","after":"one"}`, serverGet(t, server2, url))
	assert.JSONEq(t, `{"request":"one","after":"two"}`, serverGet(t, server1, url))
}

// a background worker may send tasks too, here while booting
func TestTaskFromBackgroundWorker(t *testing.T) {
	sentinel := filepath.Join(t.TempDir(), "relay.json")
	initServers(t,
		taskWorker("relay", "task-relay.php", map[string]string{"BG_TARGET": "echo", "BG_SENTINEL": sentinel}, nil),
		taskWorker("echo", "task-worker.php", nil, nil),
		frankenphp.WithNumThreads(3),
	)

	assert.Contains(t, requireFileContentEventually(t, sentinel), `"result":"processed:relayed"`)
}

// a task waits for a thread to pick it up, up to the timeout: its read()
// tells
func TestTaskPickupTimeout(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t, frankenphp.WithServer(server), taskWorker("echo", "task-worker.php", nil, server), frankenphp.WithNumThreads(2))

	body := serverGet(t, server, "http://example.com/task-busy.php")
	assert.Contains(t, body, `picked up the task sent to "echo" in time`)
	assert.Contains(t, body, `"result":"processed:slow"`)
}

// a worker exiting with a task open fails the sender's read, then restarts
func TestTaskCrashMidTask(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t, frankenphp.WithServer(server), taskWorker("echo", "task-worker.php", nil, server), frankenphp.WithNumThreads(2))

	assert.Contains(t, serverGet(t, server, "http://example.com/task.php?crash=1"), "let go of the task without completing it")
	assert.Contains(t, serverGet(t, server, "http://example.com/task.php?input=after"), `"result":"processed:after"`)
}

// closing the stream abandons the task: the worker sees it on its own stream
func TestTaskAbandoned(t *testing.T) {
	sentinel := filepath.Join(t.TempDir(), "abandoned.txt")
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t, frankenphp.WithServer(server), taskWorker("echo", "task-worker.php", map[string]string{"BG_SENTINEL": sentinel}, server), frankenphp.WithNumThreads(2))

	mark := url.QueryEscape(filepath.Join(t.TempDir(), "picked"))
	assert.Equal(t, "closed", serverGet(t, server, "http://example.com/task.php?sleep_ms=200&close_early=1&mark="+mark))
	assert.Contains(t, requireFileContentEventually(t, sentinel), "the sender closed the task before the update")
}

// a task queued while the only thread is busy reaches it when it reads its
// handle again, even with a loop taking one task per wake-up
func TestTaskQueuedWhileBusy(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t, frankenphp.WithServer(server), taskWorker("echo", "task-worker.php", map[string]string{"BG_LOOP": "if"}, server), frankenphp.WithNumThreads(3))

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
		frankenphp.WithWorkers("pool", "testdata/bgworker/task-worker.php", 2, frankenphp.WithWorkerBackground(), frankenphp.WithWorkerServerScope(server), frankenphp.WithWorkerTasks("pool")),
		frankenphp.WithNumThreads(3),
	)

	assert.Equal(t, "[\"processed:a\",\"processed:b\"]\ntwo threads", serverGet(t, server, "http://example.com/task-pool.php"))
}

// a stream outlives its handle closed: the handle took the task with it
func TestTaskStreamKeptPastItsHandle(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t, frankenphp.WithServer(server), taskWorker("echo", "task-worker.php", nil, server), frankenphp.WithNumThreads(2))

	assert.Equal(t, "closed\nclosed,closed\n", serverGet(t, server, "http://example.com/task-kept-stream.php"))
}

// closing the stream of a task leaves the task alone, on both sides: the
// handle hands out a fresh one and the result still comes
func TestTaskStreamClosedByTheScript(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t, frankenphp.WithServer(server), taskWorker("echo", "task-worker.php", nil, server), frankenphp.WithNumThreads(2))

	assert.Equal(t, "resource (closed), resource (stream)\n1 ready\nprocessed:x\ntrue\n", serverGet(t, server, "http://example.com/task-closed-stream.php"))
}

// read() has no timeout, not even the one a script sets on the stream of the
// task: stream_select() and the poll contexts take one
func TestTaskReadIgnoresTheTimeoutOfTheStream(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t, frankenphp.WithServer(server), taskWorker("echo", "task-worker.php", nil, server), frankenphp.WithNumThreads(2))

	assert.Equal(t, "processed:x\ndone", serverGet(t, server, "http://example.com/task-read-timeout.php"))
}

// a sender queues as many tasks as it wants before reading any, and reads
// them in any order, while the worker streams its updates: neither side
// waits on the other, so neither can hold the other
func TestTaskFanOut(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t, frankenphp.WithServer(server), taskWorker("echo", "task-worker.php", nil, server), frankenphp.WithNumThreads(2))

	assert.Equal(t, "20 tasks, 420 updates", serverGet(t, server, "http://example.com/task-fanout.php?n=20&steps=20"))
	assert.Equal(t, "20 tasks, 420 updates", serverGet(t, server, "http://example.com/task-fanout.php?n=20&steps=20&order=reverse"))
}

// only complete() completes a task: a worker letting go of it otherwise
// makes the sender's read() throw, as a crash does
func TestTaskDroppedWithoutComplete(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t, frankenphp.WithServer(server), taskWorker("echo", "task-worker.php", nil, server), frankenphp.WithNumThreads(2))

	assert.Equal(t, `RuntimeException: FrankenPHP\SentTaskHandle::read(): the background worker let go of the task without completing it`, serverGet(t, server, "http://example.com/task.php?drop=1"))
	assert.Contains(t, serverGet(t, server, "http://example.com/task.php?input=next"), `"result":"processed:next"`)

	// abort() says it at once, with the handle still held
	assert.Equal(t, `RuntimeException: FrankenPHP\SentTaskHandle::read(): the background worker let go of the task without completing it`, serverGet(t, server, "http://example.com/task.php?abort=1"))
	assert.Contains(t, serverGet(t, server, "http://example.com/task.php?input=again"), `"result":"processed:again"`)
}

// a worker learns from update() that its sender gave up, nothing throws
func TestTaskUpdateAfterTheSenderLeft(t *testing.T) {
	sentinel := filepath.Join(t.TempDir(), "error")
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t, frankenphp.WithServer(server), taskWorker("echo", "task-worker.php", map[string]string{"BG_SENTINEL": sentinel}, server), frankenphp.WithNumThreads(2))

	mark := url.QueryEscape(filepath.Join(t.TempDir(), "picked"))
	assert.Equal(t, "closed", serverGet(t, server, "http://example.com/task.php?steps=2&pause_ms=200&close_early=1&mark="+mark))
	assert.Equal(t, "RuntimeException: update() returned false at step 1", requireFileContentEventually(t, sentinel))
}

func TestTaskErrors(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t, frankenphp.WithServer(server), taskWorker("echo", "task-worker.php", nil, server), frankenphp.WithNumThreads(2))

	body := serverGet(t, server, "http://example.com/task-errors.php")
	assert.Contains(t, body, `unknown: RuntimeException: FrankenPHP\SentTaskHandle: no background worker receives the task namespace "nope"`)
	assert.Contains(t, body, `payload: ValueError: FrankenPHP\SentTaskHandle::__construct(): payload values must be null, scalars, arrays or enums`)
	assert.Contains(t, body, `timeout: ValueError: FrankenPHP\SentTaskHandle::__construct(): Argument #3 ($pickupTimeout) must be greater than or equal to 0`)
	assert.Contains(t, body, `big: ValueError: FrankenPHP\SentTaskHandle::__construct(): payload values must fit in memory_limit once copied`)
	assert.Contains(t, body, `received: Error: Call to private FrankenPHP\ReceivedTaskHandle::__construct()`)
	assert.Contains(t, body, `worker: LogicException: FrankenPHP\WorkerHandle can only be created from a background worker`)
}

// the metrics of a background worker follow its tasks: busy while a thread
// holds one, queued while nobody picked it up, counted by outcome
func TestTaskMetrics(t *testing.T) {
	registry := prometheus.NewRegistry()
	sentinel := filepath.Join(t.TempDir(), "abandoned.txt")
	server, err := frankenphp.NewServer(testDataDir, frankenphp.WithServerName("api"))
	require.NoError(t, err)
	initServers(t,
		frankenphp.WithServer(server),
		taskWorker("echo", "task-worker.php", map[string]string{"BG_SENTINEL": sentinel}, server),
		frankenphp.WithNumThreads(2),
		frankenphp.WithMetrics(frankenphp.NewPrometheusMetrics(registry)),
	)

	assert.Contains(t, serverGet(t, server, "http://example.com/task.php?input=done"), `"result":"processed:done"`)
	assert.Contains(t, serverGet(t, server, "http://example.com/task.php?crash=1"), "let go of the task without completing it")
	mark := url.QueryEscape(filepath.Join(t.TempDir(), "picked"))
	assert.Equal(t, "closed", serverGet(t, server, "http://example.com/task.php?sleep_ms=200&close_early=1&mark="+mark))
	requireFileContentEventually(t, sentinel)
	assert.Contains(t, serverGet(t, server, "http://example.com/task-busy.php"), `picked up the task sent to "echo" in time`)

	expected := `
		# HELP frankenphp_worker_task_count Number of tasks sent to this background worker, by outcome: completed, aborted (the worker let go of the task without completing it), abandoned (the sender gave up on the task first), timeout (no thread picked the task up in time) or cancelled (the wait for a thread ended on a restart or the shutdown)
		# TYPE frankenphp_worker_task_count counter
		frankenphp_worker_task_count{outcome="abandoned",worker="echo"} 1
		frankenphp_worker_task_count{outcome="aborted",worker="echo"} 1
		frankenphp_worker_task_count{outcome="completed",worker="echo"} 2
		frankenphp_worker_task_count{outcome="timeout",worker="echo"} 1
		# HELP frankenphp_busy_workers Number of busy PHP workers for this worker: processing a request, or a task for a background worker
		# TYPE frankenphp_busy_workers gauge
		frankenphp_busy_workers{worker="echo"} 0
		# HELP frankenphp_worker_queue_depth Number of queued requests for this worker, or of tasks waiting for a thread of a background worker
		# TYPE frankenphp_worker_queue_depth gauge
		frankenphp_worker_queue_depth{worker="echo"} 0
	`
	// the abandoned task is closed by the worker after the response
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.NoError(c, testutil.GatherAndCompare(registry, strings.NewReader(expected), "frankenphp_worker_task_count", "frankenphp_busy_workers", "frankenphp_worker_queue_depth"))
	}, 5*time.Second, 25*time.Millisecond)
}

// complete() with a result is one event for the sender: one leaving right
// after it left a completed task, whatever the two threads do next
func TestTaskResultThenLeave(t *testing.T) {
	registry := prometheus.NewRegistry()
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t,
		frankenphp.WithServer(server),
		taskWorker("echo", "task-worker.php", nil, server),
		frankenphp.WithNumThreads(2),
		frankenphp.WithMetrics(frankenphp.NewPrometheusMetrics(registry)),
	)

	assert.Equal(t, "processed:1999", serverGet(t, server, "http://example.com/task-result.php?n=2000"))

	expected := `
		# HELP frankenphp_worker_task_count Number of tasks sent to this background worker, by outcome: completed, aborted (the worker let go of the task without completing it), abandoned (the sender gave up on the task first), timeout (no thread picked the task up in time) or cancelled (the wait for a thread ended on a restart or the shutdown)
		# TYPE frankenphp_worker_task_count counter
		frankenphp_worker_task_count{outcome="completed",worker="echo"} 2000
	`
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.NoError(c, testutil.GatherAndCompare(registry, strings.NewReader(expected), "frankenphp_worker_task_count"))
	}, 5*time.Second, 25*time.Millisecond)
}

// a sender waiting for a busy worker to pick its task up is released by
// Shutdown() instead of holding it
func TestTaskSenderUnblockedOnShutdown(t *testing.T) {
	mark := filepath.Join(t.TempDir(), "picked")
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	t.Cleanup(frankenphp.Shutdown)
	require.NoError(t, frankenphp.Init(frankenphp.WithServer(server), taskWorker("echo", "task-worker.php", nil, server), frankenphp.WithNumThreads(2)))

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
	registry := prometheus.NewRegistry()
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t,
		frankenphp.WithServer(server),
		taskWorker("echo", "task-worker.php", nil, server),
		frankenphp.WithNumThreads(2),
		frankenphp.WithMetrics(frankenphp.NewPrometheusMetrics(registry)),
	)

	body := sendWhileWorkerBusy(t, server, mark)
	start := time.Now()
	frankenphp.RestartWorkers()
	assert.WithinDuration(t, start, time.Now(), 10*time.Second, "the restart must not wait for the sender's timeout")
	assert.Contains(t, <-body, "the calling thread is restarting or shutting down")
	assert.Equal(t, 1.0, taskOutcomes(t, registry)["cancelled"], "the task the restart ended was not counted")

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

// what a handle does once the task is over, on both sides
func TestTaskMethodsAfterTheEnd(t *testing.T) {
	sentinel := filepath.Join(t.TempDir(), "over.txt")
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t, frankenphp.WithServer(server),
		taskWorker("echo", "task-over-worker.php", map[string]string{"BG_SENTINEL": sentinel}, server),
		frankenphp.WithNumThreads(2))

	// the task being complete does not close the sender's handle, giving up
	// on it does, and anything that needs the other side then throws
	assert.Equal(t, `completed read: ok
completed getStream: resource (stream)
completed abandon: ok
abandoned read: LogicException: the task is over
abandoned getStream: resource (closed)
abandoned abandon: ok
`, serverGet(t, server, "http://example.com/task-over.php"))

	// the payload outlives the task, closing twice is free, and a stream
	// the script never took is not built past the end of the task
	assert.Equal(t, `getPayload: {"input":"over"}
update: LogicException: the task is over
complete: false
complete-data: LogicException: the task is over
getStream: LogicException: the task is over`, requireFileContentEventually(t, sentinel))
}

// two tasks followed through an Io\Poll\Context: the handles implement
// Io\Poll\Handle, so the script never touches a stream
func TestTaskPoll(t *testing.T) {
	if frankenphp.Version().VersionID < 80600 {
		t.Skip("the poll API needs PHP 8.6")
	}

	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t,
		frankenphp.WithServer(server),
		frankenphp.WithWorkers("pool", "testdata/bgworker/task-worker.php", 2, frankenphp.WithWorkerBackground(), frankenphp.WithWorkerServerScope(server), frankenphp.WithWorkerTasks("pool")),
		frankenphp.WithNumThreads(3),
	)

	assert.Equal(t, `["processed:a","processed:b"] 0`, serverGet(t, server, "http://example.com/task-poll.php"))
}

// the end of the task's stream comes after its updates, not ahead of them
func TestTaskStreamEndsBehindItsUpdates(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t, frankenphp.WithServer(server), taskWorker("echo", "task-worker.php", nil, server), frankenphp.WithNumThreads(2))

	assert.JSONEq(t, `{"before":false,"updates":3,"after":true}`, serverGet(t, server, "http://example.com/task-feof.php"))
}

// a context still watching a task that is over hears nothing of the tasks
// that come next
func TestTaskWatcherLeftBehind(t *testing.T) {
	if frankenphp.Version().VersionID < 80600 {
		t.Skip("the poll API needs PHP 8.6")
	}

	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t, frankenphp.WithServer(server), taskWorker("echo", "task-worker.php", nil, server), frankenphp.WithNumThreads(2))

	assert.Equal(t, "[] processed:next", serverGet(t, server, "http://example.com/task-stale-watcher.php"))
	// and so does one left on a task abandoned with an update unread, which
	// its worker still holds
	assert.Equal(t, "1 ready, then 0", serverGet(t, server, "http://example.com/task-stale-update.php"))
}

// a poller that keeps the descriptor of a task's stream is told of every
// update, not only of the first one
func TestTaskStreamInAPoller(t *testing.T) {
	if frankenphp.Version().VersionID < 80600 {
		t.Skip("the poll API needs PHP 8.6")
	}

	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t, frankenphp.WithServer(server), taskWorker("echo", "task-worker.php", nil, server), frankenphp.WithNumThreads(2))

	assert.Equal(t, "done after 5 events", serverGet(t, server, "http://example.com/task-stream-poll.php"))
}

// one pool receives several task namespaces through one queue, and each task
// tells the namespace it was sent to
func TestTaskNamespaces(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t,
		frankenphp.WithServer(server),
		frankenphp.WithWorkers("images", "testdata/bgworker/task-worker.php", 2,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerServerScope(server),
			frankenphp.WithWorkerTasks("image.resize", "image.thumbnail"),
		),
		frankenphp.WithNumThreads(3),
	)

	resize := serverGet(t, server, "http://example.com/task.php?ns=image.resize&input=a")
	assert.Contains(t, resize, `"namespace":"image.resize"`)
	assert.Contains(t, resize, `"worker":"images"`)
	assert.Contains(t, serverGet(t, server, "http://example.com/task.php?ns=image.thumbnail&input=b"), `"namespace":"image.thumbnail"`)
	assert.Contains(t, serverGet(t, server, "http://example.com/task.php?ns=images"), "no background worker receives")
}

func TestValidateTaskNamespaces(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	receiver := func(name string, extra ...frankenphp.WorkerOption) frankenphp.Option {
		return frankenphp.WithWorkers(name, "testdata/bgworker/task-worker.php", 1, append([]frankenphp.WorkerOption{frankenphp.WithWorkerBackground()}, extra...)...)
	}

	assert.ErrorContains(t, frankenphp.Validate(
		receiver("one", frankenphp.WithWorkerTasks("jobs")),
		receiver("two", frankenphp.WithWorkerTasks("mail", "jobs")),
	), `two global workers cannot receive the same task namespace: "jobs"`)

	assert.NoError(t, frankenphp.Validate(
		frankenphp.WithServer(server),
		receiver("one", frankenphp.WithWorkerTasks("jobs")),
		receiver("two", frankenphp.WithWorkerTasks("jobs"), frankenphp.WithWorkerServerScope(server)),
	))

	// vars and tasks are separate addresses, one worker may hold both
	assert.NoError(t, frankenphp.Validate(
		receiver("one", frankenphp.WithWorkerTasks("jobs"), frankenphp.WithWorkerVars("jobs")),
	))

	assert.ErrorContains(t, frankenphp.Validate(
		receiver("one", frankenphp.WithWorkerTasks("jobs", "jobs")),
	), `declares the task namespace "jobs" twice`)

	assert.ErrorContains(t, frankenphp.Validate(
		frankenphp.WithWorkers("http", "testdata/worker.php", 1, frankenphp.WithWorkerTasks("jobs")),
	), "only background workers receive them")
}

// taskOutcomes sums frankenphp_worker_task_count by outcome
func taskOutcomes(t *testing.T, registry *prometheus.Registry) map[string]float64 {
	t.Helper()
	families, err := registry.Gather()
	require.NoError(t, err)

	outcomes := map[string]float64{}
	for _, family := range families {
		if family.GetName() != "frankenphp_worker_task_count" {
			continue
		}
		for _, metric := range family.GetMetric() {
			for _, label := range metric.GetLabel() {
				if label.GetName() == "outcome" {
					outcomes[label.GetValue()] += metric.GetCounter().GetValue()
				}
			}
		}
	}

	return outcomes
}

// a handle sends one task: constructing it again throws, the first task goes on
func TestTaskHandleIsConstructedOnce(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t, frankenphp.WithServer(server), taskWorker("echo", "task-worker.php", nil, server), frankenphp.WithNumThreads(2))

	body := serverGet(t, server, "http://example.com/task-reconstruct.php")
	assert.Contains(t, body, `Error: FrankenPHP\SentTaskHandle::__construct() cannot be called twice`)
	assert.Contains(t, body, `"result":"processed:first"`)
}

// a server without a receiver of its own sends to the global one
func TestTaskFallsBackToAGlobalReceiver(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t, frankenphp.WithServer(server), taskWorker("echo", "task-worker.php", map[string]string{"BG_TAG": "global"}, nil), frankenphp.WithNumThreads(2))

	assert.Contains(t, serverGet(t, server, "http://example.com/task.php?input=x"), `"tag":"global"`)
}

// a worker with nothing else to wait on waits for its tasks in tick(): each
// one wakes it there, and so does the drain
func TestTaskTickWaits(t *testing.T) {
	sentinel := filepath.Join(t.TempDir(), "drained.txt")
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	t.Cleanup(frankenphp.Shutdown)
	require.NoError(t, frankenphp.Init(
		frankenphp.WithServer(server),
		taskWorker("echo", "task-wait-worker.php", map[string]string{"BG_SENTINEL": sentinel}, server),
		frankenphp.WithNumThreads(2),
	))

	assert.Contains(t, serverGet(t, server, "http://example.com/task.php?input=hello"), `"result":"processed:hello"`)
	// the thread sleeps between two tasks, and takes the next one
	time.Sleep(50 * time.Millisecond)
	assert.Contains(t, serverGet(t, server, "http://example.com/task.php?input=again"), `"result":"processed:again"`)

	done := make(chan struct{})
	go func() {
		frankenphp.Shutdown()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Shutdown did not end the wait in tick() within 10s")
	}
	// no wait returned but for a task, the drain ended the last one
	assert.Equal(t, "drained after 0 waits without a task\n", requireFileContentEventually(t, sentinel))
}

// with a timeout, tick() hands the loop back when no task comes
func TestTaskTickTimesOut(t *testing.T) {
	const timeout = 20 * time.Millisecond
	start := time.Now()
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t,
		frankenphp.WithServer(server),
		taskWorker("echo", "task-wait-worker.php", map[string]string{"BG_TICK_TIMEOUT": "0.02"}, server),
		frankenphp.WithNumThreads(2),
	)

	// each task is answered with the number of waits that ended without one
	var answer struct {
		Result string `json:"result"`
		Idle   int    `json:"idle"`
	}
	require.Eventually(t, func() bool {
		body := serverGet(t, server, "http://example.com/task.php?input=late")

		return json.Unmarshal([]byte(strings.SplitN(body, "\n", 2)[0]), &answer) == nil && answer.Idle >= 3
	}, 10*time.Second, 50*time.Millisecond, "tick() did not time out while no task came")
	assert.Equal(t, "processed:late", answer.Result)
	// no wait ends before its timeout: one at most per timeout elapsed
	assert.LessOrEqual(t, answer.Idle, int(time.Since(start)/timeout)+1, "tick() returned before its timeout")
}

// the threads of a pool waiting in tick() share the queue like those waiting
// on their streams, each task waking one of them
func TestTaskTickWaitsInAPool(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t,
		frankenphp.WithServer(server),
		frankenphp.WithWorkers("pool", "testdata/bgworker/task-wait-worker.php", 2, frankenphp.WithWorkerBackground(), frankenphp.WithWorkerServerScope(server), frankenphp.WithWorkerTasks("pool")),
		frankenphp.WithNumThreads(3),
	)

	assert.Equal(t, "[\"processed:a\",\"processed:b\"]\ntwo threads", serverGet(t, server, "http://example.com/task-pool.php"))
	// and again, once both threads went back to sleep
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, "[\"processed:a\",\"processed:b\"]\ntwo threads", serverGet(t, server, "http://example.com/task-pool.php"))
}

// queueRequest runs a scenario of task-queue.php against a worker of one
// thread, which the script keeps busy first
func queueRequest(t *testing.T, scenario string, registry *prometheus.Registry) string {
	t.Helper()
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	options := []frankenphp.Option{frankenphp.WithServer(server), taskWorker("echo", "task-worker.php", nil, server), frankenphp.WithNumThreads(2)}
	if registry != nil {
		options = append(options, frankenphp.WithMetrics(frankenphp.NewPrometheusMetrics(registry)))
	}
	initServers(t, options...)

	dir := t.TempDir()
	query := url.Values{"mark": {filepath.Join(dir, "picked")}, "never": {filepath.Join(dir, "never")}}

	return serverGet(t, server, "http://example.com/task-queue.php?case="+scenario+"&"+query.Encode())
}

// a task abandoned before a thread picked it up leaves the queue: the worker
// never sees it
func TestTaskAbandonedWhileQueued(t *testing.T) {
	registry := prometheus.NewRegistry()
	assert.Equal(t, "processed:after\nnever processed", queueRequest(t, "abandon", registry))

	outcomes := taskOutcomes(t, registry)
	assert.Equal(t, 1.0, outcomes["abandoned"])
	assert.Equal(t, 2.0, outcomes["completed"])
}

// the timeout for a pickup ends the wait of a sender on its stream too
func TestTaskPickupTimeoutOnAStream(t *testing.T) {
	assert.Equal(t, "1 ready at the timeout\nFrankenPHP\\SentTaskHandle: no thread picked up the task sent to \"echo\" in time", queueRequest(t, "select", nil))
}

// a refusal stands: the task left the queue when its time was up, whether its
// sender waited on it or not, so a read() called later throws at once,
// whatever became of the worker meanwhile
func TestTaskRefusalIsFinal(t *testing.T) {
	for _, scenario := range []string{"refused", "unwatched"} {
		t.Run(scenario, func(t *testing.T) {
			registry := prometheus.NewRegistry()
			assert.Equal(t, "at once: FrankenPHP\\SentTaskHandle: no thread picked up the task sent to \"echo\" in time\nnever processed", queueRequest(t, scenario, registry))

			outcomes := taskOutcomes(t, registry)
			assert.Equal(t, 1.0, outcomes["timeout"])
			assert.Equal(t, 1.0, outcomes["completed"])
		})
	}
}

// each task wakes a thread by itself, whatever the shape of the worker's loop:
// waiting on its stream, ticking once woken as the callback of an event loop
// does, which ends each wake-up on a receive() returning null, or waiting in
// tick(). The watcher of the queue wakes every thread for a task that waits,
// but only at its next look: most round trips are over long before
func TestTaskWakesAThreadByItself(t *testing.T) {
	loops := map[string]frankenphp.Option{
		"stream":   taskWorker("echo", "task-worker.php", nil, nil),
		"callback": taskWorker("echo", "task-worker.php", map[string]string{"BG_LOOP": "callback"}, nil),
		"tick":     taskWorker("echo", "task-wait-worker.php", nil, nil),
	}
	for loop, worker := range loops {
		t.Run(loop, func(t *testing.T) {
			server, err := frankenphp.NewServer(testDataDir)
			require.NoError(t, err)
			initServers(t, frankenphp.WithServer(server), worker, frankenphp.WithNumThreads(2))

			median, err := strconv.Atoi(serverGet(t, server, "http://example.com/task-latency.php"))
			require.NoError(t, err)
			assert.Less(t, median, 5000, "the tasks waited for the watcher of the queue to wake the worker")
		})
	}
}
