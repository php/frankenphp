package caddy

import (
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/dunglas/frankenphp"
	"github.com/dunglas/frankenphp/internal/fastabs"
)

// workerConfig represents the "worker" directive in the Caddyfile
// it can appear in the "frankenphp", "php_server" and "php" directives
//
//	frankenphp {
//		worker {
//			name "my-worker"
//			file "my-worker.php"
//		}
//	}
type workerConfig struct {
	// Name for the worker. Default: the filename for FrankenPHPApp workers, always prefixed with "m#" for FrankenPHPModule workers.
	Name string `json:"name,omitempty"`
	// FileName sets the path to the worker script.
	FileName string `json:"file_name,omitempty"`
	// Num sets the number of workers to start.
	Num int `json:"num,omitempty"`
	// MaxThreads sets the maximum number of threads for this worker.
	MaxThreads int `json:"max_threads,omitempty"`
	// Env sets an extra environment variable to the given value. Can be specified more than once for multiple environment variables.
	Env map[string]string `json:"env,omitempty"`
	// Directories to watch for file changes
	Watch []string `json:"watch,omitempty"`
	// The path to match against the worker
	MatchPath []string `json:"match_path,omitempty"`
	// MaxConsecutiveFailures sets the maximum number of consecutive failures before panicking (defaults to 6, set to -1 to never panick)
	MaxConsecutiveFailures int `json:"max_consecutive_failures,omitempty"`

	requestOptions     []frankenphp.RequestOption
}

func parseWorkerConfig(d *caddyfile.Dispenser) (workerConfig, error) {
	wc := workerConfig{}
	if d.NextArg() {
		wc.FileName = d.Val()
	}

	if d.NextArg() {
		if d.Val() == "watch" {
			wc.Watch = append(wc.Watch, defaultWatchPattern)
		} else {
			v, err := strconv.ParseUint(d.Val(), 10, 32)
			if err != nil {
				return wc, err
			}

			wc.Num = int(v)
		}
	}

	if d.NextArg() {
		return wc, d.Errf(`FrankenPHP: too many "worker" arguments: %s`, d.Val())
	}

	for d.NextBlock(1) {
		v := d.Val()
		switch v {
		case "name":
			if !d.NextArg() {
				return wc, d.ArgErr()
			}
			wc.Name = d.Val()
		case "file":
			if !d.NextArg() {
				return wc, d.ArgErr()
			}
			wc.FileName = d.Val()
		case "num":
			if !d.NextArg() {
				return wc, d.ArgErr()
			}

			v, err := strconv.ParseUint(d.Val(), 10, 32)
			if err != nil {
				return wc, d.WrapErr(err)
			}

			wc.Num = int(v)
		case "max_threads":
			if !d.NextArg() {
				return wc, d.ArgErr()
			}

			v, err := strconv.ParseUint(d.Val(), 10, 32)
			if err != nil {
				return wc, d.WrapErr(err)
			}

			wc.MaxThreads = int(v)
		case "env":
			args := d.RemainingArgs()
			if len(args) != 2 {
				return wc, d.ArgErr()
			}
			if wc.Env == nil {
				wc.Env = make(map[string]string)
			}
			wc.Env[args[0]] = args[1]
		case "watch":
			if !d.NextArg() {
				// the default if the watch directory is left empty:
				wc.Watch = append(wc.Watch, defaultWatchPattern)
			} else {
				wc.Watch = append(wc.Watch, d.Val())
			}
		case "match":
			// provision the path so it's identical to Caddy match rules
			// see: https://github.com/caddyserver/caddy/blob/master/modules/caddyhttp/matchers.go
			caddyMatchPath := (caddyhttp.MatchPath)(d.RemainingArgs())
			if err := caddyMatchPath.Provision(caddy.Context{}); err != nil {
				return wc, d.WrapErr(err)
			}

			wc.MatchPath = caddyMatchPath
		case "max_consecutive_failures":
			if !d.NextArg() {
				return wc, d.ArgErr()
			}

			v, err := strconv.Atoi(d.Val())
			if err != nil {
				return wc, d.WrapErr(err)
			}
			if v < -1 {
				return wc, d.Errf("max_consecutive_failures must be >= -1")
			}

			wc.MaxConsecutiveFailures = v
		default:
			return wc, wrongSubDirectiveError("worker", "name, file, num, env, watch, match, max_consecutive_failures, max_threads", v)
		}
	}

	if wc.FileName == "" {
		return wc, d.Err(`the "file" argument must be specified`)
	}

	if frankenphp.EmbeddedAppPath != "" && filepath.IsLocal(wc.FileName) {
		wc.FileName = filepath.Join(frankenphp.EmbeddedAppPath, wc.FileName)
	}

	return wc, nil
}

func (wc *workerConfig) inheritEnv(env map[string]string) {
	if wc.Env == nil {
		wc.Env = make(map[string]string, len(env))
	}
	for k, v := range env {
		// do not overwrite existing environment variables
		if _, exists := wc.Env[k]; !exists {
			wc.Env[k] = v
		}
	}
}

func (wc *workerConfig) matchesPath(r *http.Request, documentRoot string) bool {

	// try to match against a pattern if one is assigned
	if len(wc.MatchPath) != 0 {
		return (caddyhttp.MatchPath)(wc.MatchPath).Match(r)
	}

	// if there is no pattern, try to match against the actual path (in the public directory)
	fullScriptPath, _ := fastabs.FastAbs(documentRoot + "/" + r.URL.Path)
	absFileName, _ := fastabs.FastAbs(wc.FileName)

	return fullScriptPath == absFileName
}
