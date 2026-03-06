package frankenphp

// #include "frankenphp.h"
// #include "types.h"
import "C"
import (
	"os"
	"strings"
)

var lengthOfEnv = 0

//export go_init_os_env
func go_init_os_env(mainThreadEnv *C.zend_array) {
	fullEnv := os.Environ()
	lengthOfEnv = len(fullEnv)

	for _, envVar := range fullEnv {
		key, val, _ := strings.Cut(envVar, "=")
		zkey := newPersistentZendString(key)
		zStr := newPersistentZendString(val)
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
