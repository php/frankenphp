//go:build !nowatcher

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

func TestWorkerWithInactiveWatcher(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`

			frankenphp {
				worker {
					file ../testdata/worker-with-counter.php
					num 1
					watch ./**/*.php
				}
			}
		}

		localhost:`+testPort+` {
			root ../testdata
			rewrite worker-with-counter.php
			php
		}
		`, "caddyfile")

	tester.AssertGetResponse("http://localhost:"+testPort, http.StatusOK, "requests:1")
	tester.AssertGetResponse("http://localhost:"+testPort, http.StatusOK, "requests:2")
}

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
				name test
				root `+tmpDir+`
				hot_reload `+tmpDir+`/*.php
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
			_, err := resp.Body.Read(buf)
			require.NoError(t, err)

			receivedBody.Write(buf)

			if strings.Contains(receivedBody.String(), "index.php") {
				cancel()

				break
			}
		}

		require.NoError(t, resp.Body.Close())
	})

	connected.Wait()

	require.NoError(t, os.WriteFile(indexFile, []byte("<?=$_SERVER['FRANKENPHP_HOT_RELOAD'];"), 0644))

	received.Wait()

	tester.AssertGetResponse("http://localhost:"+testPort+"/index.php", http.StatusOK, u)
}
