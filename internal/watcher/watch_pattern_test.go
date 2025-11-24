//go:build !nowatcher

package watcher

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDisallowOnEventTypeBiggerThan3(t *testing.T) {
	w := watcher{name: "/some/path"}
	require.NoError(t, w.parse())

	assert.False(t, w.allowReload(&Event{PathName: "/some/path/watch-me.php", EffectType: EffectTypeOwner}))
}

func TestDisallowOnPathTypeBiggerThan2(t *testing.T) {
	w := watcher{name: "/some/path"}
	require.NoError(t, w.parse())

	assert.False(t, w.allowReload(&Event{PathName: "/some/path/watch-me.php", PathType: PathTypeSymLink}))
}

func TestWatchesCorrectDir(t *testing.T) {
	hasDir(t, "/path", "/path")
	hasDir(t, "/path/", "/path")
	hasDir(t, "/path/**/*.php", "/path")
	hasDir(t, "/path/*.php", "/path")
	hasDir(t, "/path/*/*.php", "/path")
	hasDir(t, "/path/?path/*.php", "/path")
	hasDir(t, "/path/{dir1,dir2}/**/*.php", "/path")
	hasDir(t, ".", relativeDir(t, ""))
	hasDir(t, "./", relativeDir(t, ""))
	hasDir(t, "./**", relativeDir(t, ""))
	hasDir(t, "..", relativeDir(t, "/.."))
}

func TestValidRecursiveDirectories(t *testing.T) {
	shouldMatch(t, "/path", "/path/file.php")
	shouldMatch(t, "/path", "/path/subpath/file.php")
	shouldMatch(t, "/path/", "/path/subpath/file.php")
	shouldMatch(t, "/path**", "/path/subpath/file.php")
	shouldMatch(t, "/path/**", "/path/subpath/file.php")
	shouldMatch(t, "/path/**/", "/path/subpath/file.php")
	shouldMatch(t, ".", relativeDir(t, "file.php"))
	shouldMatch(t, ".", relativeDir(t, "subpath/file.php"))
	shouldMatch(t, "./**", relativeDir(t, "subpath/file.php"))
	shouldMatch(t, "..", relativeDir(t, "subpath/file.php"))
}

func TestInvalidRecursiveDirectories(t *testing.T) {
	shouldNotMatch(t, "/path", "/other/file.php")
	shouldNotMatch(t, "/path/**", "/other/file.php")
	shouldNotMatch(t, ".", "/other/file.php")
}

func TestValidNonRecursiveFilePatterns(t *testing.T) {
	shouldMatch(t, "/*.php", "/file.php")
	shouldMatch(t, "/path/*.php", "/path/file.php")
	shouldMatch(t, "/path/?ile.php", "/path/file.php")
	shouldMatch(t, "/path/file.php", "/path/file.php")
	shouldMatch(t, "*.php", relativeDir(t, "file.php"))
	shouldMatch(t, "./*.php", relativeDir(t, "file.php"))
}

func TestInValidNonRecursiveFilePatterns(t *testing.T) {
	shouldNotMatch(t, "/path/*.txt", "/path/file.php")
	shouldNotMatch(t, "/path/*.php", "/path/subpath/file.php")
	shouldNotMatch(t, "/*.php", "/path/file.php")
	shouldNotMatch(t, "*.txt", relativeDir(t, "file.php"))
	shouldNotMatch(t, "*.php", relativeDir(t, "subpath/file.php"))
}

func TestValidRecursiveFilePatterns(t *testing.T) {
	shouldMatch(t, "/path/**/*.php", "/path/file.php")
	shouldMatch(t, "/path/**/*.php", "/path/subpath/file.php")
	shouldMatch(t, "/path/**/?ile.php", "/path/subpath/file.php")
	shouldMatch(t, "/path/**/file.php", "/path/subpath/file.php")
	shouldMatch(t, "**/*.php", relativeDir(t, "file.php"))
	shouldMatch(t, "**/*.php", relativeDir(t, "subpath/file.php"))
	shouldMatch(t, "./**/*.php", relativeDir(t, "subpath/file.php"))
}

func TestInvalidRecursiveFilePatterns(t *testing.T) {
	shouldNotMatch(t, "/path/**/*.txt", "/path/file.php")
	shouldNotMatch(t, "/path/**/*.txt", "/other/file.php")
	shouldNotMatch(t, "/path/**/*.txt", "/path/subpath/file.php")
	shouldNotMatch(t, "/path/**/?ilm.php", "/path/subpath/file.php")
	shouldNotMatch(t, "**/*.php", "/other/file.php")
	shouldNotMatch(t, ".**/*.php", "/other/file.php")
	shouldNotMatch(t, "./**/*.php", "/other/file.php")
}

