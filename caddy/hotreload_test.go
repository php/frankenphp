//go:build !nowatcher && !nomercure

package caddy_test

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

	var connected, received sync.WaitGroup

	connected.Add(1)
	received.Go(func() {
		cx, cancel := context.WithCancel(t.Context())
		req, _ := http.NewRequest(http.MethodGet, "http://localhost:"+testPort+u, nil)
		req = req.WithContext(cx)
		resp := tester.AssertResponseCode(req, http.StatusOK)

		var receivedBody strings.Builder

		buf := make([]byte, 1024)
		isConnected := false
		for {
			n, err := resp.Body.Read(buf)
			if n > 0 {
				receivedBody.Write(buf[:n])
			}
			if !isConnected {
				// wait for the first bytes before marking the client as connected
				isConnected = true
				connected.Done()
			}
			if strings.Contains(receivedBody.String(), "index.php") {
				cancel()

				break
			}
			// Surface the read error only after checking the buffer: on
			// Windows the SSE server sometimes flushes the event and closes
			// the connection in the same syscall, so Read returns (n>0, EOF)
			// and we'd otherwise fail despite having the data we wanted.
			require.NoError(t, err)
		}

		require.NoError(t, resp.Body.Close())
	})

	connected.Wait()

	require.NoError(t, os.WriteFile(indexFile, []byte("<?=$_SERVER['FRANKENPHP_HOT_RELOAD'];"), 0644))

	received.Wait()

	tester.AssertGetResponse("http://localhost:"+testPort+"/index.php", http.StatusOK, u)
}
