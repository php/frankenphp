package frankenphp

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"
)

// bodySpoolMemoryThreshold is how much of a queued request body is kept in
// memory before spilling to a temp file.
const bodySpoolMemoryThreshold = 2 << 20 // 2 MiB

// spoolRequestBody drains the request body and replaces it with a buffered copy.
//
// A request stalled in the thread queue keeps its HTTP/2 stream flow-control
// window open while no one reads the body. Enough stalled uploads multiplexed on
// a single connection exhaust the connection-level window and deadlock every
// stream on that connection, including those a thread is already serving.
// Draining a queued body up front releases the window and breaks the deadlock.
// See https://github.com/php/frankenphp/issues/1074.
//
// Only bodies with a known, non-zero Content-Length are spooled. Streaming
// requests (chunked or unknown length, e.g. long-lived uploads) keep their live
// stream and their current behavior.
// It returns ErrRequestBodyTooLarge (and rejects the request with 413) when a
// request_body max_size limit wraps the body and the client exceeds it; callers
// must stop handling the request in that case.
func (fc *frankenPHPContext) spoolRequestBody() error {
	r := fc.request
	if fc.bodySpooled || r == nil || r.Body == nil || r.Body == http.NoBody || r.ContentLength <= 0 {
		return nil
	}

	src := io.Reader(r.Body)

	// keep bounding slow uploads while draining, mirroring go_read_post
	if fc.requestBodyTimeout > 0 && !fc.isDone && fc.responseWriter != nil {
		if fc.responseController == nil {
			fc.responseController = http.NewResponseController(fc.responseWriter)
		}
		src = &deadlineReader{r: r.Body, rc: fc.responseController, timeout: fc.requestBodyTimeout}
	}

	sw := &spoolWriter{threshold: bodySpoolMemoryThreshold}
	n, err := io.Copy(sw, src)

	if fc.requestBodyTimeout > 0 && fc.responseController != nil {
		_ = fc.responseController.SetReadDeadline(time.Time{})
	}
	_ = r.Body.Close()

	// A body larger than the configured max_size cannot be handled at all;
	// reject it up front instead of feeding PHP a truncated request.
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		sw.cleanup()
		fc.reject(ErrRequestBodyTooLarge)

		return ErrRequestBodyTooLarge
	}

	if err != nil {
		// The stream is already (partially) drained and cannot be replayed.
		// Hand PHP whatever was read; a short read surfaces as EOF, matching
		// what a live read would have produced after the same failure.
		if fc.logger.Enabled(fc.request.Context(), slog.LevelWarn) {
			fc.logger.LogAttrs(fc.request.Context(), slog.LevelWarn, "error while spooling request body", slog.Any("error", err))
		}
	}

	fc.bodySpooled = true
	r.ContentLength = n

	if sw.file == nil {
		r.Body = io.NopCloser(bytes.NewReader(sw.buf.Bytes()))

		return nil
	}

	if _, err := sw.file.Seek(0, io.SeekStart); err != nil {
		sw.cleanup()
		r.Body = io.NopCloser(bytes.NewReader(nil))
		r.ContentLength = 0

		return nil
	}

	r.Body = sw.file
	fc.cleanupBody = sw.cleanup

	return nil
}

// spoolWriter buffers in memory up to threshold, then spills the rest to a temp
// file so a large queued body never grows the heap unbounded.
type spoolWriter struct {
	buf       bytes.Buffer
	file      *os.File
	threshold int
}

func (s *spoolWriter) Write(p []byte) (int, error) {
	if s.file == nil {
		if s.buf.Len()+len(p) <= s.threshold {
			return s.buf.Write(p)
		}

		f, err := os.CreateTemp("", "frankenphp-upload-*")
		if err != nil {
			return 0, err
		}

		s.file = f
		if _, err := s.file.Write(s.buf.Bytes()); err != nil {
			return 0, err
		}
		s.buf.Reset()
	}

	return s.file.Write(p)
}

// cleanup releases the backing temp file, if any. Safe to call when nothing spilled.
func (s *spoolWriter) cleanup() {
	if s.file == nil {
		return
	}

	name := s.file.Name()
	_ = s.file.Close()
	_ = os.Remove(name)
}

// deadlineReader resets the read deadline before every read to bound a stall
// without capping a steady upload.
type deadlineReader struct {
	r       io.Reader
	rc      *http.ResponseController
	timeout time.Duration
}

func (d *deadlineReader) Read(p []byte) (int, error) {
	_ = d.rc.SetReadDeadline(time.Now().Add(d.timeout))

	return d.r.Read(p)
}
