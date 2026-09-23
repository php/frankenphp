package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"sync"
	"unsafe"
)

var (
	extensions   []*C.zend_module_entry
	registerOnce sync.Once
	// keep the array alive while C holds a raw pointer to it
	registeredExtensions []*C.zend_module_entry
)

// RegisterExtension registers a new PHP extension.
func RegisterExtension(me unsafe.Pointer) {
	extensions = append(extensions, (*C.zend_module_entry)(me))
}

func registerExtensions() {
	if len(extensions) == 0 {
		return
	}

	registerOnce.Do(func() {
		registeredExtensions = extensions
		C.register_extensions((**C.zend_module_entry)(unsafe.Pointer(&extensions[0])), C.int(len(extensions)))
		extensions = nil
	})
}
