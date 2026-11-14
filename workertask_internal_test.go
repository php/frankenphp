package frankenphp

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// a force-kill ends the wait of a sender in read(), as max_execution_time
// does: the thread sleeps on its mailbox there, which the signal of the kill
// gets it out of, or the pace at which the ports without one look up
func TestTaskReadEndsOnAForceKill(t *testing.T) {
	mark := filepath.Join(t.TempDir(), "picked")
	server, err := NewServer(testDataPath)
	require.NoError(t, err)
	require.NoError(t, Init(
		WithServer(server),
		WithWorkers("echo", filepath.Join(testDataPath, "bgworker", "task-worker.php"), 1,
			WithWorkerBackground(),
			WithWorkerServerScope(server),
			WithWorkerTasks("echo"),
		),
		WithNumThreads(2),
	))
	t.Cleanup(Shutdown)

	// the worker holds the task for 10 seconds, unless its sender leaves
	body := make(chan string, 1)
	go func() {
		w := httptest.NewRecorder()
		_ = server.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "http://example.com/task.php?input=slow&sleep_ms=10000&mark="+url.QueryEscape(mark), nil))
		b, _ := io.ReadAll(w.Result().Body)
		body <- string(b)
	}()
	require.Eventually(t, func() bool {
		_, err := os.Stat(mark)

		return err == nil
	}, 5*time.Second, 10*time.Millisecond, "the worker did not pick the task up")
	// the sender is in its read() by now
	time.Sleep(100 * time.Millisecond)

	killed := 0
	for _, thread := range phpThreads {
		thread.handlerMu.RLock()
		_, regular := thread.handler.(*regularThread)
		thread.handlerMu.RUnlock()
		if regular && !thread.state.IsInWaitingState() {
			thread.sendKillSignal()
			killed++
		}
	}
	require.Equal(t, 1, killed, "the thread of the request was not found")

	select {
	case b := <-body:
		assert.NotContains(t, b, "processed:slow")
	case <-time.After(5 * time.Second):
		t.Fatal("the force-kill did not end the read()")
	}
}
