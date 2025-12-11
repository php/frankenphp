//go:build !nowatcher

package watcher

import (
	"path/filepath"
	"testing"

	"github.com/e-dant/watcher/watcher-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDisallowOnEventTypeBiggerThan3(t *testing.T) {
	w := pattern{value: "/some/path"}
	require.NoError(t, w.parse())

	assert.False(t, w.allowReload(&watcher.Event{PathName: "/some/path/watch-me.php", EffectType: watcher.EffectTypeOwner}))
}

func TestDisallowOnPathTypeBiggerThan2(t *testing.T) {
	w := pattern{value: "/some/path"}
	require.NoError(t, w.parse())

	assert.False(t, w.allowReload(&watcher.Event{PathName: "/some/path/watch-me.php", PathType: watcher.PathTypeSymLink}))
}

func TestWatchesCorrectDir(t *testing.T) {
	t.Parallel()

	data := []struct {
		pattern string
		dir     string
	}{
		{"/path", "/path"},
		{"/path/", "/path"},
		{"/path/**/*.php", "/path"},
		{"/path/*.php", "/path"},
		{"/path/*/*.php", "/path"},
		{"/path/?path/*.php", "/path"},
		{"/path/{dir1,dir2}/**/*.php", "/path"},
		{".", relativeDir(t, "")},
		{"./", relativeDir(t, "")},
		{"./**", relativeDir(t, "")},
		{"..", relativeDir(t, "/..")},
	}

	for _, d := range data {
		t.Run(d.pattern, func(t *testing.T) {
			t.Parallel()

			hasDir(t, d.pattern, d.dir)
		})
	}
}

func TestValidRecursiveDirectories(t *testing.T) {
	t.Parallel()

	data := []struct {
		pattern string
		dir     string
	}{
		{"/path", "/path/file.php"},
		{"/path", "/path/subpath/file.php"},
		{"/path/", "/path/subpath/file.php"},
		{"/path**", "/path/subpath/file.php"},
		{"/path/**", "/path/subpath/file.php"},
		{"/path/**/", "/path/subpath/file.php"},
		{".", relativeDir(t, "file.php")},
		{".", relativeDir(t, "subpath/file.php")},
		{"./**", relativeDir(t, "subpath/file.php")},
		{"..", relativeDir(t, "subpath/file.php")},
	}

	for _, d := range data {
		t.Run(d.pattern, func(t *testing.T) {
			t.Parallel()

			shouldMatch(t, d.pattern, d.dir)
		})
	}
}

func TestInvalidRecursiveDirectories(t *testing.T) {
	t.Parallel()

	data := []struct {
		pattern string
		dir     string
	}{
		{"/path", "/other/file.php"},
		{"/path/**", "/other/file.php"},
		{".", "/other/file.php"},
	}

	for _, d := range data {
		t.Run(d.pattern, func(t *testing.T) {
			t.Parallel()

			shouldNotMatch(t, d.pattern, d.dir)
		})
	}
}

func TestValidNonRecursiveFilePatterns(t *testing.T) {
	t.Parallel()

	data := []struct {
		pattern string
		dir     string
	}{
		{"/*.php", "/file.php"},
		{"/path/*.php", "/path/file.php"},
		{"/path/?ile.php", "/path/file.php"},
		{"/path/file.php", "/path/file.php"},
		{"*.php", relativeDir(t, "file.php")},
		{"./*.php", relativeDir(t, "file.php")},
	}

	for _, d := range data {
		t.Run(d.pattern, func(t *testing.T) {
			t.Parallel()

			shouldMatch(t, d.pattern, d.dir)
		})
	}
}

func TestInValidNonRecursiveFilePatterns(t *testing.T) {
	t.Parallel()

	data := []struct {
		pattern string
		dir     string
	}{
		{"/path/*.txt", "/path/file.php"},
		{"/path/*.php", "/path/subpath/file.php"},
		{"/*.php", "/path/file.php"},
		{"*.txt", relativeDir(t, "file.php")},
		{"*.php", relativeDir(t, "subpath/file.php")},
	}

	for _, d := range data {
		t.Run(d.pattern, func(t *testing.T) {
			t.Parallel()

			shouldNotMatch(t, d.pattern, d.dir)
		})
	}
}

func TestValidRecursiveFilePatterns(t *testing.T) {
	t.Parallel()

	data := []struct {
		pattern string
		dir     string
	}{
		{"/path/**/*.php", "/path/file.php"},
		{"/path/**/*.php", "/path/subpath/file.php"},
		{"/path/**/?ile.php", "/path/subpath/file.php"},
		{"/path/**/file.php", "/path/subpath/file.php"},
		{"**/*.php", relativeDir(t, "file.php")},
		{"**/*.php", relativeDir(t, "subpath/file.php")},
		{"./**/*.php", relativeDir(t, "subpath/file.php")},
	}

	for _, d := range data {
		t.Run(d.pattern, func(t *testing.T) {
			t.Parallel()

			shouldMatch(t, d.pattern, d.dir)
		})
	}
}

func TestInvalidRecursiveFilePatterns(t *testing.T) {
	t.Parallel()

	data := []struct {
		pattern string
		dir     string
	}{
		{"/path/**/*.txt", "/path/file.php"},
		{"/path/**/*.txt", "/other/file.php"},
		{"/path/**/*.txt", "/path/subpath/file.php"},
		{"/path/**/?ilm.php", "/path/subpath/file.php"},
		{"**/*.php", "/other/file.php"},
		{".**/*.php", "/other/file.php"},
		{"./**/*.php", "/other/file.php"},
		{"/a/**/very/long/path.php", "/a/short.php"},
		{"", ""},
		{"/a/**/b/c/d/**/e.php", "/a/x/e.php"},
	}

	for _, d := range data {
		t.Run(d.pattern, func(t *testing.T) {
			t.Parallel()

			shouldNotMatch(t, d.pattern, d.dir)
		})
	}
}

