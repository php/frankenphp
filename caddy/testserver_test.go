package caddy_test

import (
	"testing"

	"github.com/caddyserver/caddy/v2/caddytest"
	testserver "github.com/dunglas/frankenphp/caddy/internal/testserver"
)

func initTestServer(t *testing.T, tester *caddytest.Tester, config, format string) {
	testserver.InitTestServer(t, tester, config, format)
}
