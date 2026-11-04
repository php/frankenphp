package caddy

import (
	"regexp"
	"strconv"
	"testing"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/stretchr/testify/require"
)

func TestAssignServerIndexNumbersNamedRoutesApartFromSiteBlocks(t *testing.T) {
	adapter := caddyfile.Adapter{ServerType: httpcaddyfile.ServerType{}}
	cfg, _, err := adapter.Adapt([]byte(`
		&(a) {
			php_server
		}

		&(b) {
			php_server
		}

		http://a.localhost {
			php_server
		}

		http://b.localhost {
			php_server
		}

		http://c.localhost {
			invoke a
			invoke b
		}
	`), nil)
	require.NoError(t, err)

	var indexes []int
	for _, m := range regexp.MustCompile(`"server_index":(\d+)`).FindAllSubmatch(cfg, -1) {
		i, err := strconv.Atoi(string(m[1]))
		require.NoError(t, err)
		indexes = append(indexes, i)
	}

	require.ElementsMatch(t, []int{1, 2, namedRouteServerIndexBase + 1, namedRouteServerIndexBase + 2}, indexes)
}

func TestRegisterModulesWithSameServerIndexShareOneServer(t *testing.T) {
	app := &FrankenPHPApp{}
	shared1 := &FrankenPHPModule{ServerIndex: 1, resolvedDocumentRoot: "../testdata"}
	shared2 := &FrankenPHPModule{ServerIndex: 1, resolvedDocumentRoot: "../testdata"}
	app.modules = []*FrankenPHPModule{shared1, shared2}

	_, err := app.collectModuleOptions(caddy.NewReplacer(), map[string]bool{}, true)
	require.NoError(t, err)

	require.NotNil(t, shared1.server)
	require.Same(t, shared1.server, shared2.server, "modules with the same server_idx must share one server instance")
}

func TestRegisterModulesWithoutServerIndexGetOwnServers(t *testing.T) {
	app := &FrankenPHPApp{}
	auto1 := &FrankenPHPModule{resolvedDocumentRoot: "../testdata"}
	auto2 := &FrankenPHPModule{resolvedDocumentRoot: "../testdata"}
	indexed := &FrankenPHPModule{ServerIndex: 1, resolvedDocumentRoot: "../testdata"}
	app.modules = []*FrankenPHPModule{auto1, indexed, auto2}

	_, err := app.collectModuleOptions(caddy.NewReplacer(), map[string]bool{}, true)
	require.NoError(t, err)

	require.NotNil(t, auto1.server)
	require.NotNil(t, auto2.server)
	require.NotNil(t, indexed.server)
	require.NotSame(t, auto1.server, auto2.server, "modules without server_idx must each get their own server")
	require.NotSame(t, auto1.server, indexed.server)
	require.NotSame(t, auto2.server, indexed.server)
}
