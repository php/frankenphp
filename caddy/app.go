package caddy

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/dunglas/frankenphp"
	"github.com/dunglas/frankenphp/internal/fastabs"
)

// FrankenPHPApp represents the global "frankenphp" directive in the Caddyfile
// it's responsible for starting up the global PHP instance and all threads
//
//	{
//		frankenphp {
//			num_threads 20
//		}
//	}
type FrankenPHPApp struct {
	// NumThreads sets the number of PHP threads to start. Default: 2x the number of available CPUs.
	NumThreads int `json:"num_threads,omitempty"`
	// MaxThreads limits how many threads can be started at runtime. Default 2x NumThreads
	MaxThreads int `json:"max_threads,omitempty"`
	// Workers configures the worker scripts to start.
	Workers []workerConfig `json:"workers,omitempty"`
	// Overwrites the default php ini configuration
	PhpIni map[string]string `json:"php_ini,omitempty"`
	// The maximum amount of time a request may be stalled waiting for a thread
	MaxWaitTime time.Duration `json:"max_wait_time,omitempty"`

	metrics frankenphp.Metrics
	logger  *slog.Logger
}

var iniError = errors.New("'php_ini' must be in the format: php_ini \"<key>\" \"<value>\"")

// CaddyModule returns the Caddy module information.
func (f FrankenPHPApp) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "frankenphp",
		New: func() caddy.Module { return &f },
	}
}

// Provision sets up the module.
func (f *FrankenPHPApp) Provision(ctx caddy.Context) error {
	f.logger = ctx.Slogger()

	if httpApp, err := ctx.AppIfConfigured("http"); err == nil {
		if httpApp.(*caddyhttp.App).Metrics != nil {
			f.metrics = frankenphp.NewPrometheusMetrics(ctx.GetMetricsRegistry())
		}
	} else {
		// if the http module is not configured (this should never happen) then collect the metrics by default
		if errors.Is(err, caddy.ErrNotConfigured) {
			f.metrics = frankenphp.NewPrometheusMetrics(ctx.GetMetricsRegistry())
		} else {
			// the http module failed to provision due to invalid configuration
			return fmt.Errorf("failed to provision caddy http: %w", err)
		}
	}

	return nil
}

func (f *FrankenPHPApp) generateUniqueModuleWorkerName(filepath string) string {
	var i uint
	filepath, _ = fastabs.FastAbs(filepath)
	name := "m#" + filepath

retry:
	for _, wc := range f.Workers {
		if wc.Name == name {
			name = fmt.Sprintf("m#%s_%d", filepath, i)
			i++

			goto retry
		}
	}

	return name
}

func (f *FrankenPHPApp) addModuleWorkers(workers ...workerConfig) ([]workerConfig, error) {
	for i := range workers {
		w := &workers[i]
		if frankenphp.EmbeddedAppPath != "" && filepath.IsLocal(w.FileName) {
			w.FileName = filepath.Join(frankenphp.EmbeddedAppPath, w.FileName)
		}
		if w.Name == "" {
			w.Name = f.generateUniqueModuleWorkerName(w.FileName)
		} else if !strings.HasPrefix(w.Name, "m#") {
			w.Name = "m#" + w.Name
		}
		f.Workers = append(f.Workers, *w)
	}
	return workers, nil
}

func (f *FrankenPHPApp) Start() error {
	repl := caddy.NewReplacer()

	opts := []frankenphp.Option{
		frankenphp.WithNumThreads(f.NumThreads),
		frankenphp.WithMaxThreads(f.MaxThreads),
		frankenphp.WithLogger(f.logger),
		frankenphp.WithMetrics(f.metrics),
		frankenphp.WithPhpIni(f.PhpIni),
		frankenphp.WithMaxWaitTime(f.MaxWaitTime),
	}
	for _, w := range append(f.Workers) {
		workerOpts := []frankenphp.WorkerOption{
			frankenphp.WithWorkerEnv(w.Env),
			frankenphp.WithWorkerWatchMode(w.Watch),
			frankenphp.WithWorkerMaxFailures(w.MaxConsecutiveFailures),
		}

		opts = append(opts, frankenphp.WithWorkers(w.Name, repl.ReplaceKnown(w.FileName, ""), w.Num, workerOpts...))
	}

	frankenphp.Shutdown()
	if err := frankenphp.Init(opts...); err != nil {
		return err
	}

	return nil
}

