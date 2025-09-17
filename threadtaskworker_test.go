package frankenphp

import (
	"bytes"
	"log/slog"
	"net/http/httptest"
	"testing"

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
		),
		WithNumThreads(3),
		WithLogger(logger),
	))
	defer Shutdown()

	pendingTask, err := DispatchTask("go task", "worker")
	assert.NoError(t, err)
	pendingTask.WaitForCompletion()

	logOutput := buf.String()
	assert.Contains(t, logOutput, "go task", "should see the dispatched task in the logs")
	assert.Contains(t, logOutput, "custom var", "should see the prepared env of the task worker")
}

func TestDispatchToTaskWorkerFromWorker(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)

	assert.NoError(t, Init(
		WithWorkers("worker", "./testdata/tasks/task-worker.php", 1, AsTaskWorker(true)),
		WithWorkers("worker", "./testdata/tasks/task-dispatcher.php", 1),
		WithNumThreads(4),
		WithLogger(logger),
	))
	defer Shutdown()

	assert.Len(t, taskWorkers, 1)

	assertGetRequest(t, "http://example.com/testdata/tasks/task-dispatcher.php?count=4", "dispatched 4 tasks")

    // dispatch another task to make sure the previous ones are done
	pr, _ := DispatchTask("go task", "worker")
	pr.WaitForCompletion()

	logOutput := buf.String()
	assert.Contains(t, logOutput, "task0")
	assert.Contains(t, logOutput, "task1")
	assert.Contains(t, logOutput, "task2")
	assert.Contains(t, logOutput, "task3")
}
