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

//export go_init_os_env
func go_init_os_env(trackVarsArray *C.HashTable) {
	env := os.Environ()
	for _, envVar := range env {
		key, val, _ := strings.Cut(envVar, "=")
		zkey := C.frankenphp_init_persistent_string(toUnsafeChar(key), C.size_t(len(key)))
		zvalStr := C.frankenphp_init_persistent_string(toUnsafeChar(val), C.size_t(len(val)))

		var zval C.zval
		*(*uint32)(unsafe.Pointer(&zval.u1)) = C.IS_INTERNED_STRING_EX
		*(**C.zend_string)(unsafe.Pointer(&zval.value)) = zvalStr
		C.zend_hash_update(trackVarsArray, zkey, &zval)
	}
}

//export go_putenv
func go_putenv(name *C.char, nameLen C.int, val *C.char, valLen C.int) C.bool {
	goName := C.GoStringN(name, nameLen)

	if val == nil {
		// Unset the environment variable
		return C.bool(os.Unsetenv(goName) == nil)
	}

	goVal := C.GoStringN(val, valLen)
	return C.bool(os.Setenv(goName, goVal) == nil)
}
