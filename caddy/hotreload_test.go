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
	tester.InitServer(`
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
	defer resp.Body.Close()

	// Wait for the first bytes before changing the file, so the subscription
	// is ready. Keep reads on the test goroutine so a failed request or read
	// cannot leave a readiness WaitGroup blocked forever.
	buf := make([]byte, 1024)
	_, err = resp.Body.Read(buf)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(indexFile, []byte("<?=$_SERVER['FRANKENPHP_HOT_RELOAD'];"), 0644))

	var receivedBody strings.Builder
	for {
		n, err := resp.Body.Read(buf)
		receivedBody.Write(buf[:n])
		if strings.Contains(receivedBody.String(), "index.php") {
			break
		}
		// A read may return both the expected event and an error.
		require.NoError(t, err)
	}
	cancel()
	require.NoError(t, resp.Body.Close())

	tester.AssertGetResponse("http://localhost:"+testPort+"/index.php", http.StatusOK, u)
}
