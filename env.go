package frankenphp

// #cgo nocallback frankenphp_init_persistent_string
// #cgo noescape frankenphp_init_persistent_string
// #include "frankenphp.h"
// #include "types.h"
import "C"
import (
	"os"
	"strings"
)

//export go_init_os_env
func go_init_os_env(mainThreadEnv *C.zend_array) {
	for _, envVar := range os.Environ() {
		key, val, _ := strings.Cut(envVar, "=")
		zkey := C.frankenphp_init_persistent_string(toUnsafeChar(key), C.size_t(len(key)))
		zStr := C.frankenphp_init_persistent_string(toUnsafeChar(val), C.size_t(len(val)))
		C.__hash_update_string__(mainThreadEnv, zkey, zStr)
	}
}

//export go_putenv
func go_putenv(name *C.char, nameLen C.int, val *C.char, valLen C.int) C.bool {
	goName := C.GoStringN(name, nameLen)

	if val == nil {
		// If no "=" is present, unset the environment variable
		return C.bool(os.Unsetenv(goName) == nil)
	}

	goVal := C.GoStringN(val, valLen)
	return C.bool(os.Setenv(goName, goVal) == nil)
}
