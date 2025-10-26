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

func TestDispatchToTaskWorkerFromWorker(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)

	assert.NoError(t, Init(
		WithWorkers("taskworker", "./testdata/tasks/task-worker.php", 1, AsTaskWorker(true, 0)),
		WithWorkers("worker1", "./testdata/tasks/task-dispatcher-string.php", 1),
		WithNumThreads(3), // regular thread, task worker thread, dispatcher threads
		WithLogger(logger),
	))

	assertGetRequest(t, "http://example.com/testdata/tasks/task-dispatcher-string.php?count=4", "dispatched 4 tasks")

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

func TestDispatchArrayToTaskWorker(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)

	assert.NoError(t, Init(
		WithWorkers("taskworker", "./testdata/tasks/task-worker.php", 1, AsTaskWorker(true, 0)),
		WithWorkers("worker2", "./testdata/tasks/task-dispatcher-array.php", 1),
		WithNumThreads(3), // regular thread, task worker thread, dispatcher thread
		WithLogger(logger),
	))

	assertGetRequest(t, "http://example.com/testdata/tasks/task-dispatcher-array.php?count=1", "dispatched 1 tasks")

	// wait and shutdown to ensure all logs are flushed
	time.Sleep(10 * time.Millisecond)
	Shutdown()

	// task output appears in logs at info level
	logOutput := buf.String()
	assert.Contains(t, logOutput, "array task0")
}

func TestDispatchToMultipleWorkers(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)

	assert.NoError(t, Init(
		WithWorkers("worker1", "./testdata/tasks/task-worker.php", 1, AsTaskWorker(true, 0)),
		WithWorkers("worker2", "./testdata/tasks/task-worker.php", 1, AsTaskWorker(true, 0)),
		WithNumThreads(4),
		WithLogger(logger),
	))
	defer Shutdown()

	script := "http://example.com/testdata/tasks/task-dispatcher-string.php"
	assertGetRequest(t, script+"?count=1&worker=worker1", "dispatched 1 tasks")
	assertGetRequest(t, script+"?count=1&worker=worker2", "dispatched 1 tasks")
	assertGetRequest(t, script+"?count=1&worker=worker3", "No worker found to handle this task") // fail
}
