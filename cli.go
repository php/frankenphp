package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"fmt"
	"os"
	"unsafe"
)

// ExecuteScriptCLI executes the PHP script passed as parameter.
// It returns the exit status code of the script.
//
// Only the WithLogger and WithContext options are supported, they configure frankenphp_log().
func ExecuteScriptCLI(script string, args []string, opts ...Option) int {
	o := &opt{}
	for _, f := range opts {
		if err := f(o); err != nil {
			fmt.Fprintln(os.Stderr, err)

			return 1
		}
	}

	if o.ctx != nil {
		globalCtx = o.ctx
	}
	if o.logger != nil {
		globalLogger = o.logger
	}

	// Ensure extensions are registered before CLI execution
	registerExtensions()

	cScript := C.CString(script)
	defer C.free(unsafe.Pointer(cScript))

	argc, argv := convertArgs(args)
	defer freeArgs(argv)

	return int(C.frankenphp_execute_script_cli(cScript, argc, (**C.char)(unsafe.Pointer(&argv[0])), false))
}
