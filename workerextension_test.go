package frankenphp

import (
	"context"
	"io"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkersExtension(t *testing.T) {
	t.Cleanup(Shutdown)

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

	ret, err := externalWorker.SendMessage(t.Context(), "Hello Workers", nil)
	require.NoError(t, err)

	assert.Equal(t, "received message: Hello Workers", ret)
}

// an extension worker without a name is still the one SendRequest()
// dispatches on
func TestWorkerExtensionSendRequestWithoutName(t *testing.T) {
	t.Cleanup(Shutdown)

	externalWorker, o := WithExtensionWorkers("", "testdata/worker.php", 1)
	require.NoError(t, Init(o))

	w := httptest.NewRecorder()
	require.NoError(t, externalWorker.SendRequest(w, httptest.NewRequest("GET", "http://example.com/index.php", nil)))

	body, err := io.ReadAll(w.Result().Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "Requests handled: 0")
}

// SendMessage() after Shutdown() reports the stopped runtime instead of
// waiting for a thread
func TestWorkerExtensionSendMessageAfterShutdown(t *testing.T) {
	externalWorker, o := WithExtensionWorkers("extensionWorkers", "testdata/message-worker.php", 1)
	require.NoError(t, Init(o))
	Shutdown()

	errChan := make(chan error, 1)
	go func() {
		_, err := externalWorker.SendMessage(t.Context(), "Hello Workers", nil)
		errChan <- err
	}()

	select {
	case err := <-errChan:
		require.ErrorIs(t, err, ErrNotRunning)
	case <-time.After(2 * time.Second):
		t.Fatal("SendMessage() did not return after Shutdown()")
	}
}

func TestWorkerExtensionSendMessageCopiesZval(t *testing.T) {
	externalWorker, o := WithExtensionWorkers("extensionWorkers", "testdata/message-worker.php", 1)

	require.NoError(t, Init(o, WithNumThreads(1), WithMaxThreads(3)))
	t.Cleanup(Shutdown)

	task := newTask(func() {
		zval := PHPValue("Hello Workers")

		ret, err := externalWorker.SendMessage(t.Context(), zval, nil)
		assert.NoError(t, err)
		assert.Equal(t, "received message: Hello Workers", ret)

		message, err := GoValue[string](zval)
		assert.NoError(t, err)
		assert.Equal(t, "Hello Workers", message, "the zval should still be owned by the calling thread")

		ret, err = externalWorker.SendMessage(t.Context(), PHPValue(nil), nil)
		assert.NoError(t, err)
		assert.Equal(t, "received message: ", ret, "a null zval should be passed to the closure")
	})
	convertToTaskThread(getInactivePHPThread()).execute(task)
	task.waitForCompletion()
}

func TestWorkerExtensionSendMessageConvertsArrays(t *testing.T) {
	externalWorker, o := WithExtensionWorkers("extensionWorkers", "testdata/message-export-worker.php", 1)

	require.NoError(t, Init(o, WithNumThreads(1), WithMaxThreads(3)))
	t.Cleanup(Shutdown)

	message := AssociativeArray[any]{
		Map:   map[string]any{"list": []any{"a", int64(1), nil}, "float": 1.5, "bool": true},
		Order: []string{"list", "float", "bool"},
	}

	task := newTask(func() {
		ret, err := externalWorker.SendMessage(t.Context(), PHPValue(message), nil)
		assert.NoError(t, err)
		assert.Equal(t, "array (\n  'list' => \n  array (\n    0 => 'a',\n    1 => 1,\n    2 => NULL,\n  ),\n  'float' => 1.5,\n  'bool' => true,\n)", ret)
	})
	convertToTaskThread(getInactivePHPThread()).execute(task)
	task.waitForCompletion()
}

func TestWorkerExtensionSendMessageRejectsUnsupportedZval(t *testing.T) {
	externalWorker, o := WithExtensionWorkers("extensionWorkers", "testdata/message-worker.php", 1)

	require.NoError(t, Init(o))
	t.Cleanup(Shutdown)

	// only the type of the zval is read: IS_OBJECT (8) in u1.type_info
	var zval [2]uint64
	zval[1] = 8

	ret, err := externalWorker.SendMessage(t.Context(), unsafe.Pointer(&zval), nil)
	assert.EqualError(t, err, "cannot convert message: unsupported zval type 8")
	assert.Nil(t, ret)
}

// background workers never read requestChan, so an extension cannot send
// them requests or messages
func TestErrorIfExtensionWorkerIsBackground(t *testing.T) {
	t.Cleanup(Shutdown)

	_, o := WithExtensionWorkers("backgroundExtension", "testdata/bgworker/basic.php", 1, WithWorkerBackground())

	require.ErrorContains(t, Init(o), "cannot be an extension worker")
}

func TestErrorIf2WorkersHaveSameName(t *testing.T) {
	_, o1 := WithExtensionWorkers("duplicateWorker", "testdata/worker.php", 1)
	_, o2 := WithExtensionWorkers("duplicateWorker", "testdata/worker2.php", 1)

	t.Cleanup(Shutdown)
	require.Error(t, Init(o1, o2))
}

func TestWorkerExtensionSendMessageOnServerShutdown(t *testing.T) {
	var (
		ret any
		err error
	)

	var externalWorker Workers
	externalWorker, o := WithExtensionWorkers("extensionWorkers", "testdata/message-worker.php", 1, WithWorkerOnServerShutdown(func() {
		ret, err = externalWorker.SendMessage(context.Background(), "Bye Workers", nil)
	}))

	require.NoError(t, Init(o))
	Shutdown()

	require.NoError(t, err)
	assert.Equal(t, "received message: Bye Workers", ret)
}

func TestWorkerExtensionNotStarted(t *testing.T) {
	externalWorkers, _ := WithExtensionWorkers("extensionWorkers", "testdata/message-worker.php", 1)

	require.NoError(t, Init())
	t.Cleanup(Shutdown)

	w := httptest.NewRecorder()
	err := externalWorkers.SendRequest(w, httptest.NewRequest("GET", "https://example.com/testdata/hello.php", nil))
	require.ErrorIs(t, err, ErrNotRunning)
	assert.Empty(t, w.Body.String())

	_, err = externalWorkers.SendMessage(t.Context(), "Hello Workers", nil)
	require.ErrorIs(t, err, ErrNotRunning)
}

// a reload that doesn't start the worker again, as Caddy's did from the second one on
func TestWorkerExtensionNotStartedAfterReload(t *testing.T) {
	externalWorkers, o := WithExtensionWorkers("extensionWorkers", "testdata/message-worker.php", 1)

	require.NoError(t, Init(o))
	Shutdown()
	require.NoError(t, Init())
	t.Cleanup(Shutdown)

	w := httptest.NewRecorder()
	err := externalWorkers.SendRequest(w, httptest.NewRequest("GET", "https://example.com/testdata/hello.php", nil))
	require.ErrorIs(t, err, ErrNotRunning)
	assert.Empty(t, w.Body.String())

	_, err = externalWorkers.SendMessage(t.Context(), "Hello Workers", nil)
	require.ErrorIs(t, err, ErrNotRunning)
}

func TestWorkerExtensionSendMessageDuringReload(t *testing.T) {
	externalWorkers, o := WithExtensionWorkers("extensionWorkers", "testdata/message-worker.php", 1)

	require.NoError(t, Init(o))
	t.Cleanup(Shutdown)

	ctx, cancel := context.WithCancel(t.Context())
	var wg sync.WaitGroup
	wg.Go(func() {
		for ctx.Err() == nil {
			msgCtx, msgCancel := context.WithTimeout(ctx, 10*time.Millisecond)
			_, _ = externalWorkers.SendMessage(msgCtx, "Hello Workers", nil)
			msgCancel()
		}
	})

	for range 10 {
		Shutdown()
		require.NoError(t, Init(o))
	}

	cancel()
	wg.Wait()
}

func TestWorkerExtensionRemovedWorkerIsNotScaled(t *testing.T) {
	externalWorkers, o := WithExtensionWorkers("extensionWorkers", "testdata/message-worker.php", 1)

	require.NoError(t, Init(o))
	removed := externalWorkers.(*extensionWorkers).internalWorker.Load()
	Shutdown()

	require.NoError(t, Init(WithNumThreads(1), WithMaxThreads(2)))
	t.Cleanup(Shutdown)

	// a call that passed the running check before the reload
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	fc := newContextFromMessage("Hello Workers", nil, ctx, removed)
	fc.startedAt = time.Now().Add(-time.Second)
	numThreads := removed.countThreads()

	require.ErrorIs(t, removed.handleRequest(fc), ErrNotRunning)
	assert.Nil(t, fc.handlerReturn)
	assert.Never(t, func() bool { return removed.countThreads() > numThreads }, 500*time.Millisecond, 10*time.Millisecond)
}

func TestWithWorkerNameUnknown(t *testing.T) {
	require.ErrorIs(t, WithWorkerName("unknown")(&frankenPHPContext{}), ErrUnknownWorker)
}
