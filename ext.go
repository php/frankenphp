package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"errors"
	"sync"
	"unsafe"
)

var (
	extensions           []*C.zend_module_entry
	extensionsMu         sync.Mutex
	extensionsRegistered bool
	registerOnce         sync.Once

	// ErrExtensionRegistrationStarted is panicked with when an extension is registered after PHP startup.
	ErrExtensionRegistrationStarted = errors.New("frankenphp: RegisterExtension called after PHP extension registration started")
)

// RegisterExtension registers a new PHP extension.
func RegisterExtension(me unsafe.Pointer) {
	extensionsMu.Lock()
	defer extensionsMu.Unlock()

	if extensionsRegistered {
		panic(ErrExtensionRegistrationStarted)
	}

	extensions = append(extensions, (*C.zend_module_entry)(me))
}

func registerExtensions() {
	registerOnce.Do(func() {
		extensionsMu.Lock()
		defer extensionsMu.Unlock()

		extensionsRegistered = true
		if len(extensions) == 0 {
			return
		}

		C.register_extensions((**C.zend_module_entry)(unsafe.Pointer(&extensions[0])), C.int(len(extensions)))
		extensions = nil
	})
}