func TestValidDirectoryPatterns(t *testing.T) {
	t.Parallel()

	data := []struct {
		pattern string
		dir     string
	}{
		{"/path/*/*.php", "/path/subpath/file.php"},
		{"/path/*/*/*.php", "/path/subpath/subpath/file.php"},
		{"/path/?/*.php", "/path/1/file.php"},
		{"/path/**/vendor/*.php", "/path/vendor/file.php"},
		{"/path/**/vendor/*.php", "/path/subpath/vendor/file.php"},
		{"/path/**/vendor/**/*.php", "/path/vendor/file.php"},
		{"/path/**/vendor/**/*.php", "/path/subpath/subpath/vendor/subpath/subpath/file.php"},
		{"/path/**/vendor/*/*.php", "/path/subpath/subpath/vendor/subpath/file.php"},
		{"/path*/path*/*", "/path1/path2/file.php"},
	}

	for _, d := range data {
		t.Run(d.pattern, func(t *testing.T) {
			t.Parallel()

			shouldMatch(t, d.pattern, d.dir)
		})
	}
}

func TestInvalidDirectoryPatterns(t *testing.T) {
	t.Parallel()
	data := []struct {
		pattern string
		dir     string
	}{
		{"/path/subpath/*.php", "/path/other/file.php"},
		{"/path/*/*.php", "/path/subpath/subpath/file.php"},
		{"/path/?/*.php", "/path/subpath/file.php"},
		{"/path/*/*/*.php", "/path/subpath/file.php"},
		{"/path/*/*/*.php", "/path/subpath/subpath/subpath/file.php"},
		{"/path/**/vendor/*.php", "/path/subpath/vendor/subpath/file.php"},
		{"/path/**/vendor/*.php", "/path/subpath/file.php"},
		{"/path/**/vendor/**/*.php", "/path/subpath/file.php"},
		{"/path/**/vendor/**/*.txt", "/path/subpath/vendor/subpath/file.php"},
		{"/path/**/vendor/**/*.php", "/path/subpath/subpath/subpath/file.php"},
		{"/path/**/vendor/*/*.php", "/path/subpath/vendor/subpath/subpath/file.php"},
		{"/path*/path*", "/path1/path1/file.php"},
	}

	for _, d := range data {
		t.Run(d.pattern, func(t *testing.T) {
			t.Parallel()

			shouldNotMatch(t, d.pattern, d.dir)
		})
	}
}

func TestValidExtendedPatterns(t *testing.T) {
	data := []struct {
		pattern string
		dir     string
	}{
		{"/path/*.{php}", "/path/file.php"},
		{"/path/*.{php,twig}", "/path/file.php"},
		{"/path/*.{php,twig}", "/path/file.twig"},
		{"/path/**/{file.php,file.twig}", "/path/subpath/file.twig"},
		{"/path/{dir1,dir2}/file.php", "/path/dir1/file.php"},
		{"/path/{dir1,dir2}/file.php", "/path/dir2/file.php"},
		{"/app/{app,config,resources}/**/*.php", "/app/app/subpath/file.php"},
		{"/app/{app,config,resources}/**/*.php", "/app/config/subpath/file.php"},
	}

	for _, d := range data {
		t.Run(d.pattern, func(t *testing.T) {
			t.Parallel()

			shouldMatch(t, d.pattern, d.dir)
		})
	}
}

func TestInvalidExtendedPatterns(t *testing.T) {
	data := []struct {
		pattern string
		dir     string
	}{
		{"/path/*.{php}", "/path/file.txt"},
		{"/path/*.{php,twig}", "/path/file.txt"},
		{"/path/{file.php,file.twig}", "/path/file.txt"},
		{"/path/{dir1,dir2}/file.php", "/path/dir3/file.php"},
		{"/path/{dir1,dir2}/**/*.php", "/path/dir1/subpath/file.txt"},
	}

	for _, d := range data {
		t.Run(d.pattern, func(t *testing.T) {
			t.Parallel()

			shouldNotMatch(t, d.pattern, d.dir)
		})
	}

}

func TestAnAssociatedEventTriggersTheWatcher(t *testing.T) {
	w := pattern{value: "/**/*.php"}
	require.NoError(t, w.parse())
	w.events = make(chan eventHolder)

	e := &watcher.Event{PathName: "/path/temporary_file", AssociatedPathName: "/path/file.php"}
	go w.handle(e)

	assert.Equal(t, e, (<-w.events).event)
}

func relativeDir(t *testing.T, relativePath string) string {
	dir, err := filepath.Abs("./" + relativePath)
	assert.NoError(t, err)
	return dir
}

func hasDir(t *testing.T, p string, dir string) {
	t.Helper()

	w := pattern{value: p}
	require.NoError(t, w.parse())

	assert.Equal(t, dir, w.value)
}

func shouldMatch(t *testing.T, p string, fileName string) {
	t.Helper()

	w := pattern{value: p}
	require.NoError(t, w.parse())

	assert.True(t, w.allowReload(&watcher.Event{PathName: fileName}))
}

func shouldNotMatch(t *testing.T, p string, fileName string) {
	t.Helper()

	w := pattern{value: p}
	require.NoError(t, w.parse())

	assert.False(t, w.allowReload(&watcher.Event{PathName: fileName}))
}
