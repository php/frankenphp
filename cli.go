// TODO: ignored on Windows for now (even if it should work with a custom PHP build),
// because static builds of the embed SAPI aren't available yet and php.exe is ship with
// the standard PHP distribution.

package frankenphp

// #include "frankenphp.h"
import "C"
import "unsafe"

// ExecuteScriptCLI executes the PHP script passed as parameter.
// It returns the exit status code of the script.
func ExecuteScriptCLI(script string, args []string) int {
	// Ensure extensions are registered before CLI execution
	registerExtensions()

	cScript := C.CString(script)
	defer C.free(unsafe.Pointer(cScript))

	argc, argv := convertArgs(args)
	defer freeArgs(argv)

	return int(C.frankenphp_execute_script_cli(cScript, argc, (**C.char)(unsafe.Pointer(&argv[0])), false))
}

func ExecutePHPCode(phpCode string) int {
	// Ensure extensions are registered before CLI execution
	registerExtensions()

	cCode := C.CString(phpCode)
	defer C.free(unsafe.Pointer(cCode))
	return int(C.frankenphp_execute_script_cli(cCode, 0, nil, true))
}
