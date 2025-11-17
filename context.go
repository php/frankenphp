package frankenphp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// frankenPHPContext provides contextual information about the Request to handle.
type frankenPHPContext struct {
	documentRoot    string
	splitPath       []string
	env             PreparedEnv
	logger          *slog.Logger
	request         *http.Request
	originalRequest *http.Request
	worker          *worker

	docURI         string
	pathInfo       string
	scriptName     string
	scriptFilename string

	// Whether the request is already closed by us
	isDone bool

	responseWriter    http.ResponseWriter
	handlerParameters any
	handlerReturn     any

	done      chan any
	startedAt time.Time
}

type contextHolder struct {
	ctx               context.Context
	frankenPHPContext *frankenPHPContext
}

// fromContext extracts the frankenPHPContext from a context.
func fromContext(ctx context.Context) (fctx *frankenPHPContext, ok bool) {
	fctx, ok = ctx.Value(contextKey).(*frankenPHPContext)
	return
}

func newFrankenPHPContext() *frankenPHPContext {
	return &frankenPHPContext{
		done:      make(chan any),
		startedAt: time.Now(),
	}
}

// NewRequestWithContext creates a new FrankenPHP request context.
func NewRequestWithContext(r *http.Request, opts ...RequestOption) (*http.Request, error) {
	fc := newFrankenPHPContext()
	fc.request = r

	for _, o := range opts {
		if err := o(fc); err != nil {
			return nil, err
		}
	}

	if fc.logger == nil {
		fc.logger = globalLogger
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

	// If a worker is already assigned explicitly, use its filename and skip parsing path variables
	if fc.worker != nil {
		fc.scriptFilename = fc.worker.fileName
	} else {
		// If no worker was assigned, split the path into the "traditional" CGI path variables.
		// This needs to already happen here in case a worker script still matches the path.
		splitCgiPath(fc)
	}

	c := context.WithValue(r.Context(), contextKey, fc)

	return r.WithContext(c), nil
}

// newDummyContext creates a fake context from a request path
func newDummyContext(requestPath string, opts ...RequestOption) (*frankenPHPContext, error) {
	r, err := http.NewRequest(http.MethodGet, requestPath, nil)
	if err != nil {
		return nil, err
	}

	fr, err := NewRequestWithContext(r, opts...)
	if err != nil {
		return nil, err
	}

	fc, _ := fromContext(fr.Context())

	return fc, nil
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
		return false
	}

	select {
	case <-fc.request.Context().Done():
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
