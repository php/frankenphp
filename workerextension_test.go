package frankenphp

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkersExtension(t *testing.T) {
	readyWorkers := 0
	shutdownWorkers := 0
	serverStarts := 0
	serverShutDowns := 0

	externalWorkers, o := WithExtensionWorkers(
		"extensionWorkers",
		"testdata/worker.php",
		1,
		WithWorkerOnReady(func(id int) {
			readyWorkers++
		}),
		WithWorkerOnShutdown(func(id int) {
			serverShutDowns++
		}),
		WithWorkerOnServerStartup(func() {
			serverStarts++
		}),
		WithWorkerOnServerShutdown(func() {
			shutdownWorkers++
		}),
	)

	require.NoError(t, Init(o))
	t.Cleanup(func() {
		Shutdown()
		assert.Equal(t, 1, shutdownWorkers, "Worker shutdown hook should have been called")
		assert.Equal(t, 1, serverShutDowns, "Server shutdown hook should have been called")
	})

	assert.Equal(t, readyWorkers, 1, "Worker thread should have called onReady()")
	assert.Equal(t, serverStarts, 1, "Server start hook should have been called")
	assert.Equal(t, externalWorkers.NumThreads(), 1, "NumThreads() should report 1 thread")

	// Create a test request
	req := httptest.NewRequest("GET", "https://example.com/test/?foo=bar", nil)
	req.Header.Set("X-Test-Header", "test-value")
	w := httptest.NewRecorder()

	// Inject the request into the worker through the extension
	err := externalWorkers.SendRequest(w, req)
	assert.NoError(t, err, "Sending request should not produce an error")

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	// The worker.php script should output information about the request
	// We're just checking that we got a response, not the specific content
	assert.NotEmpty(t, body, "Response body should not be empty")
	assert.Contains(t, string(body), "Requests handled: 0", "Response body should contain request information")
}

func TestWorkerExtensionSendMessage(t *testing.T) {
	externalWorker, o := WithExtensionWorkers("extensionWorkers", "testdata/message-worker.php", 1)

	err := Init(o)
	require.NoError(t, err)
	t.Cleanup(Shutdown)

	assert.Equal(t, "received message: Hello Workers", externalWorker.SendMessage("Hello Workers", nil))
}

func TestErrorIf2WorkersHaveSameName(t *testing.T) {
	_, o1 := WithExtensionWorkers("duplicateWorker", "testdata/worker.php", 1)
	_, o2 := WithExtensionWorkers("duplicateWorker", "testdata/worker2.php", 1)

	t.Cleanup(Shutdown)
	require.Error(t, Init(o1, o2))
}
