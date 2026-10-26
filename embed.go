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
	"path/filepath"
	"runtime"
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
// and need a predictable extraction location. Existing files at this path are
// kept as is, so it must only be writable by the user running the binary.
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
		ownerID, err := embeddedAppOwnerID()
		if err != nil {
			panic(err)
		}

		EmbeddedAppPath = filepath.Join(os.TempDir(), "frankenphp_"+ownerID+"_"+string(embeddedAppChecksum))
		if err := prepareDefaultEmbeddedAppPath(EmbeddedAppPath); err != nil {
			panic(err)
		}
	}

	if err := untar(EmbeddedAppPath); err != nil {
		_ = os.RemoveAll(EmbeddedAppPath)
		panic(err)
	}
}

// prepareDefaultEmbeddedAppPath makes sure that dir is a private directory
// containing the embedded app. Other local users can predict its name, so an
// existing directory is only reused if the current user owns it.
func prepareDefaultEmbeddedAppPath(dir string) error {
	parent := filepath.Dir(dir)
	if err := checkEmbeddedAppParent(parent); err != nil {
		return err
	}

	if info, err := os.Lstat(dir); err == nil {
		return reuseEmbeddedAppPath(dir, info)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	// Extract next to the final path and rename, so concurrent processes never see a partial app
	tmp, err := os.MkdirTemp(parent, filepath.Base(dir)+".tmp")
	if err != nil {
		return err
	}
	if err := extractEmbeddedApp(tmp); err != nil {
		_ = os.RemoveAll(tmp)
		return err
	}

	if err := os.Rename(tmp, dir); err != nil {
		_ = os.RemoveAll(tmp)

		info, lstatErr := os.Lstat(dir)
		if lstatErr != nil {
			return err
		}

		return reuseEmbeddedAppPath(dir, info)
	}

	return nil
}

func extractEmbeddedApp(dir string) error {
	if err := restrictEmbeddedAppPath(dir); err != nil {
		return err
	}

	// untar keeps existing files: refuse anything planted before the directory was restricted
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("embedded app path %q is not empty", dir)
	}

	return untar(dir)
}

func reuseEmbeddedAppPath(dir string, info fs.FileInfo) error {
	if !info.IsDir() {
		return fmt.Errorf("embedded app path %q must be a directory", dir)
	}

	owned, err := embeddedAppPathOwnedByCurrentUser(dir, info)
	if err != nil {
		return err
	}
	if !owned {
		return fmt.Errorf("embedded app path %q must be owned by the current user", dir)
	}

	return restrictEmbeddedAppPath(dir)
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
		if !filepath.IsLocal(f.Name) {
			return fmt.Errorf("tar file contained invalid name %q", f.Name)
		}
		abs := filepath.Join(dir, f.Name)

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
