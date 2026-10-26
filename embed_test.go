package frankenphp

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setEmbeddedApp(t *testing.T, files map[string]string) {
	t.Helper()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "public/", Typeflag: tar.TypeDir, Mode: 0o755}))
	for name, content := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(content))}))
		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())

	previous := embeddedApp
	embeddedApp = buf.Bytes()
	t.Cleanup(func() { embeddedApp = previous })
}

func TestPrepareDefaultEmbeddedAppPathExtractsAndReuses(t *testing.T) {
	setEmbeddedApp(t, map[string]string{"public/index.php": "<?php echo 'ok';"})
	path := filepath.Join(t.TempDir(), "app")

	require.NoError(t, prepareDefaultEmbeddedAppPath(path))
	assert.FileExists(t, filepath.Join(path, "public", "index.php"))

	require.NoError(t, prepareDefaultEmbeddedAppPath(path))
	require.NoError(t, untar(path))
	content, err := os.ReadFile(filepath.Join(path, "public", "index.php"))
	require.NoError(t, err)
	assert.Equal(t, "<?php echo 'ok';", string(content))
}

func TestPrepareDefaultEmbeddedAppPathConcurrently(t *testing.T) {
	setEmbeddedApp(t, map[string]string{"public/index.php": "<?php echo 'ok';"})
	parent := t.TempDir()
	path := filepath.Join(parent, "app")

	var wg sync.WaitGroup
	errs := make([]error, 16)
	for i := range errs {
		wg.Go(func() { errs[i] = prepareDefaultEmbeddedAppPath(path) })
	}
	wg.Wait()

	for _, err := range errs {
		assert.NoError(t, err)
	}
	content, err := os.ReadFile(filepath.Join(path, "public", "index.php"))
	require.NoError(t, err)
	assert.Equal(t, "<?php echo 'ok';", string(content))

	entries, err := os.ReadDir(parent)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "temporary extraction directories must be removed")
}

func TestPrepareDefaultEmbeddedAppPathRejectsSymlink(t *testing.T) {
	setEmbeddedApp(t, map[string]string{"public/index.php": "<?php echo 'ok';"})
	parent := t.TempDir()

	target := filepath.Join(parent, "target")
	require.NoError(t, os.Mkdir(target, 0o700))
	link := filepath.Join(parent, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("cannot create symlinks: %v", err)
	}

	assert.Error(t, prepareDefaultEmbeddedAppPath(link))
}
