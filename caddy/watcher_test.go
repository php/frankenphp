//go:build !nowatcher

package caddy_test

import (
	"net/http"
	"testing"

	"github.com/caddyserver/caddy/v2/caddytest"
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
