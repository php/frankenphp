package caddy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveEmbeddedScript(t *testing.T) {
	app := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(app, "bin"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(app, "bin", "console"), []byte("#!/usr/bin/env php\n<?php\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(app, "index.php"), []byte("<?php\n"), 0o644))

	cwd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(cwd, "local.php"), []byte("<?php\n"), 0o644))
	t.Chdir(cwd)

	orig := frankenphp.EmbeddedAppPath
	t.Cleanup(func() { frankenphp.EmbeddedAppPath = orig })

	frankenphp.EmbeddedAppPath = ""
	assert.Equal(t, "bin/console", resolveEmbeddedScript("bin/console"))

	frankenphp.EmbeddedAppPath = app
	assert.Equal(t, filepath.Join(app, "bin", "console"), resolveEmbeddedScript("bin/console"))
	assert.Equal(t, filepath.Join(app, "index.php"), resolveEmbeddedScript("index.php"))
	assert.Equal(t, "local.php", resolveEmbeddedScript("local.php"), "files of the working directory take precedence")
	assert.Equal(t, "missing.php", resolveEmbeddedScript("missing.php"))
	assert.Equal(t, "-r", resolveEmbeddedScript("-r"))
	assert.Equal(t, "/bin/console", resolveEmbeddedScript("/bin/console"))
}
