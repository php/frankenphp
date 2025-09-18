package frankenphp

import (
	"io"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockWorkerExtension implements the WorkerExtension interface
type mockWorkerExtension struct {
	Worker
}

func newMockWorkerExtension(name, fileName string, minThreads int) *mockWorkerExtension {
	return &mockWorkerExtension{
		Worker: Worker{
			ExtensionName:  name,
			WorkerFileName: fileName,
			WorkerEnv:      nil,
			MinThreads:     minThreads,
			RequestChan:    make(chan *WorkerRequest[any, any], minThreads),
			ActivatedCount: atomic.Int32{},
			DrainCount:     atomic.Int32{},
		},
	}
}

func (m *mockWorkerExtension) InjectRequest(r *WorkerRequest[any, any]) {
	m.RequestChan <- r
}

func (m *mockWorkerExtension) GetActivatedCount() int {
	return int(m.ActivatedCount.Load())
}

func TestWorkerExtension(t *testing.T) {
	// Create a mock extension
	mockExt := newMockWorkerExtension("mockWorker", "testdata/worker.php", 1)

	// Register the mock extension
	RegisterExternalWorker(mockExt)

	// Clean up external workers after test to avoid interfering with other tests
	defer func() {
		delete(externalWorkers, mockExt.Name())
	}()

	// Initialize FrankenPHP with a worker that has a different name than our extension
	err := Init()
	require.NoError(t, err)
	defer Shutdown()

	// Wait a bit for the worker to be ready
	time.Sleep(100 * time.Millisecond)

	// Verify that the extension's thread was activated
	assert.GreaterOrEqual(t, mockExt.GetActivatedCount(), 1, "Thread should have been activated")

	// Create a test request
	req := httptest.NewRequest("GET", "http://example.com/test/?foo=bar", nil)
	req.Header.Set("X-Test-Header", "test-value")

	w := httptest.NewRecorder()

	// Create a channel to signal when the request is done
	done := make(chan struct{})

	// Inject the request into the worker through the extension
	mockExt.InjectRequest(&WorkerRequest[any, any]{
		Request:  req,
		Response: w,
		AfterFunc: func(callbackReturn any) {
			close(done)
		},
	})

	// Wait for the request to be fully processed
	<-done

	// Check the response - now safe from race conditions
	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	// The worker.php script should output information about the request
	// We're just checking that we got a response, not the specific content
	assert.NotEmpty(t, body, "Response body should not be empty")
}
