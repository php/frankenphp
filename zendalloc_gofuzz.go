//go:build gofuzz

package frankenphp

import "os"

func init() {
	// Zend's own arena allocator hides bugs from the sanitizers OSS-Fuzz builds with.
	os.Setenv("USE_ZEND_ALLOC", "0")
}
