package frankenphp_test

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// socketCounter accepts the sockets opened by testdata/persistent-socket.php
// and counts how many of them PHP closed again.
type socketCounter struct {
	listener net.Listener
	opened   atomic.Int32
	closed   atomic.Int32
}

func newSocketCounter(t *testing.T) *socketCounter {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	counter := &socketCounter{listener: listener}
	go func() {
		for {
			connection, err := listener.Accept()
			if err != nil {
				return
			}
			counter.opened.Add(1)

			go func() {
				defer func() { _ = connection.Close() }()
				// returns once PHP closes its end of the socket
				_, _ = io.Copy(io.Discard, connection)
				counter.closed.Add(1)
			}()
		}
	}()

	return counter
}

func (counter *socketCounter) url() string {
	return fmt.Sprintf("http://example.com/persistent-socket.php?port=%d", counter.listener.Addr().(*net.TCPAddr).Port)
}

func (counter *socketCounter) assertCounts(t *testing.T, opened int32, closed int32) {
	t.Helper()

	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, opened, counter.opened.Load(), "persistent sockets opened")
		assert.Equal(c, closed, counter.closed.Load(), "persistent sockets closed")
	}, 5*time.Second, 10*time.Millisecond)
}

// A PHP thread must destroy its persistent resources (pfsockopen(), persistent
// PDO connections...) before it exits: since PHP 8.4, ts_free_thread() no
// longer does it, and up to PHP 8.3 a reboot then aborts with a corrupted heap
// for persistent streams.
func TestPersistentResourcesAreDestroyedOnReboot(t *testing.T) {
	sockets := newSocketCounter(t)

	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, _ int) {
		body, _ := testGet(sockets.url(), handler, t)
		require.Equal(t, "ok", body)
		sockets.assertCounts(t, 1, 0)

		frankenphp.RestartWorkers()

		body, _ = testGet(sockets.url(), handler, t)
		require.Equal(t, "ok", body)
		sockets.assertCounts(t, 2, 1)
	}, &testOptions{
		nbParallelRequests: 1,
		initOpts:           []frankenphp.Option{frankenphp.WithNumThreads(1)},
	})
}

func TestPersistentResourcesAreDestroyedOnMaxRequests(t *testing.T) {
	const maxRequests = 2
	const totalRequests = 10
	sockets := newSocketCounter(t)

	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, _ int) {
		for range totalRequests {
			body, _ := testGet(sockets.url(), handler, t)
			require.Equal(t, "ok", body)
		}

		// each thread reuses its socket until it restarts after maxRequests
		sockets.assertCounts(t, totalRequests/maxRequests, totalRequests/maxRequests)
	}, &testOptions{
		nbParallelRequests: 1,
		initOpts: []frankenphp.Option{
			frankenphp.WithNumThreads(1),
			frankenphp.WithMaxRequests(maxRequests),
		},
	})
}
