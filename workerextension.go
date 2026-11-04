package frankenphp

// #include "frankenphp_types.h"
import "C"
import (
	"context"
	"fmt"
	"net/http"
	"unsafe"
)

// EXPERIMENTAL: Workers allows you to register a worker.
type Workers interface {
	// SendRequest calls the closure passed to frankenphp_handle_request() and updates the PHP context .
	// The generated HTTP response will be written through the provided writer.
	SendRequest(rw http.ResponseWriter, r *http.Request) error
	// SendMessage calls the closure passed to frankenphp_handle_request(), passes message as a parameter, and returns the value produced by the closure.
	// message can be any type supported by PHPValue(), or an unsafe.Pointer to a zval supported by GoValue().
	// The zval is converted with GoValue() and stays owned by the caller.
	SendMessage(ctx context.Context, message any, rw http.ResponseWriter) (any, error)
	// NumThreads returns the number of available threads.
	NumThreads() int
}

type extensionWorkers struct {
	name           string
	fileName       string
	num            int
	options        []WorkerOption
	internalWorker *worker
}

// server is the one the worker belongs to, the fallback one for a global
// worker, nil before Init()
func (w *extensionWorkers) server() *Server {
	switch {
	case w.internalWorker == nil:
		return nil
	case w.internalWorker.server != nil:
		return w.internalWorker.server
	default:
		return fallbackServer
	}
}

// EXPERIMENTAL: SendRequest sends an HTTP request to the worker and writes the response to the provided ResponseWriter.
func (w *extensionWorkers) SendRequest(rw http.ResponseWriter, r *http.Request) error {
	server := w.server()
	if server == nil {
		return ErrNotRunning
	}

	// the worker itself, the name may be the empty one it was declared with
	return server.ServeHTTP(rw, r, WithOriginalRequest(r), withWorker(w.internalWorker))
}

func (w *extensionWorkers) NumThreads() int {
	if w.internalWorker == nil {
		return 0
	}

	return w.internalWorker.countThreads()
}

// nullMessage is passed to the closure as null, unlike a nil message.
type nullMessage struct{}

func (nullMessage) toZval(zval *C.zval) {
	C.__zval_null__(zval)
}

// EXPERIMENTAL: SendMessage sends a message to the worker and waits for a response.
func (w *extensionWorkers) SendMessage(ctx context.Context, message any, rw http.ResponseWriter) (any, error) {
	// the worker stays referenced past Shutdown(), with no thread left to take the message
	if server := w.server(); server == nil || !server.isRegistered.Load() {
		return nil, ErrNotRunning
	}

	// a zval belongs to the thread that created it (per-thread heap, non-atomic refcount)
	if zval, ok := message.(unsafe.Pointer); ok {
		message = nil

		if zval != nil {
			var err error
			if message, err = GoValue[any](zval); err != nil {
				return nil, fmt.Errorf("cannot convert message: %w", err)
			}

			if message == nil {
				message = nullMessage{}
			}
		}
	}

	fc := newContextFromMessage(message, rw, ctx, w.internalWorker)
	err := w.internalWorker.handleRequest(fc)

	return fc.handlerReturn, err
}
