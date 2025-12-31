package frankenphp

import (
	"log/slog"
	"net/http"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/dunglas/frankenphp/internal/fastabs"
)

// RequestOption instances allow to configure a FrankenPHP Request.
type RequestOption func(h *frankenPHPContext) error

var (
	documentRootCache    sync.Map
	documentRootCacheLen atomic.Uint32
)

// WithRequestDocumentRoot sets the root directory of the PHP application.
// if resolveSymlink is true, oath declared as root directory will be resolved
// to its absolute value after the evaluation of any symbolic links.
// Due to the nature of PHP opcache, root directory path is cached: when
// using a symlinked directory as root this could generate errors when
// symlink is changed without PHP being restarted; enabling this
// directive will set $_SERVER['DOCUMENT_ROOT'] to the real directory path.
func WithRequestDocumentRoot(documentRoot string, resolveSymlink bool) RequestOption {
	return func(o *frankenPHPContext) (err error) {
		v, ok := documentRootCache.Load(documentRoot)
		if !ok {
			// make sure file root is absolute
			v, err = fastabs.FastAbs(documentRoot)
			if err != nil {
				return err
			}

			// prevent the cache to grow forever, this is a totally arbitrary value
			if documentRootCacheLen.Load() < 1024 {
				documentRootCache.LoadOrStore(documentRoot, v)
				documentRootCacheLen.Add(1)
			}
		}

		if resolveSymlink {
			if v, err = filepath.EvalSymlinks(v.(string)); err != nil {
				return err
			}
		}

		o.documentRoot = v.(string)

		return nil
	}
}

// WithRequestResolvedDocumentRoot is similar to WithRequestDocumentRoot
// but doesn't do any checks or resolving on the path to improve performance.
func WithRequestResolvedDocumentRoot(documentRoot string) RequestOption {
	return func(o *frankenPHPContext) error {
		o.documentRoot = documentRoot

		return nil
	}
}

// WithRequestSplitPath contains a list of split path strings.
//
// The path in the URL will be split into two, with the first piece ending
// with the value of splitPath. The first piece will be assumed as the
// actual resource (CGI script) name, and the second piece will be set to
// PATH_INFO for the CGI script to use.
//
// Future enhancements should be careful to avoid CVE-2019-11043,
// which can be mitigated with use of a try_files-like behavior
// that 404s if the FastCGI path info is not found.
func WithRequestSplitPath(splitPath []string) RequestOption {
	return func(o *frankenPHPContext) error {
		o.splitPath = splitPath

		return nil
	}
}

type PreparedEnv = map[string]string

func PrepareEnv(env map[string]string) PreparedEnv {
	preparedEnv := make(PreparedEnv, len(env))
	for k, v := range env {
		preparedEnv[k+"\x00"] = v
	}

	return preparedEnv
}

// WithRequestEnv set CGI-like environment variables that will be available in $_SERVER.
// Values set with WithEnv always have priority over automatically populated values.
func WithRequestEnv(env map[string]string) RequestOption {
	return WithRequestPreparedEnv(PrepareEnv(env))
}

func WithRequestPreparedEnv(env PreparedEnv) RequestOption {
	return func(o *frankenPHPContext) error {
		o.env = env

		return nil
	}
}

func WithOriginalRequest(r *http.Request) RequestOption {
	return func(o *frankenPHPContext) error {
		o.originalRequest = r

		return nil
	}
}

// WithRequestLogger sets the logger associated with the current request
func WithRequestLogger(logger *slog.Logger) RequestOption {
	return func(o *frankenPHPContext) error {
		o.logger = logger

		return nil
	}
}

// WithWorkerName sets the worker that should handle the request
func WithWorkerName(name string) RequestOption {
	return func(o *frankenPHPContext) error {
		if name != "" {
			o.worker = getWorkerByName(name)
		}

		return nil
	}
}
