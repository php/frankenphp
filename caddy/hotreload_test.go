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

	"github.com/caddyserver/caddy/v2/caddytest"
	"github.com/stretchr/testify/require"
)

func TestHotReload(t *testing.T) {
	const topic = "https://frankenphp.dev/hot-reload/test"

	u := "/.well-known/mercure?topic=" + url.QueryEscape(topic)

	tmpDir := t.TempDir()
	indexFile := filepath.Join(tmpDir, "index.php")

	tester := caddytest.NewTester(t)
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

		connected.Done()

		var receivedBody strings.Builder

		buf := make([]byte, 1024)
		for {
			n, err := resp.Body.Read(buf)
			if n > 0 {
				receivedBody.Write(buf[:n])
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
