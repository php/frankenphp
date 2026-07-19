package caddy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/dunglas/frankenphp"
	"github.com/dunglas/frankenphp/internal/fastabs"
)

var (
	options   []frankenphp.Option
	optionsMU sync.RWMutex
)

// EXPERIMENTAL: RegisterWorkers provides a way for extensions to register frankenphp.Workers
func RegisterWorkers(name, fileName string, num int, wo ...frankenphp.WorkerOption) frankenphp.Workers {
	w, opt := frankenphp.WithExtensionWorkers(name, fileName, num, wo...)

	optionsMU.Lock()
	options = append(options, opt)
	optionsMU.Unlock()

	return w
}

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
	// Workers configures the worker scripts to start
	Workers []workerConfig `json:"workers,omitempty"`
	// Overwrites the default php ini configuration
	PhpIni map[string]string `json:"php_ini,omitempty"`
	// The maximum amount of time a request may be stalled waiting for a thread
	MaxWaitTime time.Duration `json:"max_wait_time,omitempty"`
	// The maximum amount of time an autoscaled thread may be idle before being deactivated
	MaxIdleTime time.Duration `json:"max_idle_time,omitempty"`
	// EXPERIMENTAL: MaxRequests sets the maximum number of requests a PHP thread handles before restarting (0 = unlimited)
	MaxRequests int `json:"max_requests,omitempty"`

	opts            []frankenphp.Option
	metrics         frankenphp.Metrics
	ctx             context.Context
	logger          *slog.Logger
	modules         []*FrankenPHPModule
	usedWorkerNames map[string]bool
	httpApp         *caddyhttp.App
}

var errIni = errors.New(`"php_ini" must be in the format: php_ini "<key>" "<value>"`)

// CaddyModule returns the Caddy module information.
func (f FrankenPHPApp) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "frankenphp",
		New: func() caddy.Module { return &f },
	}
}

