//go:build linux

package frankenphp

// #include <stdlib.h>
import "C"
import (
	"os"
	"path/filepath"
	"strings"
)

// go_frankenphp_cli_init is called by the native launcher after Go's package
// initializers have registered extensions, and before PHP starts on the C thread.
// It returns an optional replacement for the script path, owned by the caller.
//
//export go_frankenphp_cli_init
func go_frankenphp_cli_init() *C.char {
	registerExtensions()

	if EmbeddedAppPath != "" && len(os.Args) > 2 {
		script := os.Args[2]
		if !strings.HasPrefix(script, "-") && strings.HasSuffix(script, ".php") {
			if _, err := os.Stat(script); err != nil {
				return C.CString(filepath.Join(EmbeddedAppPath, script))
			}
		}
	}
	return nil
}
