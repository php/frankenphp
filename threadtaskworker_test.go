package frankenphp

import (
	"bytes"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func assertGetRequest(t *testing.T, url string, expectedBodyContains string, opts ...RequestOption) {
	t.Helper()
	r := httptest.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()
	req, err := NewRequestWithContext(r, opts...)
	assert.NoError(t, err)
	assert.NoError(t, ServeHTTP(w, req))
	assert.Contains(t, w.Body.String(), expectedBodyContains)
}

func TestDispatchToTaskWorker(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)

	assert.NoError(t, Init(
		WithWorkers(
			"worker",
			"./testdata/tasks/task-worker.php",
			1,
			AsTaskWorker(true),
			WithWorkerEnv(PreparedEnv{"CUSTOM_VAR": "custom var"}),
			WithWorkerArgs([]string{"arg1", "arg2"}),
		),
		WithNumThreads(3),
		WithLogger(logger),
	))
	assert.Len(t, taskWorkers, 1)
	defer func() {
		Shutdown()
		assert.Len(t, taskWorkers[0].threads, 0, "no task-worker threads should remain after shutdown")
	}()

	pendingTask, err := DispatchTask("go task", "worker")
	assert.NoError(t, err)
	pendingTask.WaitForCompletion()

	logOutput := buf.String()
	assert.Contains(t, logOutput, "go task", "should see the dispatched task in the logs")
	assert.Contains(t, logOutput, "custom var", "should see the prepared env of the task worker")
	assert.Contains(t, logOutput, "arg1", "should see args passed to the task worker")
	assert.Contains(t, logOutput, "arg2", "should see args passed to the task worker")
}

func TestDispatchToTaskWorkerFromWorker(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)

	assert.NoError(t, Init(
		WithWorkers("worker", "./testdata/tasks/task-worker.php", 1, AsTaskWorker(true)),
		WithWorkers("worker", "./testdata/tasks/task-dispatcher.php", 1),
		WithNumThreads(3),
		WithLogger(logger),
	))

	assertGetRequest(t, "http://example.com/testdata/tasks/task-dispatcher.php?count=4", "dispatched 4 tasks")

	// wait and shutdown to ensure all logs are flushed
	time.Sleep(10 * time.Millisecond)
	Shutdown()

	// task output appears in logs at info level
	logOutput := buf.String()
	assert.Contains(t, logOutput, "task0")
	assert.Contains(t, logOutput, "task1")
	assert.Contains(t, logOutput, "task2")
	assert.Contains(t, logOutput, "task3")
}

func TestDispatchToMultipleWorkers(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)

	assert.NoError(t, Init(
		WithWorkers("worker1", "./testdata/tasks/task-worker.php", 1, AsTaskWorker(true)),
		WithWorkers("worker2", "./testdata/tasks/task-worker2.php", 1, AsTaskWorker(true)),
		WithNumThreads(4),
		WithLogger(logger),
	))
	defer Shutdown()

	script := "http://example.com/testdata/tasks/task-dispatcher.php"
	assertGetRequest(t, script+"?count=1&worker=worker1", "dispatched 1 tasks")
	assertGetRequest(t, script+"?count=1&worker=worker2", "dispatched 1 tasks")
	assertGetRequest(t, script+"?count=1&worker=worker3", "No worker found to handle the task") // fail
}
