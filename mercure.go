//go:build !nomercure

package frankenphp

// #include <stdint.h>
// #include <php.h>
import "C"
import (
	"encoding/json"
	"log/slog"
	"unsafe"

	"github.com/dunglas/frankenphp/internal/watcher"
	"github.com/dunglas/mercure"
)

var workersRestarted = make(chan struct{})

type mercureContext struct {
	mercureHub *mercure.Hub
}

//export go_mercure_publish
func go_mercure_publish(threadIndex C.uintptr_t, topics *C.struct__zval_struct, data *C.zend_string, private bool, id, typ *C.zend_string, retry uint64) (generatedID *C.zend_string, error C.short) {
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

func (w *worker) configureMercure(o *workerOpt) {
	if o.mercureHub == nil {
		return
	}

	w.mercureHub = o.mercureHub
}

func broadcastHotReloadEvents() {
	close(workersRestarted)
	workersRestarted = make(chan struct{})
}

// WithHotReload sets files to watch for file changes to trigger a hot reload update.
func WithHotReload(name string, hub *mercure.Hub, patterns []string) Option {
	return func(o *opt) error {
		o.hotReload = append(o.hotReload, &watcher.PatternGroup{
			Patterns: patterns,
			Callback: func(events []*watcher.Event) {
				// Wait for workers to restart before sending the update
				go func() {
					data, err := json.Marshal(events)
					if err != nil {
						if globalLogger.Enabled(globalCtx, slog.LevelError) {
							globalLogger.LogAttrs(globalCtx, slog.LevelError, "error marshaling watcher events", slog.Any("error", err))
						}

						return
					}

					<-workersRestarted
					if err := hub.Publish(globalCtx, &mercure.Update{
						Topics: []string{"https://frankenphp.dev/hot-reload/" + name},
						Event:  mercure.Event{Data: string(data)},
						Debug:  globalLogger.Enabled(globalCtx, slog.LevelDebug),
					}); err != nil && globalLogger.Enabled(globalCtx, slog.LevelError) {
						globalLogger.LogAttrs(globalCtx, slog.LevelError, "error publishing hot reloading Mercure update", slog.Any("error", err))
					}
				}()
			},
		})

		return nil
	}
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
