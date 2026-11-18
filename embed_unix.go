//go:build !windows

package frankenphp

import (
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"syscall"
)

func embeddedAppOwnerID() (string, error) {
	return strconv.Itoa(os.Geteuid()), nil
}

func embeddedAppPathOwnedByCurrentUser(_ string, info fs.FileInfo) (bool, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)

	return ok && stat.Uid == uint32(os.Geteuid()), nil
}

func restrictEmbeddedAppPath(dir string) error {
	return os.Chmod(dir, 0o700)
}

// checkEmbeddedAppParent rejects parents where other users could swap the app directory.
func checkEmbeddedAppParent(parent string) error {
	info, err := os.Stat(parent)
	if err != nil {
		return err
	}
	if info.Mode().Perm()&0o022 != 0 && info.Mode()&fs.ModeSticky == 0 {
		return fmt.Errorf("embedded app parent directory %q must not be writable by group or others without the sticky bit", parent)
	}

	return nil
}
