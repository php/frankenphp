package frankenphp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// frankenPHPContext provides contextual information about the Request to handle.
type frankenPHPContext struct {
	mercureContext

	ctx          context.Context
	documentRoot string
	splitPath    []string
	env          PreparedEnv
	logger       *slog.Logger
	request      *http.Request
	worker       *worker
	server       *Server

	docURI         string
	pathInfo       string
	scriptName     string
	scriptFilename string
	requestURI     string

	// Whether the request is already closed by us
	isDone bool

	responseWriter     http.ResponseWriter
	responseController *http.ResponseController
	handlerParameters  any
	handlerReturn      any

	done      chan any
	startedAt time.Time
}

// NewRequestWithContext creates a new FrankenPHP request context.
//
// FrankenPHP does not strip request headers whose name contains an underscore.
// Because CGI maps dashes to underscores ("Foo-Bar" becomes the HTTP_FOO_BAR
// variable), a client-supplied "Foo_Bar" header is indistinguishable from the
// legitimate "Foo-Bar" in $_SERVER and can spoof it. This affects any such
// header an application or upstream proxy trusts (forwarded-for, auth, etc.).
// Drop headers containing an underscore before calling this function, unless
// you explicitly need (and whitelist) them. The Caddy-based server and reverse
// proxies such as nginx (underscores_in_headers off) already do this.
func NewRequestWithContext(r *http.Request, opts ...RequestOption) (*http.Request, error) {
	c := context.WithValue(r.Context(), contextKey, opts)

	return r.WithContext(c), nil
}

func newContextFromRequest(request *http.Request, responseWriter http.ResponseWriter, s *Server, opts ...RequestOption) (*frankenPHPContext, error) {
	fc := &frankenPHPContext{
		ctx:            request.Context(),
		done:           make(chan any),
		startedAt:      time.Now(),
		server:         s,
		splitPath:      s.splitPath,
		logger:         globalLogger,
		request:        request,
		documentRoot:   s.root,
		responseWriter: responseWriter,
	}

	for _, o := range opts {
		if err := o(fc); err != nil {
			return nil, err
		}
	}

	// assign a worker directly if it has a request matcher
	if fc.worker == nil {
		for _, w := range s.workersWithRequestMatcher {
			if w.matchRequest(request) {
				fc.worker = w
				break
			}
		}
	}

	if fc.documentRoot == "" {
		if EmbeddedAppPath != "" {
			fc.documentRoot = EmbeddedAppPath
		} else {
			var err error
			if fc.documentRoot, err = os.Getwd(); err != nil {
				return nil, err
			}
		}
	}

	// if no originalRequest was passed, use the URI from the actual request
	// when using Caddy's http module, the original unchanged uri will be used here
	// request.URL is often already rewritten to match a PHP script path
	if fc.requestURI == "" {
		fc.requestURI = fc.request.URL.RequestURI()
	}

	splitCgiPath(fc)

	return fc, nil
}

// newWorkerDummyContext creates a context for worker startup
func newWorkerDummyContext(w *worker) (*frankenPHPContext, error) {
	r, err := http.NewRequestWithContext(globalCtx, http.MethodGet, filepath.Base(w.fileName), nil)
	if err != nil {
		return nil, err
	}

	fc := &frankenPHPContext{
		ctx:       r.Context(),
		server:    w.server,
		request:   r,
		startedAt: time.Now(),
		logger:    globalLogger,
		worker:    w,
	}

	for _, o := range w.requestOptions {
		if err := o(fc); err != nil {
			return nil, err
		}
	}

	if fc.server == nil {
		// global worker, not associated with a server
		fc.server = fallbackServer
	}

	splitCgiPath(fc)

	return fc, nil
}

// newContextFromMessage creates a context from a message (external workers)
func newContextFromMessage(message any, rw http.ResponseWriter, ctx context.Context, w *worker) *frankenPHPContext {
	fc := &frankenPHPContext{
		done:              make(chan any),
		startedAt:         time.Now(),
		server:            w.server,
		worker:            w,
		logger:            globalLogger,
		responseWriter:    rw,
		handlerParameters: message,
		ctx:               ctx,
	}

	if fc.server == nil {
		fc.server = fallbackServer
	}

	if fc.ctx == nil {
		fc.ctx = globalCtx
	}

	return fc
}

// closeContext sends the response to the client
func (fc *frankenPHPContext) closeContext() {
	if fc.isDone {
		return
	}

	close(fc.done)
	fc.isDone = true
}

// validate checks if the request should be outright rejected
func (fc *frankenPHPContext) validate() error {
	if strings.Contains(fc.request.URL.Path, "\x00") {
		fc.reject(ErrInvalidRequestPath)

		return ErrInvalidRequestPath
	}

	contentLengthStr := fc.request.Header.Get("Content-Length")
	if contentLengthStr != "" {
		if contentLength, err := strconv.Atoi(contentLengthStr); err != nil || contentLength < 0 {
			e := fmt.Errorf("%w: %q", ErrInvalidContentLengthHeader, contentLengthStr)

			fc.reject(e)

			return e
		}
	}

	return nil
}

func (fc *frankenPHPContext) clientHasClosed() bool {
	if fc.request == nil {
		return false // not in HTTP context
	}

	select {
	case <-fc.ctx.Done():
		return true
	default:
		return false
	}
}

// reject sends a response with the given status code and error
func (fc *frankenPHPContext) reject(err error) {
	if fc.isDone {
		return
	}

	re := &ErrRejected{}
	if !errors.As(err, re) {
		// Should never happen
		panic("only instance of ErrRejected can be passed to reject")
	}

	rw := fc.responseWriter
	if rw != nil {
		rw.WriteHeader(re.status)
		_, _ = rw.Write([]byte(err.Error()))

		if f, ok := rw.(http.Flusher); ok {
			f.Flush()
		}
	}

	fc.closeContext()
}
