package caddy_test

import (
	"encoding/json"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/caddytest"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/require"
)

// initTestServer starts each test with its own listeners. Reloading the previous
// test's configuration can leave Caddy's shared Windows admin listener with an
// expired accept deadline, preventing all subsequent admin requests.
func initTestServer(t *testing.T, tester *caddytest.Tester, config, format string) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	configJSON := []byte(config)
	if format != "json" {
		adapter := caddyconfig.GetAdapter(format)
		require.NotNil(t, adapter, "unknown config adapter %q", format)
		var err error
		configJSON, _, err = adapter.Adapt(configJSON, nil)
		require.NoError(t, err)
	}

	var cfg caddy.Config
	require.NoError(t, json.Unmarshal(configJSON, &cfg))
	var httpApp caddyhttp.App
	require.NoError(t, json.Unmarshal(cfg.AppsRaw["http"], &httpApp))

	var listeners []caddy.NetworkAddress
	addListener := func(address string) {
		t.Helper()
		addr, err := caddy.ParseNetworkAddress(address)
		require.NoError(t, err)
		listeners = append(listeners, addr.Expand()...)
	}
	if cfg.Admin == nil || !cfg.Admin.Disabled {
		address := caddy.DefaultAdminListen
		if cfg.Admin != nil && cfg.Admin.Listen != "" {
			address = cfg.Admin.Listen
		}
		addListener(address)
	}
	for _, server := range httpApp.Servers {
		for _, address := range server.Listen {
			addListener(address)
		}
	}

	t.Cleanup(func() {
		tester.Client.CloseIdleConnections()
		http.DefaultClient.CloseIdleConnections()
		if t.Failed() {
			t.Logf("Caddy configuration:\n%s", configJSON)
		}
		// Disable the admin endpoint through the Go API: caddy.Stop leaves it
		// running, and fetching diagnostics over HTTP can hang if it is broken.
		require.NoError(t, caddy.Load([]byte(`{"admin":{"disabled":true}}`), true))
		frankenphp.Shutdown()
		// Caddy closes listeners asynchronously. Wait until the old sockets
		// are released before the next test binds the same addresses.
		require.Eventually(t, func() bool {
			for _, addr := range listeners {
				if caddy.ListenerUsage(addr.Network, addr.JoinHostPort(0)) != 0 {
					return false
				}
				// The pool entry is removed before the socket is closed.
				listener, err := net.Listen(addr.Network, addr.JoinHostPort(0))
				if err != nil {
					return false
				}
				if err := listener.Close(); err != nil {
					return false
				}
			}
			return true
		}, 5*time.Second, 10*time.Millisecond, "Caddy test listeners were not released")
	})

	// Loading directly avoids caddytest's bootstrap admin listener and the
	// immediate reload of that listener when the real configuration is sent.
	require.NoError(t, caddy.Load(configJSON, true))
}
