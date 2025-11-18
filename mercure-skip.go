//go:build nomercure

package frankenphp

// #include <stdint.h>
// #include <php.h>
import "C"
import (
	"unsafe"
)

type mercureContext struct {
}

//export go_mercure_publish
func go_mercure_publish(_ C.uintptr_t, _ *C.struct__zval_struct, _ unsafe.Pointer, _ bool, _, _ unsafe.Pointer, _ uint64) (*C.zend_string, C.short) {
	return nil, 3
}
