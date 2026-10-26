//go:build !windows

package frankenphp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareDefaultEmbeddedAppPathCreatesPrivateDirectory(t *testing.T) {
	setEmbeddedApp(t, map[string]string{"public/index.php": "<?php echo 'ok';"})
	path := filepath.Join(t.TempDir(), "app")

	require.NoError(t, prepareDefaultEmbeddedAppPath(path))
	info, err := os.Lstat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
}

func TestPrepareDefaultEmbeddedAppPathRestrictsOwnedDirectory(t *testing.T) {
	setEmbeddedApp(t, map[string]string{"public/index.php": "<?php echo 'ok';"})
	path := filepath.Join(t.TempDir(), "app")
	require.NoError(t, os.Mkdir(path, 0o700))
	require.NoError(t, os.Chmod(path, 0o777))

	require.NoError(t, prepareDefaultEmbeddedAppPath(path))
	info, err := os.Lstat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
}

// Reproduces GHSA-6q3j-62j9-f9hh: another user plants PHP code before the app is extracted.
func TestPrepareDefaultEmbeddedAppPathRejectsDirectoryOwnedByAnotherUser(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("changing the owner of a directory requires root")
	}

	setEmbeddedApp(t, map[string]string{"public/index.php": "<?php echo 'ok';"})
	path := filepath.Join(t.TempDir(), "app")
	require.NoError(t, os.MkdirAll(filepath.Join(path, "public"), 0o777))
	planted := filepath.Join(path, "public", "index.php")
	require.NoError(t, os.WriteFile(planted, []byte("<?php echo 'pwned';"), 0o666))
	for _, p := range []string{path, filepath.Join(path, "public"), planted} {
		require.NoError(t, os.Chown(p, 65534, 65534))
	}

	assert.ErrorContains(t, prepareDefaultEmbeddedAppPath(path), "must be owned by the current user")
}

func TestPrepareDefaultEmbeddedAppPathRejectsUnsafeParent(t *testing.T) {
	setEmbeddedApp(t, map[string]string{"public/index.php": "<?php echo 'ok';"})
	parent := t.TempDir()
	require.NoError(t, os.Chmod(parent, 0o777))

	assert.ErrorContains(t, prepareDefaultEmbeddedAppPath(filepath.Join(parent, "app")), "sticky bit")

	require.NoError(t, os.Chmod(parent, 0o777|os.ModeSticky))
	assert.NoError(t, prepareDefaultEmbeddedAppPath(filepath.Join(parent, "app")))
}
