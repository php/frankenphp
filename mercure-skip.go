//go:build nomercure

package frankenphp

// #include <stdint.h>
// #include <php.h>
import "C"
import (
	"errors"
	"unsafe"
)

type mercureContext struct {
}

//export go_mercure_publish
func go_mercure_publish(_ C.uintptr_t, _ *C.struct__zval_struct, _ unsafe.Pointer, _ bool, _, _ unsafe.Pointer, _ uint64) (*C.zend_string, C.short) {
	return nil, 2
}

// WithMercureHub always return an error when Mercure support is disabled
func WithMercureHub(hub *mercure.Hub) RequestOption {
	return func(o *frankenPHPContext) error {
		return errors.New("mercure support disabled")
	}
}
