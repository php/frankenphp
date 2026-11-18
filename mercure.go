//go:build !nomercure

package frankenphp

// #include <stdint.h>
// #include "frankenphp.h"
// #include <php.h>
import "C"
import (
	"errors"
	"log/slog"
	"unsafe"

	"github.com/dunglas/mercure"
)

type mercureContext struct {
	mercureHub *mercure.Hub
}

//export go_mercure_publish
func go_mercure_publish(threadIndex C.uintptr_t, topics *C.struct__zval_struct, data *C.zend_string, private bool, id, typ *C.zend_string, retry uint64) (generatedID *C.zend_string, errorMessage *C.char, status C.frankenphp_mercure_status, argument C.uint32_t) {
	thread := phpThreads[threadIndex]
	fc := thread.handler.frankenPHPContext()

	if fc.mercureHub == nil {
		if fc.logger.Enabled(fc.ctx, slog.LevelError) {
			fc.logger.LogAttrs(fc.ctx, slog.LevelError, "No Mercure hub configured")
		}

		return nil, nil, C.FRANKENPHP_MERCURE_NO_HUB, 0
	}

	u := &mercure.Update{
		Event: mercure.Event{
			Data:  GoString(unsafe.Pointer(data)),
			ID:    GoString(unsafe.Pointer(id)),
			Retry: retry,
			Type:  GoString(unsafe.Pointer(typ)),
		},
		Private: private,
		Debug:   fc.logger.Enabled(fc.ctx, slog.LevelDebug),
	}

	zvalType := C.zval_get_type(topics)
	switch zvalType {
	case C.IS_STRING:
		u.Topics = []string{GoString(unsafe.Pointer(*(**C.zend_string)(unsafe.Pointer(&topics.value[0]))))}
	case C.IS_ARRAY:
		ts, err := GoPackedArray[string](unsafe.Pointer(*(**C.zend_array)(unsafe.Pointer(&topics.value[0]))))
		if err != nil {
			return nil, C.CString(err.Error()), C.FRANKENPHP_MERCURE_INVALID_UPDATE, 1
		}

		u.Topics = ts
	default:
		// Never happens as the function is called from C with proper types
		panic("invalid topics type")
	}

	if err := fc.mercureHub.Publish(fc.ctx, u); err != nil {
		// The hub validates the update before dispatching it: report a violation
		// of the protocol as an error of the argument at fault.
		if argument := invalidMercureArgument(err); argument != 0 {
			return nil, C.CString(err.Error()), C.FRANKENPHP_MERCURE_INVALID_UPDATE, argument
		}

		if fc.logger.Enabled(fc.ctx, slog.LevelError) {
			fc.logger.LogAttrs(fc.ctx, slog.LevelError, "Unable to publish Mercure update", slog.Any("error", err))
		}

		return nil, C.CString(err.Error()), C.FRANKENPHP_MERCURE_PUBLISH_FAILED, 0
	}

	return (*C.zend_string)(PHPString(u.ID, false)), nil, C.FRANKENPHP_MERCURE_OK, 0
}

// invalidMercureArgument maps a validation error of the hub to the position of
// the mercure_publish() argument at fault, 0 for any other error.
func invalidMercureArgument(err error) C.uint32_t {
	switch {
	case errors.Is(err, mercure.ErrMissingTopic),
		errors.Is(err, mercure.ErrTooManyTopics),
		errors.Is(err, mercure.ErrInvalidTopic),
		errors.Is(err, mercure.ErrReservedTopic),
		errors.Is(err, mercure.ErrReservedWildcard):
		return 1
	case errors.Is(err, mercure.ErrInvalidData):
		return 2
	case errors.Is(err, mercure.ErrInvalidEventID):
		return 4
	case errors.Is(err, mercure.ErrInvalidEventType),
		errors.Is(err, mercure.ErrReservedEventType):
		return 5
	}

	return 0
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
