package frankenphp

import (
	"log/slog"
	"net/http"
	"os"
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

	responseWriter http.ResponseWriter

	done      chan any
	startedAt time.Time
}

// newFrankenPHPContext creates a new FrankenPHP request context.
func newFrankenPHPContext(rw http.ResponseWriter, r *http.Request, opts ...RequestOption) (*frankenPHPContext, error) {
	fc := &frankenPHPContext{
		done:           make(chan any),
		startedAt:      time.Now(),
		request:        r,
		responseWriter: rw,
	}
	for _, o := range opts {
		if err := o(fc); err != nil {
			return nil, err
		}
	}

	if fc.logger == nil {
		fc.logger = logger
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

	if fc.splitPath == nil {
		fc.splitPath = []string{".php"}
	}

	if fc.env == nil {
		fc.env = make(map[string]string)
	}

	if splitPos := splitPos(fc, r.URL.Path); splitPos > -1 {
		fc.docURI = r.URL.Path[:splitPos]
		fc.pathInfo = r.URL.Path[splitPos:]

		// Strip PATH_INFO from SCRIPT_NAME
		fc.scriptName = strings.TrimSuffix(r.URL.Path, fc.pathInfo)

		// Ensure the SCRIPT_NAME has a leading slash for compliance with RFC3875
		// Info: https://tools.ietf.org/html/rfc3875#section-4.1.13
		if fc.scriptName != "" && !strings.HasPrefix(fc.scriptName, "/") {
			fc.scriptName = "/" + fc.scriptName
		}
	}

	// if a worker is assigned explicitly, use its filename
	// determine if the filename belongs to a worker otherwise
	if fc.worker != nil {
		fc.scriptFilename = fc.worker.fileName
	} else {
		// SCRIPT_FILENAME is the absolute path of SCRIPT_NAME
		fc.scriptFilename = sanitizedPathJoin(fc.documentRoot, fc.scriptName)
		fc.worker = getWorkerByPath(fc.scriptFilename)
	}

	return fc, nil
}

func newDummyContext(requestPath string, opts ...RequestOption) (*frankenPHPContext, error) {
	r, err := http.NewRequest(http.MethodGet, requestPath, nil)
	if err != nil {
		return nil, err
	}

	fc, err := newFrankenPHPContext(nil, r, opts...)
	if err != nil {
		return nil, err
	}

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
func (fc *frankenPHPContext) validate() bool {
	if !strings.Contains(fc.request.URL.Path, "\x00") {
		return true
	}

	fc.rejectBadRequest("Invalid request path")

	return false
}

func (fc *frankenPHPContext) clientHasClosed() bool {
	select {
	case <-fc.request.Context().Done():
		return true
	default:
		return false
	}
}

// reject sends a response with the given status code and message
func (fc *frankenPHPContext) reject(statusCode int, message string) {
	if fc.isDone {
		return
	}

	rw := fc.responseWriter
	if rw != nil {
		rw.WriteHeader(statusCode)
		_, _ = rw.Write([]byte(message))

		if f, ok := rw.(http.Flusher); ok {
			f.Flush()
		}
	}

	fc.closeContext()
}

func (fc *frankenPHPContext) rejectBadRequest(message string) {
	fc.reject(http.StatusBadRequest, message)
}