func TestValidDirectoryPatterns(t *testing.T) {
	shouldMatch(t, "/path/*/*.php", "/path/subpath/file.php")
	shouldMatch(t, "/path/*/*/*.php", "/path/subpath/subpath/file.php")
	shouldMatch(t, "/path/?/*.php", "/path/1/file.php")
	shouldMatch(t, "/path/**/vendor/*.php", "/path/vendor/file.php")
	shouldMatch(t, "/path/**/vendor/*.php", "/path/subpath/vendor/file.php")
	shouldMatch(t, "/path/**/vendor/**/*.php", "/path/vendor/file.php")
	shouldMatch(t, "/path/**/vendor/**/*.php", "/path/subpath/subpath/vendor/subpath/subpath/file.php")
	shouldMatch(t, "/path/**/vendor/*/*.php", "/path/subpath/subpath/vendor/subpath/file.php")
	shouldMatch(t, "/path*/path*/*", "/path1/path2/file.php")
}

func TestInvalidDirectoryPatterns(t *testing.T) {
	shouldNotMatch(t, "/path/subpath/*.php", "/path/other/file.php")
	shouldNotMatch(t, "/path/*/*.php", "/path/subpath/subpath/file.php")
	shouldNotMatch(t, "/path/?/*.php", "/path/subpath/file.php")
	shouldNotMatch(t, "/path/*/*/*.php", "/path/subpath/file.php")
	shouldNotMatch(t, "/path/*/*/*.php", "/path/subpath/subpath/subpath/file.php")
	shouldNotMatch(t, "/path/**/vendor/*.php", "/path/subpath/vendor/subpath/file.php")
	shouldNotMatch(t, "/path/**/vendor/*.php", "/path/subpath/file.php")
	shouldNotMatch(t, "/path/**/vendor/**/*.php", "/path/subpath/file.php")
	shouldNotMatch(t, "/path/**/vendor/**/*.txt", "/path/subpath/vendor/subpath/file.php")
	shouldNotMatch(t, "/path/**/vendor/**/*.php", "/path/subpath/subpath/subpath/file.php")
	shouldNotMatch(t, "/path/**/vendor/*/*.php", "/path/subpath/vendor/subpath/subpath/file.php")
	shouldNotMatch(t, "/path*/path*", "/path1/path1/file.php")
}

func TestValidExtendedPatterns(t *testing.T) {
	shouldMatch(t, "/path/*.{php}", "/path/file.php")
	shouldMatch(t, "/path/*.{php,twig}", "/path/file.php")
	shouldMatch(t, "/path/*.{php,twig}", "/path/file.twig")
	shouldMatch(t, "/path/**/{file.php,file.twig}", "/path/subpath/file.twig")
	shouldMatch(t, "/path/{dir1,dir2}/file.php", "/path/dir1/file.php")
	shouldMatch(t, "/path/{dir1,dir2}/file.php", "/path/dir2/file.php")
	shouldMatch(t, "/app/{app,config,resources}/**/*.php", "/app/app/subpath/file.php")
	shouldMatch(t, "/app/{app,config,resources}/**/*.php", "/app/config/subpath/file.php")
}

func TestInValidExtendedPatterns(t *testing.T) {
	shouldNotMatch(t, "/path/*.{php}", "/path/file.txt")
	shouldNotMatch(t, "/path/*.{php,twig}", "/path/file.txt")
	shouldNotMatch(t, "/path/{file.php,file.twig}", "/path/file.txt")
	shouldNotMatch(t, "/path/{dir1,dir2}/file.php", "/path/dir3/file.php")
	shouldNotMatch(t, "/path/{dir1,dir2}/**/*.php", "/path/dir1/subpath/file.txt")
}

func TestAnAssociatedEventTriggersTheWatcher(t *testing.T) {
	w := watcher{name: "/**/*.php"}
	require.NoError(t, w.parse())
	w.events = make(chan eventHolder)

	e := &Event{PathName: "/path/temorary_file", AssociatedPathName: "/path/file.php"}
	go w.handle(e)

	assert.Equal(t, e, (<-w.events).event)
}

func relativeDir(t *testing.T, relativePath string) string {
	dir, err := filepath.Abs("./" + relativePath)
	assert.NoError(t, err)
	return dir
}

func hasDir(t *testing.T, pattern string, dir string) {
	t.Helper()

	w := watcher{name: pattern}
	require.NoError(t, w.parse())

	assert.Equal(t, dir, w.name)
}

func shouldMatch(t *testing.T, pattern string, fileName string) {
	t.Helper()

	w := watcher{name: pattern}
	require.NoError(t, w.parse())

	assert.Truef(t, w.allowReload(&Event{PathName: fileName}), "%s %s", pattern, fileName)
}

func shouldNotMatch(t *testing.T, pattern string, fileName string) {
	t.Helper()

	w := watcher{name: pattern}
	require.NoError(t, w.parse())

	assert.Falsef(t, w.allowReload(&Event{PathName: fileName}), "%s %s", pattern, fileName)
}
