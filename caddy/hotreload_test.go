//go:build !nowatcher && !nomercure

package caddy_test

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2/caddytest"
	"github.com/stretchr/testify/require"
)

func TestHotReload(t *testing.T) {
	const topic = "https://frankenphp.dev/hot-reload/test"

	u := "/.well-known/mercure?topic=" + url.QueryEscape(topic)

	tmpDir := t.TempDir()
	indexFile := filepath.Join(tmpDir, "index.php")

	tester := caddytest.NewTester(t)
	// caddytest's default 5s http.Client.Timeout is too tight for the
	// SSE roundtrip below on slow CI runners (notably emulated armv7).
	// 30s keeps the test bounded so a real regression fails fast.
	tester.Client.Timeout = 30 * time.Second
	initTestServer(t, tester, `
		{
			debug
			skip_install_trust
			admin localhost:2999
		}

		http://localhost:`+testPort+` {
			mercure {
				transport local
				subscriber_jwt TestKey 
				anonymous
			}

			php_server {
				root `+tmpDir+`
				hot_reload {
					topic `+topic+`
					watch `+tmpDir+`/*.php
				}
			}
		`, "caddyfile")

	cx, cancel := context.WithCancel(t.Context())
	defer cancel()
	req, err := http.NewRequestWithContext(cx, http.MethodGet, "http://localhost:"+testPort+u, nil)
	require.NoError(t, err)
	resp := tester.AssertResponseCode(req, http.StatusOK)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	// Wait for the first bytes before changing the file, so the subscription
	// is ready. A failed request or read must not leave a readiness wait blocked.
	buf := make([]byte, 1024)
	_, err = resp.Body.Read(buf)
	require.NoError(t, err)

	// Return read failures to the test goroutine. Buffer the result so this
	// goroutine can exit even if a failed file write ends the test first.
	received := make(chan error, 1)
	go func() {
		var receivedBody strings.Builder
		for {
			n, err := resp.Body.Read(buf)
			receivedBody.Write(buf[:n])
			if strings.Contains(receivedBody.String(), "index.php") {
				received <- nil
				return
			}
			// A read may return both the expected event and an error.
			if err != nil {
				received <- err
				return
			}
		}
	}()

	// The native watcher starts asynchronously, independently of the SSE
	// subscription. Retry changes until one is observed, leaving more time
	// between writes than the watcher's 150 ms debounce interval.
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
waitForReload:
	for {
		require.NoError(t, os.WriteFile(indexFile, []byte("<?=$_SERVER['FRANKENPHP_HOT_RELOAD'];"), 0644))
		select {
		case err := <-received:
			require.NoError(t, err)
			break waitForReload
		case <-ticker.C:
		}
	}
	cancel()
	require.NoError(t, resp.Body.Close())

	tester.AssertGetResponse("http://localhost:"+testPort+"/index.php", http.StatusOK, u)
}
