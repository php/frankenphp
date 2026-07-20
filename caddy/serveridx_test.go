package caddy

import (
	"testing"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/stretchr/testify/require"
)

func TestAssignServerIdxIncrements(t *testing.T) {
	h := httpcaddyfile.Helper{State: map[string]any{}}
	m1 := &FrankenPHPModule{}
	m2 := &FrankenPHPModule{}

	m1.assignServerIdx(h)
	m2.assignServerIdx(h)

	require.Equal(t, 1, m1.ServerIdx)
	require.Equal(t, 2, m2.ServerIdx)
}

func TestRegisterModulesWithSameServerIdxShareOneServer(t *testing.T) {
	app := &FrankenPHPApp{}
	shared1 := &FrankenPHPModule{ServerIdx: 1, resolvedDocumentRoot: "../testdata"}
	shared2 := &FrankenPHPModule{ServerIdx: 1, resolvedDocumentRoot: "../testdata"}
	app.modules = []*FrankenPHPModule{shared1, shared2}

	require.NoError(t, app.registerModules(caddy.NewReplacer()))

	require.NotNil(t, shared1.server)
	require.Same(t, shared1.server, shared2.server, "modules with the same server_idx must share one server instance")
}

func TestRegisterModulesWithoutServerIdxGetOwnServers(t *testing.T) {
	app := &FrankenPHPApp{}
	auto1 := &FrankenPHPModule{resolvedDocumentRoot: "../testdata"}
	auto2 := &FrankenPHPModule{resolvedDocumentRoot: "../testdata"}
	indexed := &FrankenPHPModule{ServerIdx: 1, resolvedDocumentRoot: "../testdata"}
	app.modules = []*FrankenPHPModule{auto1, indexed, auto2}

	require.NoError(t, app.registerModules(caddy.NewReplacer()))

	require.NotNil(t, auto1.server)
	require.NotNil(t, auto2.server)
	require.NotNil(t, indexed.server)
	require.NotSame(t, auto1.server, auto2.server, "modules without server_idx must each get their own server")
	require.NotSame(t, auto1.server, indexed.server)
	require.NotSame(t, auto2.server, indexed.server)
}

// regression test for the double-registration scenario: a module POSTed via
// the admin API with an explicit server_idx must not steal the server of a
// module that already registered under the same index; it joins it instead,
// and the first registered module defines the server configuration
func TestRegisterModulesFirstModuleWinsPerIdx(t *testing.T) {
	app := &FrankenPHPApp{}
	first := &FrankenPHPModule{ServerIdx: 2, resolvedDocumentRoot: "../testdata"}
	second := &FrankenPHPModule{ServerIdx: 2, resolvedDocumentRoot: "../testdata/env"}
	app.modules = []*FrankenPHPModule{first, second}

	require.NoError(t, app.registerModules(caddy.NewReplacer()))

	require.Same(t, first.server, second.server)
	require.Len(t, app.opts, 1, "only one server must be registered for a shared index")
}
