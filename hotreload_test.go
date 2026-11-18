//go:build !nomercure && !nowatcher

package frankenphp_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dunglas/frankenphp"
	"github.com/dunglas/mercure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHotReloadOpcacheRevalidateFreq(t *testing.T) {
	for name, tc := range map[string]struct {
		hotReload bool
		phpIni    map[string]string
		expected  string
	}{
		"hot reload":                 {hotReload: true, expected: "0"},
		"hot reload with php_ini":    {hotReload: true, phpIni: map[string]string{"opcache.revalidate_freq": "5"}, expected: "5"},
		"without hot reload":         {expected: "2"},
		"without hot reload php_ini": {phpIni: map[string]string{"opcache.revalidate_freq": "5"}, expected: "5"},
	} {
		t.Run(name, func(t *testing.T) {
			opts := &testOptions{nbParallelRequests: 1, phpIni: tc.phpIni}
			if tc.hotReload {
				h, err := mercure.NewHub(t.Context(), mercure.WithTransport(mercure.NewLocalTransport(mercure.NewSubscriberList(0))))
				require.NoError(t, err)

				opts.initOpts = []frankenphp.Option{frankenphp.WithHotReload("hot-reload", h, []string{"./testdata/**/*.none"})}
			}

			runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, _ int) {
				body, _ := testGet("http://example.com/ini.php?key=opcache.revalidate_freq", handler, t)
				if strings.HasSuffix(body, ":") {
					t.Skip("OPcache is not loaded")
				}

				assert.Equal(t, "opcache.revalidate_freq:"+tc.expected, body)
			}, opts)
		})
	}
}
