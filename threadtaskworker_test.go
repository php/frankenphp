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

func TestDispatchWorkToTaskWorker(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)

	assert.NoError(t, Init(
		WithWorkers("worker", "./testdata/tasks/task-worker.php", 1, AsTaskWorker(true)),
		WithNumThreads(3),
		WithLogger(logger),
	))
	defer Shutdown()

	assert.Len(t, taskWorkers, 1)

	assertGetRequest(t, "http://example.com/testdata/tasks/task-dispatcher.php?count=4", "dispatched 4 tasks")

	time.Sleep(time.Millisecond * 200) // wait a bit for tasks to complete

	logOutput := buf.String()
	assert.Contains(t, logOutput, "task0")
	assert.Contains(t, logOutput, "task1")
	assert.Contains(t, logOutput, "task2")
	assert.Contains(t, logOutput, "task3")
}
