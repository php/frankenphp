package frankenphp_test

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConcurrentUploadsHTTP2 reproduces php/frankenphp#1074: many request bodies
// multiplexed on a single HTTP/2 connection, with more streams than PHP threads.
// A queued stream that no thread reads keeps its flow-control window open; enough
// of them exhaust the connection window and deadlock every stream, including the
// ones a thread is serving. Draining queued bodies up front must let all of them
// complete.
func TestConcurrentUploadsHTTP2(t *testing.T) {
	require.NoError(t, frankenphp.Init(
		frankenphp.WithNumThreads(2),
		frankenphp.WithMaxThreads(2),
	))
	defer frankenphp.Shutdown()

	cwd, _ := os.Getwd()
	handler := func(w http.ResponseWriter, r *http.Request) {
		req, err := frankenphp.NewRequestWithContext(r,
			frankenphp.WithRequestDocumentRoot(cwd+"/testdata/", false),
		)
		require.NoError(t, err)
		require.NoError(t, frankenphp.ServeHTTP(w, req))
	}

	addr, client := newH2CServer(t, handler)

	const (
		concurrency = 30
		bodySize    = 512 << 10 // large enough to exhaust the connection window
	)
	body := bytes.Repeat([]byte("x"), bodySize)
	want := fmt.Sprintf("read=%d", bodySize)

	var wg sync.WaitGroup
	errs := make(chan error, concurrency)
	for i := range concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()

			req, err := http.NewRequest(http.MethodPost, "http://"+addr+"/read-input.php", bytes.NewReader(body))
			if err != nil {
				errs <- err
				return
			}
			req.ContentLength = bodySize
			req.Header.Set("Content-Type", "application/octet-stream")

			resp, err := client.Do(req)
			if err != nil {
				errs <- fmt.Errorf("request %d: %w", i, err)
				return
			}
			defer func() { _ = resp.Body.Close() }()

			got, err := io.ReadAll(resp.Body)
			if err != nil {
				errs <- fmt.Errorf("request %d: %w", i, err)
				return
			}
			if string(got) != want {
				errs <- fmt.Errorf("request %d: got %q, want %q", i, got, want)
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		close(errs)
		for err := range errs {
			assert.NoError(t, err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("concurrent uploads deadlocked: streams stalled waiting for a PHP thread")
	}
}
