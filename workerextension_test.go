package frankenphp

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkerExtension(t *testing.T) {
	readyWorkers := 0
	serverShutDowns := 0

	externalWorker := NewWorker(
		"externalWorker",
		"testdata/worker.php",
		1,
		WithWorkerOnReady(func(id int) {
			readyWorkers++
		}),
		WithWorkerOnServerShutdown(func(id int) {
			serverShutDowns++
		}),
	)
	RegisterWorker(externalWorker)

	// Clean up external workers after test to avoid interfering with other tests
	defer func() {
		delete(extensionWorkers, externalWorker.Name)
	}()

	err := Init()
	require.NoError(t, err)
	defer func() {
		Shutdown()
		assert.Equal(t, 1, serverShutDowns, "Server shutdown hook should have been called")
	}()

	assert.Equal(t, readyWorkers, 1, "Worker thread should have called onReady()")

	// Create a test request
	req := httptest.NewRequest("GET", "https://example.com/test/?foo=bar", nil)
	req.Header.Set("X-Test-Header", "test-value")
	w := httptest.NewRecorder()

	// Inject the request into the worker through the extension
	err = externalWorker.SendRequest(w, req)
	assert.NoError(t, err, "Sending request should not produce an error")

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	// The worker.php script should output information about the request
	// We're just checking that we got a response, not the specific content
	assert.NotEmpty(t, body, "Response body should not be empty")
}

func TestWorkerExtensionSendMessage(t *testing.T) {
	externalWorker := NewWorker("externalWorker", "testdata/message-worker.php", 1)
	RegisterWorker(externalWorker)

	// Clean up external workers after test to avoid interfering with other tests
	defer func() {
		delete(extensionWorkers, externalWorker.Name)
	}()

	err := Init()
	require.NoError(t, err)
	defer Shutdown()

	result, err := externalWorker.SendMessage("Hello Worker", nil)
	assert.NoError(t, err, "Sending message should not produce an error")

	switch v := result.(type) {
	case string:
		assert.Equal(t, "received message: Hello Worker", v)
	default:
		t.Fatalf("Expected result to be string, got %T", v)
	}
}
