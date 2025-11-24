//go:build nomercure

package frankenphp

// #include <stdint.h>
// #include <php.h>
import "C"
import (
	"unsafe"

	"github.com/dunglas/frankenphp/internal/watcher"
)

type mercureContext struct {
}

//export go_mercure_publish
func go_mercure_publish(_ C.uintptr_t, _ *C.struct__zval_struct, _ unsafe.Pointer, _ bool, _, _ unsafe.Pointer, _ uint64) (*C.zend_string, C.short) {
	return nil, 3
}

func (w *worker) publishHotReloadingUpdate() func([]*watcher.Event) {
	return func(events []*watcher.Event) {}
}

func (w *worker) configureMercure(o *workerOpt) {
}