// Provision sets up the module.
func (f *FrankenPHPApp) Provision(ctx caddy.Context) error {
	f.ctx = ctx
	f.logger = ctx.Slogger()

	// We have at least 7 hardcoded options
	f.opts = make([]frankenphp.Option, 0, 7+len(options))

	if httpApp, err := ctx.AppIfConfigured("http"); err == nil {
		f.httpApp = httpApp.(*caddyhttp.App)
		if f.httpApp.Metrics != nil {
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

func (f *FrankenPHPApp) Start() error {
	defer f.reset() // reset after startup since the app persists across reloads

	repl := caddy.NewReplacer()

	optionsMU.RLock()
	f.opts = append(f.opts, options...)
	optionsMU.RUnlock()

	f.opts = append(f.opts,
		frankenphp.WithContext(f.ctx),
		frankenphp.WithLogger(f.logger),
		frankenphp.WithNumThreads(f.NumThreads),
		frankenphp.WithMaxThreads(f.MaxThreads),
		frankenphp.WithMetrics(f.metrics),
		frankenphp.WithPhpIni(f.PhpIni),
		frankenphp.WithMaxWaitTime(f.MaxWaitTime),
		frankenphp.WithMaxIdleTime(f.MaxIdleTime),
		frankenphp.WithMaxRequests(f.MaxRequests),
	)

	// register global workers
	for _, w := range f.Workers {
		w.FileName = repl.ReplaceKnown(w.FileName, "")
		w.Name = f.createUniqueWorkerName(w)
		f.opts = append(f.opts, frankenphp.WithWorkers(w.Name, w.FileName, w.Num, w.toWorkerOptions()...))
	}

	if err := f.registerModules(repl); err != nil {
		return err
	}

	// if FrankenPHP is currently running, shut it down first
	// this will happen in admin API reloads and caddy tests
	frankenphp.Shutdown()
	if err := frankenphp.Init(f.opts...); err != nil {
		return err
	}

	return nil
}

func (f *FrankenPHPApp) Stop() error {
	if f.logger.Enabled(f.ctx, slog.LevelInfo) {
		f.logger.LogAttrs(f.ctx, slog.LevelInfo, "FrankenPHP stopped 🐘")
	}

	// attempt a graceful shutdown if caddy is exiting
	// note: Exiting() is currently marked as 'experimental'
	// https://github.com/caddyserver/caddy/blob/e76405d55058b0a3e5ba222b44b5ef00516116aa/caddy.go#L810
	if caddy.Exiting() {
		frankenphp.Shutdown()
	}

	// reset global options
	optionsMU.Lock()
	options = nil
	optionsMU.Unlock()

	return nil
}

func (f *FrankenPHPApp) reset() {
	f.Workers = nil
	f.NumThreads = 0
	f.MaxWaitTime = 0
	f.MaxIdleTime = 0
	f.MaxRequests = 0
	f.PhpIni = nil
	f.modules = nil
	f.opts = nil
	f.ctx = nil
	f.metrics = nil
	f.usedWorkerNames = nil
	f.httpApp = nil
}

// register workers and servers for "php" and "php_server" modules
func (f *FrankenPHPApp) registerModules(repl *caddy.Replacer) error {
	modulesByIndex := make(map[int]*FrankenPHPModule)
	for _, module := range f.modules {
		if module.ServerIdx == 0 {
			if err := f.registerModule(repl, module); err != nil {
				return err
			}
			continue
		}

		// modules with the same server_idx should share the same server instance
		// example: the worker { match * } rule adds 2 "php" subroutes to the caddy handler
		// the 2 handlers belong to the same "php_server" and must therefore share workers
		if existingModule, ok := modulesByIndex[module.ServerIdx]; ok {
			module.server = existingModule.server
			continue
		}

		modulesByIndex[module.ServerIdx] = module
		if err := f.registerModule(repl, module); err != nil {
			return err
		}
	}

	return nil
}

// register a server instance and its workers for a single caddy module
func (f *FrankenPHPApp) registerModule(repl *caddy.Replacer, module *FrankenPHPModule) error {
	serverName := f.resolveServerName(module)
	server, err := frankenphp.NewServer(serverName, module.resolvedDocumentRoot, module.SplitPath, module.resolvedEnv, module.logger)
	if err != nil {
		return err
	}

	module.server = server
	f.opts = append(f.opts, frankenphp.WithServer(server))

	for _, w := range module.Workers {
		w.FileName = repl.ReplaceKnown(w.FileName, "")
		w.Name = f.createUniqueWorkerName(w)
		workerOptions := append(w.toWorkerOptions(), frankenphp.WithWorkerServerScope(server))
		f.opts = append(f.opts, frankenphp.WithWorkers(w.Name, w.FileName, w.Num, workerOptions...))
	}

	return nil
}

// avoid name collisions for workers
func (f *FrankenPHPApp) createUniqueWorkerName(wc workerConfig) string {
	if f.usedWorkerNames == nil {
		f.usedWorkerNames = make(map[string]bool)
	}

	if wc.Name == "" {
		wc.Name, _ = fastabs.FastAbs(wc.FileName)
	}

	name := wc.Name
	suffix := 0
	for {
		if _, ok := f.usedWorkerNames[name]; !ok {
			f.usedWorkerNames[name] = true
			break
		}
		suffix++
		name = fmt.Sprintf("%s_%d", wc.Name, suffix)
	}

	return name
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
					return d.Err("max_wait_time must be a valid duration (example: 10s)")
				}

				f.MaxWaitTime = v
			case "max_idle_time":
				if !d.NextArg() {
					return d.ArgErr()
				}

				v, err := time.ParseDuration(d.Val())
				if err != nil {
					return d.Err("max_idle_time must be a valid duration (example: 30s)")
				}

				f.MaxIdleTime = v
			case "max_requests":
				if !d.NextArg() {
					return d.ArgErr()
				}

				v, err := strconv.ParseUint(d.Val(), 10, 32)
				if err != nil {
					return d.WrapErr(err)
				}

				f.MaxRequests = int(v)
			case "php_ini":
				parseIniLine := func(d *caddyfile.Dispenser) error {
					key := d.Val()
					if !d.NextArg() {
						return d.WrapErr(errIni)
					}
					if f.PhpIni == nil {
						f.PhpIni = make(map[string]string)
					}
					f.PhpIni[key] = d.Val()
					if d.NextArg() {
						return d.WrapErr(errIni)
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
						return d.WrapErr(errIni)
					}
					err := parseIniLine(d)
					if err != nil {
						return err
					}
				}

			case "worker":
				wc, err := unmarshalWorker(d)
				if err != nil {
					return err
				}
				if frankenphp.EmbeddedAppPath != "" && filepath.IsLocal(wc.FileName) {
					wc.FileName = filepath.Join(frankenphp.EmbeddedAppPath, wc.FileName)
				}
				// check for duplicate workers
				for _, existingWorker := range f.Workers {
					if existingWorker.FileName == wc.FileName {
						return d.Errf("global workers must not have duplicate filenames: %q", wc.FileName)
					}
				}

				f.Workers = append(f.Workers, wc)
			default:
				return wrongSubDirectiveError("frankenphp", "num_threads, max_threads, php_ini, worker, max_wait_time, max_idle_time, max_requests", d.Val())
			}
		}
	}

	if f.MaxThreads > 0 && f.NumThreads > 0 && f.MaxThreads < f.NumThreads {
		return d.Err(`"max_threads"" must be greater than or equal to "num_threads"`)
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

var (
	_ caddy.App         = (*FrankenPHPApp)(nil)
	_ caddy.Provisioner = (*FrankenPHPApp)(nil)
)
