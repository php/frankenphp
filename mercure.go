//go:build !nomercure

package frankenphp

// #include <stdint.h>
// #include "frankenphp.h"
// #include <php.h>
import "C"
import (
	"log/slog"
	"unsafe"

	"github.com/dunglas/mercure"
)

type mercureContext struct {
	mercureHub *mercure.Hub
}

//export go_mercure_publish
func go_mercure_publish(threadIndex C.uintptr_t, topics *C.struct__zval_struct, data *C.zend_string, private bool, id, typ *C.zend_string, retry uint64) (generatedID *C.zend_string, errorMessage *C.char, status C.frankenphp_mercure_status) {
	thread := phpThreads[threadIndex]
	ctx := thread.context()
	fc := thread.frankenPHPContext()

	if fc.mercureHub == nil {
		if fc.logger.Enabled(ctx, slog.LevelError) {
			fc.logger.LogAttrs(ctx, slog.LevelError, "No Mercure hub configured")
		}

		return nil, nil, C.FRANKENPHP_MERCURE_NO_HUB
	}

	u := &mercure.Update{
		Event: mercure.Event{
			Data:  GoString(unsafe.Pointer(data)),
			ID:    GoString(unsafe.Pointer(id)),
			Retry: retry,
			Type:  GoString(unsafe.Pointer(typ)),
		},
		Private: private,
		Debug:   fc.logger.Enabled(ctx, slog.LevelDebug),
	}

	zvalType := C.zval_get_type(topics)
	switch zvalType {
	case C.IS_STRING:
		u.Topics = []string{GoString(unsafe.Pointer(*(**C.zend_string)(unsafe.Pointer(&topics.value[0]))))}
	case C.IS_ARRAY:
		ts, err := GoPackedArray[string](unsafe.Pointer(*(**C.zend_array)(unsafe.Pointer(&topics.value[0]))))
		if err != nil {
			return nil, C.CString(err.Error()), C.FRANKENPHP_MERCURE_INVALID_UPDATE
		}

		u.Topics = ts
	default:
		// Never happens as the function is called from C with proper types
		panic("invalid topics type")
	}

	// The protocol constrains topics, id, type and data: report the violations
	// as argument errors instead of as a failed publication.
	if err := u.Validate(); err != nil {
		return nil, C.CString(err.Error()), C.FRANKENPHP_MERCURE_INVALID_UPDATE
	}

	if err := fc.mercureHub.Publish(ctx, u); err != nil {
		if fc.logger.Enabled(ctx, slog.LevelError) {
			fc.logger.LogAttrs(ctx, slog.LevelError, "Unable to publish Mercure update", slog.Any("error", err))
		}

		return nil, C.CString(err.Error()), C.FRANKENPHP_MERCURE_PUBLISH_FAILED
	}

	return (*C.zend_string)(PHPString(u.ID, false)), nil, C.FRANKENPHP_MERCURE_OK
}

func (w *worker) configureMercure(o *workerOpt) {
	if o.mercureHub == nil {
		return
	}

	w.mercureHub = o.mercureHub
}

// WithMercureHub sets the mercure.Hub to use to publish updates
func WithMercureHub(hub *mercure.Hub) RequestOption {
	return func(o *frankenPHPContext) error {
		o.mercureHub = hub

		return nil
	}
}

// WithWorkerMercureHub sets the mercure.Hub in the worker script and used to dispatch hot reloading-related mercure.Update.
func WithWorkerMercureHub(hub *mercure.Hub) WorkerOption {
	return func(w *workerOpt) error {
		w.mercureHub = hub

		w.requestOptions = append(w.requestOptions, WithMercureHub(hub))

		return nil
	}
}
