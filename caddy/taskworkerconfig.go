package caddy

import (
	"errors"
	"path/filepath"
	"strconv"

	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/dunglas/frankenphp"
)

// taskWorkerConfig represents the "task_worker" directive in the Caddyfile
//
//	frankenphp {
//		task_worker {
//			name "my-worker"
//			file "my-worker.php"
//		}
//	}
type taskWorkerConfig struct {
	// Name for the worker. Default: the filename for FrankenPHPApp workers, always prefixed with "m#" for FrankenPHPModule workers.
	Name string `json:"name,omitempty"`
	// FileName sets the path to the worker script.
	FileName string `json:"file_name,omitempty"`
	// Num sets the number of workers to start.
	Num int `json:"num,omitempty"`
	// Env sets an extra environment variable to the given value. Can be specified more than once for multiple environment variables.
	Env map[string]string `json:"env,omitempty"`
}

func parseTaskWorkerConfig(d *caddyfile.Dispenser) (taskWorkerConfig, error) {
	wc := taskWorkerConfig{}
	if d.NextArg() {
		wc.FileName = d.Val()
	}

	if d.NextArg() {
		return wc, errors.New(`FrankenPHP: too many "task_worker" arguments: ` + d.Val())
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
				return wc, err
			}

			wc.Num = int(v)
		case "env":
			args := d.RemainingArgs()
			if len(args) != 2 {
				return wc, d.ArgErr()
			}
			if wc.Env == nil {
				wc.Env = make(map[string]string)
			}
			wc.Env[args[0]] = args[1]
		default:
			allowedDirectives := "name, file, num, env"
			return wc, wrongSubDirectiveError("worker", allowedDirectives, v)
		}
	}

	if wc.FileName == "" {
		return wc, errors.New(`the "file" argument for "task_worker" must be specified`)
	}

	if frankenphp.EmbeddedAppPath != "" && filepath.IsLocal(wc.FileName) {
		wc.FileName = filepath.Join(frankenphp.EmbeddedAppPath, wc.FileName)
	}

	return wc, nil
}
