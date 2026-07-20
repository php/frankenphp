//go:build !nowatcher

package caddy_test

import (
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2/caddytest"
	"github.com/stretchr/testify/require"
)

// waitForListener polls a raw TCP connection instead of an HTTP request so it
// doesn't consume a request from the worker's counter (unlike the initServer
// helper), while still covering the same listener hot-swap race it guards against.
func waitForListener(t *testing.T, addr string) {
	t.Helper()

	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err != nil {
			return false
		}
		_ = conn.Close()

		return true
	}, 5*time.Second, 100*time.Millisecond, "server failed to become ready")
}

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

	waitForListener(t, "localhost:"+testPort)

	tester.AssertGetResponse("http://localhost:"+testPort, http.StatusOK, "requests:1")
	tester.AssertGetResponse("http://localhost:"+testPort, http.StatusOK, "requests:2")
}
