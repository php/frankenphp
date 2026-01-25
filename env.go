package frankenphp

// #cgo nocallback frankenphp_init_persistent_string
// #cgo noescape frankenphp_init_persistent_string
// #include "frankenphp.h"
// #include <Zend/zend_API.h>
import "C"
import (
	"os"
	"strings"
	"unsafe"
)

func initializeEnv() map[string]*C.zend_string {
	env := os.Environ()
	envMap := make(map[string]*C.zend_string, len(env))

	for _, envVar := range env {
		key, val, _ := strings.Cut(envVar, "=")
		envMap[key] = C.frankenphp_init_persistent_string(toUnsafeChar(val), C.size_t(len(val)))
	}

	return envMap
}

// get the main thread env or the thread specific env
func getSandboxedEnv(thread *phpThread) map[string]*C.zend_string {
	if thread.sandboxedEnv != nil {
		return thread.sandboxedEnv
	}

	return mainThread.sandboxedEnv
}

func clearSandboxedEnv(thread *phpThread) {
	if thread.sandboxedEnv == nil {
		return
	}

	for key, val := range thread.sandboxedEnv {
		valInMainThread, ok := mainThread.sandboxedEnv[key]
		if !ok || val != valInMainThread {
			C.free(unsafe.Pointer(val))
		}
	}

	thread.sandboxedEnv = nil
}

// if an env var already exists, it needs to be freed
func removeEnvFromThread(thread *phpThread, key string) {
	valueInThread, existsInThread := thread.sandboxedEnv[key]
	if !existsInThread {
		return
	}

	valueInMainThread, ok := mainThread.sandboxedEnv[key]
	if !ok || valueInThread != valueInMainThread {
		C.free(unsafe.Pointer(valueInThread))
	}

	delete(thread.sandboxedEnv, key)
}

// copy the main thread env to the thread specific env
func cloneSandboxedEnv(thread *phpThread) {
	if thread.sandboxedEnv != nil {
		return
	}
	thread.sandboxedEnv = make(map[string]*C.zend_string, len(mainThread.sandboxedEnv))
	for key, value := range mainThread.sandboxedEnv {
		thread.sandboxedEnv[key] = value
	}
}

//export go_putenv
func go_putenv(threadIndex C.uintptr_t, str *C.char, length C.int) C.bool {
	thread := phpThreads[threadIndex]
	envString := C.GoStringN(str, length)
	cloneSandboxedEnv(thread)

	// Check if '=' is present in the string
	if key, val, found := strings.Cut(envString, "="); found {
		removeEnvFromThread(thread, key)
		thread.sandboxedEnv[key] = C.frankenphp_init_persistent_string(toUnsafeChar(val), C.size_t(len(val)))
		return os.Setenv(key, val) == nil
	}

	// No '=', unset the environment variable
	removeEnvFromThread(thread, envString)
	return os.Unsetenv(envString) == nil
}

//export go_getfullenv
func go_getfullenv(threadIndex C.uintptr_t, trackVarsArray *C.zval) {
	thread := phpThreads[threadIndex]
	env := getSandboxedEnv(thread)

	for key, val := range env {
		C.add_assoc_str_ex(trackVarsArray, toUnsafeChar(key), C.size_t(len(key)), val)
	}
}

//export go_getenv
func go_getenv(threadIndex C.uintptr_t, name *C.char) (C.bool, *C.zend_string) {
	thread := phpThreads[threadIndex]

	// Get the environment variable value
	envValue, exists := getSandboxedEnv(thread)[C.GoString(name)]
	if !exists {
		// Environment variable does not exist
		return false, nil // Return 0 to indicate failure
	}

	return true, envValue // Return 1 to indicate success
}
