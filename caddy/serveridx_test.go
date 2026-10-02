package caddy

import (
	"testing"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/stretchr/testify/require"
)

func TestAssignServerIndexIncrements(t *testing.T) {
	h := httpcaddyfile.Helper{State: map[string]any{}}
	m1 := &FrankenPHPModule{}
	m2 := &FrankenPHPModule{}

	m1.assignServerIndex(h)
	m2.assignServerIndex(h)

	require.Equal(t, 1, m1.ServerIndex)
	require.Equal(t, 2, m2.ServerIndex)
}

func TestRegisterModulesWithSameServerIndexShareOneServer(t *testing.T) {
	app := &FrankenPHPApp{}
	shared1 := &FrankenPHPModule{ServerIndex: 1, resolvedDocumentRoot: "../testdata"}
	shared2 := &FrankenPHPModule{ServerIndex: 1, resolvedDocumentRoot: "../testdata"}
	app.modules = []*FrankenPHPModule{shared1, shared2}

	require.NoError(t, app.registerModules(caddy.NewReplacer()))

	require.NotNil(t, shared1.server)
	require.Same(t, shared1.server, shared2.server, "modules with the same server_idx must share one server instance")
}

func TestRegisterModulesWithoutServerIndexGetOwnServers(t *testing.T) {
	app := &FrankenPHPApp{}
	auto1 := &FrankenPHPModule{resolvedDocumentRoot: "../testdata"}
	auto2 := &FrankenPHPModule{resolvedDocumentRoot: "../testdata"}
	indexed := &FrankenPHPModule{ServerIndex: 1, resolvedDocumentRoot: "../testdata"}
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
	first := &FrankenPHPModule{Name: "first", ServerIndex: 2, resolvedDocumentRoot: "../testdata"}
	second := &FrankenPHPModule{Name: "second", ServerIndex: 2, resolvedDocumentRoot: "../testdata/env"}
	app.modules = []*FrankenPHPModule{first, second}

	require.NoError(t, app.registerModules(caddy.NewReplacer()))

	require.Same(t, first.server, second.server)
	require.Equal(t, first.reloadName, second.reloadName)
	require.Same(t, first, app.reloadModule("first"))
	require.Nil(t, app.reloadModule("second"))
	require.Len(t, app.opts, 1, "only one server must be registered for a shared index")
}

func TestReloadModuleUsesRegisteredLabels(t *testing.T) {
	first := &FrankenPHPModule{Name: "site", ServerIndex: 1, resolvedDocumentRoot: "../testdata"}
	shared := &FrankenPHPModule{Name: "site", ServerIndex: 1, resolvedDocumentRoot: "../testdata"}
	other := &FrankenPHPModule{Name: "other", resolvedDocumentRoot: "../testdata"}
	app := &FrankenPHPApp{modules: []*FrankenPHPModule{first, shared, other}}
	require.NoError(t, app.registerModules(caddy.NewReplacer()))

	first.Name = "changed"
	shared.Name = "changed"
	require.Same(t, first, app.reloadModule("site"))
	require.Same(t, other, app.reloadModule("other"))

	other.reloadName = "site"
	require.Nil(t, app.reloadModule("site"))
}
