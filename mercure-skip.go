//go:build nomercure

package frankenphp

// #include <stdint.h>
// #include "frankenphp.h"
// #include <php.h>
import "C"

type mercureContext struct {
}

//export go_mercure_publish
func go_mercure_publish(threadIndex C.uintptr_t, topics *C.struct__zval_struct, data *C.zend_string, private bool, id, typ *C.zend_string, retry uint64) (generatedID *C.zend_string, errorMessage *C.char, status C.frankenphp_mercure_status) {
	return nil, nil, C.FRANKENPHP_MERCURE_UNSUPPORTED
}

func (w *worker) configureMercure(_ *workerOpt) {
}
