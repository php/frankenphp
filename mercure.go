//go:build !nomercure

package frankenphp

// #include <stdint.h>
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
func go_mercure_publish(threadIndex C.uintptr_t, topics *C.struct__zval_struct, data unsafe.Pointer, private bool, id, typ unsafe.Pointer, retry uint64) (generatedID *C.zend_string, error C.short) {
	thread := phpThreads[threadIndex]
	ctx := thread.context()
	fc := thread.frankenPHPContext()

	if fc.mercureHub == nil {
		if fc.logger.Enabled(ctx, slog.LevelError) {
			fc.logger.LogAttrs(ctx, slog.LevelError, "No Mercure hub configured")
		}

		return nil, 1
	}

	u := &mercure.Update{
		Event: mercure.Event{
			Data:  GoString(data),
			ID:    GoString(id),
			Retry: retry,
			Type:  GoString(typ),
		},
		Private: private,
	}

	zvalType := C.zval_get_type(topics)
	switch zvalType {
	case C.IS_STRING:
		u.Topics = []string{GoString(unsafe.Pointer(topics))}
	case C.IS_ARRAY:
		ts, err := GoPackedArray[string](unsafe.Pointer(topics))
		if err != nil {
			if fc.logger.Enabled(ctx, slog.LevelError) {
				fc.logger.LogAttrs(ctx, slog.LevelError, "invalid topics type", slog.Any("error", err))
			}

			return nil, 1
		}

		u.Topics = ts
	default:
		// Never happens as the function is called from C with proper types
		panic("invalid topics type")
	}

	if err := fc.mercureHub.Publish(ctx, u); err != nil {
		if fc.logger.Enabled(ctx, slog.LevelError) {
			fc.logger.LogAttrs(ctx, slog.LevelError, "Unable to publish Mercure update", slog.Any("error", err))
		}

		return nil, 2
	}

	return (*C.zend_string)(PHPString(u.ID, false)), 0
}

// WithMercureHub sets the mercure.Hub to use to publish updates
func WithMercureHub(hub *mercure.Hub) RequestOption {
	return func(o *frankenPHPContext) error {
		o.mercureHub = hub

		return nil
	}
}
