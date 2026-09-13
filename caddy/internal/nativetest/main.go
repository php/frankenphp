package main

// #cgo linux CFLAGS: -D_GNU_SOURCE
// #include "extension.h"
import "C"
import (
	"os"
	"runtime"
	"sync"
	"unsafe"

	caddycmd "github.com/caddyserver/caddy/v2/cmd"
	_ "github.com/caddyserver/caddy/v2/modules/standard"
	"github.com/dunglas/frankenphp"
	_ "github.com/dunglas/frankenphp/caddy"
)

func init() {
	frankenphp.RegisterExtension(unsafe.Pointer(&C.native_cli_test_module))
	if path := os.Getenv("FRANKENPHP_TEST_EMBEDDED_PATH"); path != "" {
		frankenphp.EmbeddedAppPath = path
	}
}

//export go_frankenphp_native_test
func go_frankenphp_native_test() C.int {
	// Exercise Go initialization, callbacks, and creation of additional OS
	// threads, which must inherit the CLI signal mask too.
	var ready, done sync.WaitGroup
	ready.Add(8)
	done.Add(8)
	release := make(chan struct{})
	for range 8 {
		go func() {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			defer done.Done()
			ready.Done()
			<-release
		}()
	}
	ready.Wait()
	close(release)
	done.Wait()
	return 42
}

func main() {
	caddycmd.Main()
}
