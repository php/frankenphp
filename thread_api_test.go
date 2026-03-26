package frankenphp

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestThreadReturnsNilWhenNotRunning(t *testing.T) {
	// Before Init(), Thread() should return nil
	thread, ok := Thread(0)
	assert.Nil(t, thread)
	assert.False(t, ok)
}

func TestThreadReturnsValidThread(t *testing.T) {
	t.Cleanup(Shutdown)

	require.NoError(t, Init(
		WithNumThreads(2),
		WithMaxThreads(2),
	))

	// Thread 0 should exist after init
	thread, ok := Thread(0)
	assert.True(t, ok)
	assert.NotNil(t, thread)
}

func TestThreadReturnsNilForInvalidIndex(t *testing.T) {
	t.Cleanup(Shutdown)

	require.NoError(t, Init(
		WithNumThreads(2),
		WithMaxThreads(2),
	))

	// Index beyond thread count should return nil
	thread, ok := Thread(999)
	assert.Nil(t, thread)
	assert.False(t, ok)
}

func TestThreadExposesRequestDuringExecution(t *testing.T) {
	t.Cleanup(Shutdown)

	require.NoError(t, Init(
		WithNumThreads(2),
		WithMaxThreads(2),
	))

	handler := func(w http.ResponseWriter, r *http.Request) {
		req, err := NewRequestWithContext(r,
			WithRequestDocumentRoot(testDataPath, false),
		)
		assert.NoError(t, err)

		err = ServeHTTP(w, req)
		assert.NoError(t, err)
	}

	req := httptest.NewRequest("GET", "http://localhost/echo.php", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPHPThreadPin(t *testing.T) {
	t.Cleanup(Shutdown)

	require.NoError(t, Init(
		WithNumThreads(2),
		WithMaxThreads(2),
	))

	thread, ok := Thread(0)
	require.True(t, ok)

	// Pin should not panic
	data := "test data"
	assert.NotPanics(t, func() {
		thread.Pin(&data)
	})
}
