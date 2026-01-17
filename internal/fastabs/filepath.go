//go:build !unix

package fastabs

import (
	"path/filepath"
)

// FastAbs can't be optimized on Windows because the
// syscall.FullPath function takes an input.
func FastAbs(path string) (string, error) {
	// Normalize forward slashes to backslashes for Windows compatibility
	return filepath.Abs(filepath.FromSlash(path))
}
