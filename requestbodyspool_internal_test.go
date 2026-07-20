package frankenphp

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSpoolContext(body []byte) *frankenPHPContext {
	fc := newFrankenPHPContext()
	fc.logger = slog.Default()
	fc.request = httptest.NewRequest("POST", "/", bytes.NewReader(body))

	return fc
}

// TestSpoolRequestBodyInMemory drains a small body into memory, leaving no temp
// file to clean up.
func TestSpoolRequestBodyInMemory(t *testing.T) {
	body := bytes.Repeat([]byte("a"), 1024)
	fc := newSpoolContext(body)

	fc.spoolRequestBody()

	assert.True(t, fc.bodySpooled)
	assert.Nil(t, fc.cleanupBody, "a small body stays in memory")
	assert.Equal(t, int64(len(body)), fc.request.ContentLength)

	got, err := io.ReadAll(fc.request.Body)
	require.NoError(t, err)
	assert.Equal(t, body, got)
}

// TestSpoolRequestBodyToFile spills a body larger than the memory threshold to a
// temp file, serves identical bytes, and removes the file on cleanup.
func TestSpoolRequestBodyToFile(t *testing.T) {
	body := bytes.Repeat([]byte("b"), bodySpoolMemoryThreshold+4096)
	fc := newSpoolContext(body)

	fc.spoolRequestBody()

	assert.True(t, fc.bodySpooled)
	require.NotNil(t, fc.cleanupBody, "a large body spills to a temp file")
	assert.Equal(t, int64(len(body)), fc.request.ContentLength)

	got, err := io.ReadAll(fc.request.Body)
	require.NoError(t, err)
	assert.Equal(t, body, got)

	fc.cleanupBody()
}

// TestSpoolRequestBodySkipsStreaming leaves a body of unknown length untouched so
// long-lived streaming uploads keep their live stream.
func TestSpoolRequestBodySkipsStreaming(t *testing.T) {
	fc := newSpoolContext([]byte("streamed"))
	fc.request.ContentLength = -1

	fc.spoolRequestBody()

	assert.False(t, fc.bodySpooled)
	got, err := io.ReadAll(fc.request.Body)
	require.NoError(t, err)
	assert.Equal(t, "streamed", string(got))
}

// TestSpoolRequestBodyIdempotent does not re-drain an already spooled body: a
// second call must not touch the already consumed stream.
func TestSpoolRequestBodyIdempotent(t *testing.T) {
	body := bytes.Repeat([]byte("c"), 512)
	fc := newSpoolContext(body)

	require.NoError(t, fc.spoolRequestBody())
	require.NoError(t, fc.spoolRequestBody())

	assert.True(t, fc.bodySpooled)
	got, err := io.ReadAll(fc.request.Body)
	require.NoError(t, err)
	assert.Equal(t, body, got)
}

// TestSpoolRequestBodyRejectsOversized rejects a body that overruns a
// request_body max_size limit (an http.MaxBytesReader) with 413 instead of
// feeding PHP a truncated request.
func TestSpoolRequestBodyRejectsOversized(t *testing.T) {
	fc := newSpoolContext(bytes.Repeat([]byte("d"), 4096))
	rec := httptest.NewRecorder()
	fc.responseWriter = rec
	fc.request.Body = http.MaxBytesReader(rec, fc.request.Body, 1024)

	err := fc.spoolRequestBody()

	require.ErrorIs(t, err, ErrRequestBodyTooLarge)
	assert.False(t, fc.bodySpooled)
	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
}
