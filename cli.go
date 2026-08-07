package frankenphp

// #include "frankenphp.h"
import "C"

import (
	"log/slog"
	"unsafe"
)

// cliUsageErrorExitCode is the exit status returned by ExecuteScriptCLI and
// ExecutePHPCode when the call cannot be executed, for instance when FrankenPHP
// is already running in the current process.
const cliUsageErrorExitCode = 1

// refuseCLIWhileRunning reports whether a CLI execution must be refused because
// FrankenPHP is already running in this process. ExecuteScriptCLI/ExecutePHPCode
// start the embedded PHP CLI SAPI, and doing so on top of the already-started
// FrankenPHP SAPI corrupts the PHP engine and crashes the whole process with a
// segmentation fault. Run the PHP CLI in a dedicated process instead.
// See https://github.com/php/frankenphp/issues/2342.
func refuseCLIWhileRunning() bool {
	if !isRunning {
		return false
	}

	globalLogger.LogAttrs(globalCtx, slog.LevelError, "the PHP CLI cannot be executed while FrankenPHP is running; run ExecuteScriptCLI/ExecutePHPCode in a dedicated process instead")

	return true
}

// ExecuteScriptCLI executes the PHP script passed as parameter.
// It returns the exit status code of the script.
//
// It must not be called while FrankenPHP is running in the same process (that is,
// after a successful [Init] and before [Shutdown]): the embedded PHP CLI SAPI
// cannot coexist with the running FrankenPHP SAPI. Run it before [Init], after
// [Shutdown], or in a separate process (for example the same binary in "php-cli"
// mode) instead. While FrankenPHP is running the call is refused: an error is
// logged and a non-zero exit status is returned.
func ExecuteScriptCLI(script string, args []string) int {
	if refuseCLIWhileRunning() {
		return cliUsageErrorExitCode
	}

	// Ensure extensions are registered before CLI execution
	registerExtensions()

	cScript := C.CString(script)
	defer C.free(unsafe.Pointer(cScript))

	argc, argv := convertArgs(args)
	defer freeArgs(argv)

	return int(C.frankenphp_execute_script_cli(cScript, argc, (**C.char)(unsafe.Pointer(&argv[0])), false))
}

// ExecutePHPCode evaluates the PHP code passed as parameter.
// It returns the exit status code of the code.
//
// Like [ExecuteScriptCLI], it must not be called while FrankenPHP is running in
// the same process; in that case the call is refused, an error is logged, and a
// non-zero exit status is returned.
func ExecutePHPCode(phpCode string) int {
	if refuseCLIWhileRunning() {
		return cliUsageErrorExitCode
	}

	// Ensure extensions are registered before CLI execution
	registerExtensions()

	cCode := C.CString(phpCode)
	defer C.free(unsafe.Pointer(cCode))
	return int(C.frankenphp_execute_script_cli(cCode, 0, nil, true))
}
