package frankenphp

// #include "frankenphp_types.h"
import "C"
import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
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
	internalWorker atomic.Pointer[worker]
}

// running returns the worker started by the running instance and its server
// (the fallback one for a global worker): the handle outlives that instance
func (w *extensionWorkers) running() (*worker, *Server) {
	worker := w.internalWorker.Load()
	switch {
	case worker == nil || worker.stopped.Load():
		return nil, nil
	case worker.server != nil:
		return worker, worker.server
	default:
		return worker, fallbackServer
	}
}

// EXPERIMENTAL: SendRequest sends an HTTP request to the worker and writes the response to the provided ResponseWriter.
func (w *extensionWorkers) SendRequest(rw http.ResponseWriter, r *http.Request) error {
	worker, server := w.running()
	if worker == nil {
		return ErrNotRunning
	}

	// the worker itself, the name may be the empty one it was declared with
	return server.ServeHTTP(rw, r, WithOriginalRequest(r), withWorker(worker))
}

func (w *extensionWorkers) NumThreads() int {
	worker := w.internalWorker.Load()
	if worker == nil {
		return 0
	}

	return worker.countThreads()
}

// nullMessage is passed to the closure as null, unlike a nil message.
type nullMessage struct{}

func (nullMessage) toZval(zval *C.zval) {
	C.__zval_null__(zval)
}

// EXPERIMENTAL: SendMessage sends a message to the worker and waits for a response.
func (w *extensionWorkers) SendMessage(ctx context.Context, message any, rw http.ResponseWriter) (any, error) {
	// no thread is left to take the message once the instance stopped
	worker, server := w.running()
	if worker == nil || !server.isRegistered.Load() {
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

	fc := newContextFromMessage(message, rw, ctx, worker)
	err := worker.handleRequest(fc)

	return fc.handlerReturn, err
}
