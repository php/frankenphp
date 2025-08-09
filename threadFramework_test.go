package frankenphp_test

import (
	"io"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockWorkerExtension implements the frankenphp.WorkerExtension interface
type mockWorkerExtension struct {
	name             string
	fileName         string
	env              frankenphp.PreparedEnv
	minThreads       int
	requestChan      chan *frankenphp.WorkerRequest
	activatedCount   int
	drainCount       int
	deactivatedCount int
	mu               sync.Mutex
}

func newMockWorkerExtension(name, fileName string, minThreads int) *mockWorkerExtension {
	return &mockWorkerExtension{
		name:        name,
		fileName:    fileName,
		env:         make(frankenphp.PreparedEnv),
		minThreads:  minThreads,
		requestChan: make(chan *frankenphp.WorkerRequest, 10), // Buffer to avoid blocking
	}
}

func (m *mockWorkerExtension) Name() string {
	return m.name
}

func (m *mockWorkerExtension) FileName() string {
	return m.fileName
}

func (m *mockWorkerExtension) Env() frankenphp.PreparedEnv {
	return m.env
}

func (m *mockWorkerExtension) GetMinThreads() int {
	return m.minThreads
}

func (m *mockWorkerExtension) ThreadActivatedNotification(threadId int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activatedCount++
}

func (m *mockWorkerExtension) ThreadDrainNotification(threadId int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.drainCount++
}

func (m *mockWorkerExtension) ThreadDeactivatedNotification(threadId int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deactivatedCount++
}

func (m *mockWorkerExtension) ProvideRequest() *frankenphp.WorkerRequest {
	return <-m.requestChan
}

func (m *mockWorkerExtension) InjectRequest(r *frankenphp.WorkerRequest) {
	m.requestChan <- r
}

func (m *mockWorkerExtension) GetActivatedCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.activatedCount
}

func TestWorkerExtension(t *testing.T) {
	// Create a mock extension
	mockExt := newMockWorkerExtension("mockWorker", "testdata/worker.php", 1)

	// Register the mock extension
	frankenphp.RegisterExternalWorker(mockExt)

	// Initialize FrankenPHP with a worker that has a different name than our extension
	err := frankenphp.Init()
	require.NoError(t, err)
	defer frankenphp.Shutdown()

	// Wait a bit for the worker to be ready
	time.Sleep(100 * time.Millisecond)

	// Verify that the extension's thread was activated
	assert.GreaterOrEqual(t, mockExt.GetActivatedCount(), 1, "Thread should have been activated")

	// Create a test request
	req := httptest.NewRequest("GET", "http://example.com/test/?foo=bar", nil)
	req.Header.Set("X-Test-Header", "test-value")

	w := httptest.NewRecorder()

	// Inject the request into the worker through the extension
	mockExt.InjectRequest(&frankenphp.WorkerRequest{
		Request:  req,
		Response: w,
	})

	// Wait a bit for the request to be processed
	time.Sleep(100 * time.Millisecond)

	// Check the response
	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	// The worker.php script should output information about the request
	// We're just checking that we got a response, not the specific content
	assert.NotEmpty(t, body, "Response body should not be empty")
}
