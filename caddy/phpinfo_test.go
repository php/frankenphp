package caddy_test

import (
	"html"
	"io"
	"net/http"
	"regexp"
	"testing"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddytest"
	"github.com/stretchr/testify/require"
)

func TestPHPInfoCaddyVersion(t *testing.T) {
	tester := caddytest.NewTester(t)
	initServer(t, tester, `
		{
			skip_install_trust
			admin localhost:2999
		}

		http://localhost:`+testPort+` {
			php_server {
				root ../testdata
			}
		}
		`, "caddyfile")

	resp, err := tester.Client.Get("http://localhost:" + testPort + "/phpinfo.php")
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	row := regexp.MustCompile(`<tr><td class="e">caddy </td><td class="v">(.*?) </td></tr>`).FindSubmatch(body)
	simpleVersion, _ := caddy.Version()
	require.Len(t, row, 2, "phpinfo must include the Caddy version row")
	require.Equal(t, html.EscapeString(simpleVersion), string(row[1]))
}
