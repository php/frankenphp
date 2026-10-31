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

// a sender waits for a thread to pick its task up, up to the timeout
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

	assert.Contains(t, serverGet(t, server, "http://example.com/task.php?crash=1"), "exited without completing the task")
	assert.Contains(t, serverGet(t, server, "http://example.com/task.php?input=after"), `"result":"processed:after"`)
}

// closing the stream abandons the task: the worker sees it on its own stream
func TestTaskAbandoned(t *testing.T) {
	sentinel := filepath.Join(t.TempDir(), "abandoned.txt")
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t, frankenphp.WithServer(server), taskWorker("echo", "task-worker.php", map[string]string{"BG_SENTINEL": sentinel}, server), frankenphp.WithNumThreads(2))

	assert.Equal(t, "closed", serverGet(t, server, "http://example.com/task.php?sleep_ms=200&close_early=1"))
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

func TestTaskErrors(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t, frankenphp.WithServer(server), taskWorker("echo", "task-worker.php", nil, server), frankenphp.WithNumThreads(2))

	body := serverGet(t, server, "http://example.com/task-errors.php")
	assert.Contains(t, body, `unknown: RuntimeException: FrankenPHP\SentTaskHandle: no background worker receives the task namespace "nope"`)
	assert.Contains(t, body, `payload: ValueError: FrankenPHP\SentTaskHandle::__construct(): payload values must be null, scalars, arrays or enums`)
	assert.Contains(t, body, `timeout: ValueError: FrankenPHP\SentTaskHandle::__construct(): Argument #3 ($timeout) must be greater than or equal to 0`)
	assert.Contains(t, body, `received: Error: Call to private FrankenPHP\ReceivedTaskHandle::__construct()`)
	assert.Contains(t, body, `worker: RuntimeException: FrankenPHP\WorkerHandle can only be created from a background worker`)
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
	assert.Contains(t, serverGet(t, server, "http://example.com/task.php?crash=1"), "exited without completing the task")
	assert.Equal(t, "closed", serverGet(t, server, "http://example.com/task.php?sleep_ms=200&close_early=1"))
	requireFileContentEventually(t, sentinel)
	assert.Contains(t, serverGet(t, server, "http://example.com/task-busy.php"), `picked up the task sent to "echo" in time`)

	expected := `
		# HELP frankenphp_worker_task_count Number of tasks sent to this background worker, by outcome: completed, aborted (the script ended with the task open), abandoned (the sender closed its stream first), timeout (no thread picked the task up in time) or cancelled (the wait for a thread ended on a restart or the shutdown)
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
abandoned read: RuntimeException: the task is over
abandoned getStream: resource (closed)
abandoned abandon: ok
`, serverGet(t, server, "http://example.com/task-over.php"))

	// the payload outlives the task, closing twice is free, and a stream
	// the script never took is not built past the end of the task
	assert.Equal(t, `getPayload: {"input":"over"}
update: RuntimeException: the task is over
complete: ok
complete-data: RuntimeException: the task is over
getStream: RuntimeException: the task is over`, requireFileContentEventually(t, sentinel))
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
