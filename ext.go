package frankenphp

//#include "frankenphp.h"
import "C"
import (
	"sync"
	"unsafe"
)

var (
	extensions   []*C.zend_module_entry
	registerOnce sync.Once
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
		C.register_extensions(extensions[0], C.int(len(extensions)))
		extensions = nil
	})
}
