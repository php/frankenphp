package frankenphp

import (
	"archive/tar"
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// EmbeddedAppPath contains the path of the embedded PHP application (empty if none).
// It can be set at build time using -ldflags to override the default extraction path:
//
//	go build -ldflags "-X github.com/dunglas/frankenphp.EmbeddedAppPath=/app" ...
//
// When set, the embedded app is extracted to this fixed path instead of a temp
// directory with a checksum suffix. This is useful when the app contains
// pre-compiled artifacts (e.g. OPcache file cache) that reference absolute paths
// and need a predictable extraction location.
var EmbeddedAppPath string

//go:embed app.tar
var embeddedApp []byte

//go:embed app_checksum.txt
var embeddedAppChecksum []byte

func init() {
	if len(embeddedApp) == 0 {
		// No embedded app
		return
	}

	if EmbeddedAppPath == "" {
		EmbeddedAppPath = filepath.Join(os.TempDir(), "frankenphp_"+string(embeddedAppChecksum))
	}

	if err := untar(EmbeddedAppPath); err != nil {
		_ = os.RemoveAll(EmbeddedAppPath)
		panic(err)
	}
}

// untar reads the tar file from r and writes it into dir.
//
// Adapted from https://github.com/golang/build/blob/master/cmd/buildlet/buildlet.go
func untar(dir string) (err error) {
	t0 := time.Now()
	nFiles := 0
	madeDir := map[string]bool{}

	tr := tar.NewReader(bytes.NewReader(embeddedApp))
	loggedChtimesError := false
	for {
		f, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar error: %w", err)
		}
		if f.Typeflag == tar.TypeXGlobalHeader {
			// golang.org/issue/22748: git archive exports
			// a global header ('g') which after Go 1.9
			// (for a bit?) contained an empty filename.
			// Ignore it.
			continue
		}
		rel, err := nativeRelPath(f.Name)
		if err != nil {
			return fmt.Errorf("tar file contained invalid name %q: %v", f.Name, err)
		}
		abs := filepath.Join(dir, rel)

		fi := f.FileInfo()
		mode := fi.Mode()
		switch {
		case mode.IsRegular():
			// Make the directory. This is redundant because it should
			// already be made by a directory entry in the tar
			// beforehand. Thus, don't check for errors; the next
			// write will fail with the same error.
			dir := filepath.Dir(abs)
			if !madeDir[dir] {
				if err := os.MkdirAll(filepath.Dir(abs), mode.Perm()); err != nil {
					return err
				}
				madeDir[dir] = true
			}
			if runtime.GOOS == "darwin" && mode&0111 != 0 {
				// See comment in writeFile.
				err := os.Remove(abs)
				if err != nil && !errors.Is(err, fs.ErrNotExist) {
					return err
				}
			}
			if _, err := os.Stat(abs); os.IsNotExist(err) {
				wf, err := os.OpenFile(abs, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode.Perm())
				if err != nil {
					return err
				}
				n, err := io.Copy(wf, tr)
				if closeErr := wf.Close(); closeErr != nil && err == nil {
					err = closeErr
				}
				if err != nil {
					return fmt.Errorf("error writing to %s: %v", abs, err)
				}
				if n != f.Size {
					return fmt.Errorf("only wrote %d bytes to %s; expected %d", n, abs, f.Size)
				}
				modTime := f.ModTime
				if modTime.After(t0) {
					// Clamp modtimes at system time. See
					// golang.org/issue/19062 when clock on
					// buildlet was behind the gitmirror server
					// doing the git-archive.
					modTime = t0
				}
				if !modTime.IsZero() {
					if err := os.Chtimes(abs, modTime, modTime); err != nil && !loggedChtimesError {
						// benign error. Gerrit doesn't even set the
						// modtime in these, and we don't end up relying
						// on it anywhere (the gomote push command relies
						// on digests only), so this is a little pointless
						// for now.
						log.Printf("error changing modtime: %v (further Chtimes errors suppressed)", err)
						loggedChtimesError = true // once is enough
					}
				}
			}
			nFiles++
		case mode.IsDir():
			if err := os.MkdirAll(abs, mode.Perm()); err != nil {
				return err
			}
			madeDir[abs] = true
		case mode&os.ModeSymlink != 0:
			// TODO: ignore these for now. They were breaking x/build tests.
			// Implement these if/when we ever have a test that needs them.
			// But maybe we'd have to skip creating them on Windows for some builders
			// without permissions.
		default:
			return fmt.Errorf("tar file entry %s contained unsupported file type %v", f.Name, mode)
		}
	}
	return nil
}

// nativeRelPath verifies that p is a non-empty relative path
// using either slashes or the buildlet's native path separator,
// and returns it canonicalized to the native path separator.
func nativeRelPath(p string) (string, error) {
	if p == "" {
		return "", errors.New("path not provided")
	}

	if filepath.Separator != '/' && strings.Contains(p, string(filepath.Separator)) {
		clean := filepath.Clean(p)
		if filepath.IsAbs(clean) {
			return "", fmt.Errorf("path %q is not relative", p)
		}
		if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("path %q refers to a parent directory", p)
		}
		if strings.HasPrefix(p, string(filepath.Separator)) || filepath.VolumeName(clean) != "" {
			// On Windows, this catches semi-relative paths like "C:" (meaning “the
			// current working directory on volume C:”) and "\windows" (meaning “the
			// windows subdirectory of the current drive letter”).
			return "", fmt.Errorf("path %q is relative to volume", p)
		}
		return p, nil
	}

	clean := path.Clean(p)
	if path.IsAbs(clean) {
		return "", fmt.Errorf("path %q is not relative", p)
	}
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("path %q refers to a parent directory", p)
	}
	canon := filepath.FromSlash(p)
	if filepath.VolumeName(canon) != "" {
		return "", fmt.Errorf("path %q begins with a native volume name", p)
	}
	return canon, nil
}