func (f *FrankenPHPApp) Stop() error {
	f.logger.Info("FrankenPHP stopped 🐘")

	// attempt a graceful shutdown if caddy is exiting
	// note: Exiting() is currently marked as 'experimental'
	// https://github.com/caddyserver/caddy/blob/e76405d55058b0a3e5ba222b44b5ef00516116aa/caddy.go#L810
	if caddy.Exiting() {
		frankenphp.DrainWorkers()
	}

	// reset the configuration so it doesn't bleed into later tests
	f.Workers = nil
	f.NumThreads = 0
	f.MaxWaitTime = 0

	return nil
}

// UnmarshalCaddyfile implements caddyfile.Unmarshaler.
func (f *FrankenPHPApp) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		for d.NextBlock(0) {
			// when adding a new directive, also update the allowedDirectives error message
			switch d.Val() {
			case "num_threads":
				if !d.NextArg() {
					return d.ArgErr()
				}

				v, err := strconv.ParseUint(d.Val(), 10, 32)
				if err != nil {
					return err
				}

				f.NumThreads = int(v)
			case "max_threads":
				if !d.NextArg() {
					return d.ArgErr()
				}

				if d.Val() == "auto" {
					f.MaxThreads = -1
					continue
				}

				v, err := strconv.ParseUint(d.Val(), 10, 32)
				if err != nil {
					return err
				}

				f.MaxThreads = int(v)
			case "max_wait_time":
				if !d.NextArg() {
					return d.ArgErr()
				}

				v, err := time.ParseDuration(d.Val())
				if err != nil {
					return errors.New("max_wait_time must be a valid duration (example: 10s)")
				}

				f.MaxWaitTime = v
			case "php_ini":
				parseIniLine := func(d *caddyfile.Dispenser) error {
					key := d.Val()
					if !d.NextArg() {
						return iniError
					}
					if f.PhpIni == nil {
						f.PhpIni = make(map[string]string)
					}
					f.PhpIni[key] = d.Val()
					if d.NextArg() {
						return iniError
					}

					return nil
				}

				isBlock := false
				for d.NextBlock(1) {
					isBlock = true
					err := parseIniLine(d)
					if err != nil {
						return err
					}
				}

				if !isBlock {
					if !d.NextArg() {
						return iniError
					}
					err := parseIniLine(d)
					if err != nil {
						return err
					}
				}

			case "worker":
				wc, err := parseWorkerConfig(d)
				if err != nil {
					return err
				}
				if frankenphp.EmbeddedAppPath != "" && filepath.IsLocal(wc.FileName) {
					wc.FileName = filepath.Join(frankenphp.EmbeddedAppPath, wc.FileName)
				}
				if strings.HasPrefix(wc.Name, "m#") {
					return fmt.Errorf(`global worker names must not start with "m#": %q`, wc.Name)
				}
				// check for duplicate workers
				for _, existingWorker := range f.Workers {
					if existingWorker.FileName == wc.FileName {
						return fmt.Errorf("global workers must not have duplicate filenames: %q", wc.FileName)
					}
				}

				f.Workers = append(f.Workers, wc)
			default:
				allowedDirectives := "num_threads, max_threads, php_ini, worker, max_wait_time"
				return wrongSubDirectiveError("frankenphp", allowedDirectives, d.Val())
			}
		}
	}

	if f.MaxThreads > 0 && f.NumThreads > 0 && f.MaxThreads < f.NumThreads {
		return errors.New(`"max_threads"" must be greater than or equal to "num_threads"`)
	}

	return nil
}

func parseGlobalOption(d *caddyfile.Dispenser, _ any) (any, error) {
	app := &FrankenPHPApp{}
	if err := app.UnmarshalCaddyfile(d); err != nil {
		return nil, err
	}

	// tell Caddyfile adapter that this is the JSON for an app
	return httpcaddyfile.App{
		Name:  "frankenphp",
		Value: caddyconfig.JSON(app, nil),
	}, nil
}
