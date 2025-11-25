//go:build nomercure

package frankenphp

// #include <stdint.h>
// #include <php.h>
import "C"
import (
	"github.com/dunglas/frankenphp/internal/watcher"
)

type mercureContext struct {
}

//export go_mercure_publish
func go_mercure_publish(threadIndex C.uintptr_t, topics *C.struct__zval_struct, data *C.zend_string, private bool, id, typ *C.zend_string, retry uint64) (generatedID *C.zend_string, error C.short) {
	return nil, 3
}

func (w *worker) publishHotReloadingUpdate() func([]*watcher.Event) {
	return func(events []*watcher.Event) {}
}

func (w *worker) configureMercure(o *workerOpt) {
}
