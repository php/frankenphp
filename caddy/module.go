package caddy

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/fileserver"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/rewrite"
	"github.com/dunglas/frankenphp"
	"github.com/dunglas/frankenphp/internal/fastabs"
)

var serverHeader = []string{"FrankenPHP Caddy"}

// FrankenPHPModule represents the "php_server" and "php" directives in the Caddyfile
// they are responsible for forwarding requests to FrankenPHP via "ServeHTTP"
//
//	example.com {
//		php_server {
//			root /var/www/html
//		}
//	}
type FrankenPHPModule struct {
	// Root sets the root folder to the site. Default: `root` directive, or the path of the public directory of the embed app it exists.
	Root string `json:"root,omitempty"`
	// SplitPath sets the substrings for splitting the URI into two parts. The first matching substring will be used to split the "path info" from the path. The first piece is suffixed with the matching substring and will be assumed as the actual resource (CGI script) name. The second piece will be set to PATH_INFO for the CGI script to use. Default: `.php`.
	SplitPath []string `json:"split_path,omitempty"`
	// ResolveRootSymlink enables resolving the `root` directory to its actual value by evaluating a symbolic link, if one exists.
	ResolveRootSymlink *bool `json:"resolve_root_symlink,omitempty"`
	// Env sets an extra environment variable to the given value. Can be specified more than once for multiple environment variables.
	Env map[string]string `json:"env,omitempty"`
	// Workers configures the worker scripts to start.
	Workers []workerConfig `json:"workers,omitempty"`

	resolvedDocumentRoot        string
	preparedEnv                 frankenphp.PreparedEnv
	preparedEnvNeedsReplacement bool
	logger                      *slog.Logger
	mercureHubRequestOption     *frankenphp.RequestOption
}

// CaddyModule returns the Caddy module information.
func (FrankenPHPModule) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.php",
		New: func() caddy.Module { return new(FrankenPHPModule) },
	}
}

// Provision sets up the module.
func (f *FrankenPHPModule) Provision(ctx caddy.Context) error {
	f.logger = ctx.Slogger()
	app, err := ctx.App("frankenphp")
	if err != nil {
		return err
	}
	fapp, ok := app.(*FrankenPHPApp)
	if !ok {
		return fmt.Errorf(`expected ctx.App("frankenphp") to return *FrankenPHPApp, got %T`, app)
	}
	if fapp == nil {
		return fmt.Errorf(`expected ctx.App("frankenphp") to return *FrankenPHPApp, got nil`)
	}

	f.assignMercureHub(ctx)

	for i, wc := range f.Workers {
		// make the file path absolute from the public directory
		// this can only be done if the root is defined inside php_server
		if !filepath.IsAbs(wc.FileName) && f.Root != "" {
			wc.FileName = filepath.Join(f.Root, wc.FileName)
		}

		// Inherit environment variables from the parent php_server directive
		if f.Env != nil {
			wc.inheritEnv(f.Env)
		}

		wc.requestOptions = []frankenphp.RequestOption{frankenphp.WithRequestLogger(f.logger)}

		f.Workers[i] = wc
	}

	workers, err := fapp.addModuleWorkers(f.Workers...)
	if err != nil {
		return err
	}
	f.Workers = workers

	if f.Root == "" {
		if frankenphp.EmbeddedAppPath == "" {
			f.Root = "{http.vars.root}"
		} else {
			rrs := false
			f.Root = filepath.Join(frankenphp.EmbeddedAppPath, defaultDocumentRoot)
			f.ResolveRootSymlink = &rrs
		}
	} else {
		if frankenphp.EmbeddedAppPath != "" && filepath.IsLocal(f.Root) {
			f.Root = filepath.Join(frankenphp.EmbeddedAppPath, f.Root)
		}
	}

	if len(f.SplitPath) == 0 {
		f.SplitPath = []string{".php"}
	}

	if f.ResolveRootSymlink == nil {
		rrs := true
		f.ResolveRootSymlink = &rrs
	}

	if !needReplacement(f.Root) {
		root, err := fastabs.FastAbs(f.Root)
		if err != nil {
			return fmt.Errorf("unable to make the root path absolute: %w", err)
		}
		f.resolvedDocumentRoot = root

		if *f.ResolveRootSymlink {
			root, err := filepath.EvalSymlinks(root)
			if err != nil {
				return fmt.Errorf("unable to resolve root symlink: %w", err)
			}

			f.resolvedDocumentRoot = root
		}
	}

	if f.preparedEnv == nil {
		f.preparedEnv = frankenphp.PrepareEnv(f.Env)

		for _, e := range f.preparedEnv {
			if needReplacement(e) {
				f.preparedEnvNeedsReplacement = true

				break
			}
		}
	}

	return nil
}

// needReplacement checks if a string contains placeholders.
func needReplacement(s string) bool {
	return strings.ContainsAny(s, "{}")
}

// ServeHTTP implements caddyhttp.MiddlewareHandler.
func (f *FrankenPHPModule) ServeHTTP(w http.ResponseWriter, r *http.Request, _ caddyhttp.Handler) error {
	origReq := r.Context().Value(caddyhttp.OriginalRequestCtxKey).(http.Request)
	repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)

	var documentRootOption frankenphp.RequestOption
	var documentRoot string
	if f.resolvedDocumentRoot == "" {
		documentRoot = repl.ReplaceKnown(f.Root, "")
		if documentRoot == "" && frankenphp.EmbeddedAppPath != "" {
			documentRoot = frankenphp.EmbeddedAppPath
		}
		documentRootOption = frankenphp.WithRequestDocumentRoot(documentRoot, *f.ResolveRootSymlink)
	} else {
		documentRoot = f.resolvedDocumentRoot
		documentRootOption = frankenphp.WithRequestResolvedDocumentRoot(documentRoot)
	}

	env := f.preparedEnv
	if f.preparedEnvNeedsReplacement {
		env = make(frankenphp.PreparedEnv, len(f.Env))
		for k, v := range f.preparedEnv {
			env[k] = repl.ReplaceKnown(v, "")
		}
	}

	workerName := ""
	for _, w := range f.Workers {
		if w.matchesPath(r, documentRoot) {
			workerName = w.Name
			break
		}
	}

	var (
		err error
		fr  *http.Request
	)

	if f.mercureHubRequestOption == nil {
		fr, err = frankenphp.NewRequestWithContext(
			r,
			documentRootOption,
			frankenphp.WithRequestSplitPath(f.SplitPath),
			frankenphp.WithRequestPreparedEnv(env),
			frankenphp.WithOriginalRequest(&origReq),
			frankenphp.WithWorkerName(workerName),
		)
	} else {
		fr, err = frankenphp.NewRequestWithContext(
			r,
			documentRootOption,
			frankenphp.WithRequestSplitPath(f.SplitPath),
			frankenphp.WithRequestPreparedEnv(env),
			frankenphp.WithOriginalRequest(&origReq),
			frankenphp.WithWorkerName(workerName),
			*f.mercureHubRequestOption,
		)
	}

	if err != nil {
		return caddyhttp.Error(http.StatusInternalServerError, err)
	}

	// TODO: set caddyhttp.ServerHeader when https://github.com/caddyserver/caddy/pull/7338 will be released
	w.Header()["Server"] = serverHeader
	if err = frankenphp.ServeHTTP(w, fr); err != nil && !errors.As(err, &frankenphp.ErrRejected{}) {
		return caddyhttp.Error(http.StatusInternalServerError, err)
	}

	return nil
}

// UnmarshalCaddyfile implements caddyfile.Unmarshaler.
func (f *FrankenPHPModule) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		for d.NextBlock(0) {
			switch d.Val() {
			case "root":
				if !d.NextArg() {
					return d.ArgErr()
				}
				f.Root = d.Val()

			case "split":
				f.SplitPath = d.RemainingArgs()
				if len(f.SplitPath) == 0 {
					return d.ArgErr()
				}

			case "env":
				args := d.RemainingArgs()
				if len(args) != 2 {
					return d.ArgErr()
				}
				if f.Env == nil {
					f.Env = make(map[string]string)
					f.preparedEnv = make(frankenphp.PreparedEnv)
				}
				f.Env[args[0]] = args[1]
				f.preparedEnv[args[0]+"\x00"] = args[1]

			case "resolve_root_symlink":
				if !d.NextArg() {
					continue
				}
				v, err := strconv.ParseBool(d.Val())
				if err != nil {
					return err
				}
				if d.NextArg() {
					return d.ArgErr()
				}
				f.ResolveRootSymlink = &v

			case "worker":
				wc, err := parseWorkerConfig(d)
				if err != nil {
					return err
				}
				f.Workers = append(f.Workers, wc)

			default:
				return wrongSubDirectiveError("php or php_server", "root, split, env, resolve_root_symlink, worker", d.Val())
			}
		}
	}

	// Check if a worker with this filename already exists in this module
	fileNames := make(map[string]struct{}, len(f.Workers))
	for _, w := range f.Workers {
		if _, ok := fileNames[w.FileName]; ok {
			return fmt.Errorf(`workers in a single "php_server" block must not have duplicate filenames: %q`, w.FileName)
		}

		if len(w.MatchPath) == 0 {
			fileNames[w.FileName] = struct{}{}
		}
	}

	return nil
}

// parseCaddyfile unmarshals tokens from h into a new Middleware.
func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	m := &FrankenPHPModule{}
	err := m.UnmarshalCaddyfile(h.Dispenser)

	return m, err
}

// parsePhpServer parses the php_server directive, which has a similar syntax
// to the php_fastcgi directive. A line such as this:
//
//	php_server
//
// is equivalent to a route consisting of:
//
//		# Add trailing slash for directory requests
//		@canonicalPath {
//		    file {path}/index.php
//		    not path */
//		}
//		redir @canonicalPath {path}/ 308
//
//		# If the requested file does not exist, try index files
//		@indexFiles file {
//		    try_files {path} {path}/index.php index.php
//		    split_path .php
//		}
//		rewrite @indexFiles {http.matchers.file.relative}
//
//		# FrankenPHP!
//		@phpFiles path *.php
//	 	php @phpFiles
//		file_server
//
// parsePhpServer is freely inspired from the php_fastgci directive of the Caddy server (Apache License 2.0, Matthew Holt and The Caddy Authors)
func parsePhpServer(h httpcaddyfile.Helper) ([]httpcaddyfile.ConfigValue, error) {
	if !h.Next() {
		return nil, h.ArgErr()
	}

	// set up FrankenPHP
	phpsrv := FrankenPHPModule{}

	// set up file server
	fsrv := fileserver.FileServer{}
	disableFsrv := false

	// set up the set of file extensions allowed to execute PHP code
	extensions := []string{".php"}

	// set the default index file for the try_files rewrites
	indexFile := "index.php"

	// set up for explicitly overriding try_files
	var tryFiles []string

	// if the user specified a matcher token, use that
	// matcher in a route that wraps both of our routes;
	// either way, strip the matcher token and pass
	// the remaining tokens to the unmarshaler so that
	// we can gain the rest of the directive syntax
	userMatcherSet, err := h.ExtractMatcherSet()
	if err != nil {
		return nil, err
	}

	// make a new dispenser from the remaining tokens so that we
	// can reset the dispenser back to this point for the
	// php unmarshaler to read from it as well
	dispenser := h.NewFromNextSegment()

	// read the subdirectives that we allow as overrides to
	// the php_server shortcut
	// NOTE: we delete the tokens as we go so that the php
	// unmarshal doesn't see these subdirectives which it cannot handle
	for dispenser.Next() {
		for dispenser.NextBlock(0) {
			// ignore any sub-subdirectives that might
			// have the same name somewhere within
			// the php passthrough tokens
			if dispenser.Nesting() != 1 {
				continue
			}

			// parse the php_server subdirectives
			switch dispenser.Val() {
			case "root":
				if !dispenser.NextArg() {
					return nil, dispenser.ArgErr()
				}
				phpsrv.Root = dispenser.Val()
				fsrv.Root = phpsrv.Root
				dispenser.DeleteN(2)

			case "split":
				extensions = dispenser.RemainingArgs()
				dispenser.DeleteN(len(extensions) + 1)
				if len(extensions) == 0 {
					return nil, dispenser.ArgErr()
				}

			case "index":
				args := dispenser.RemainingArgs()
				dispenser.DeleteN(len(args) + 1)
				if len(args) != 1 {
					return nil, dispenser.ArgErr()
				}
				indexFile = args[0]

			case "try_files":
				args := dispenser.RemainingArgs()
				dispenser.DeleteN(len(args) + 1)
				if len(args) < 1 {
					return nil, dispenser.ArgErr()
				}
				tryFiles = args

			case "file_server":
				args := dispenser.RemainingArgs()
				dispenser.DeleteN(len(args) + 1)
				if len(args) < 1 || args[0] != "off" {
					return nil, dispenser.ArgErr()
				}
				disableFsrv = true
			}
		}
	}

	// reset the dispenser after we're done so that the frankenphp
	// unmarshaler can read it from the start
	dispenser.Reset()

	// the rest of the config is specified by the user
	// using the php directive syntax
	dispenser.Next() // consume the directive name
	if err := phpsrv.UnmarshalCaddyfile(dispenser); err != nil {
		return nil, err
	}

	if frankenphp.EmbeddedAppPath != "" {
		if phpsrv.Root == "" {
			phpsrv.Root = filepath.Join(frankenphp.EmbeddedAppPath, defaultDocumentRoot)
			fsrv.Root = phpsrv.Root
			rrs := false
			phpsrv.ResolveRootSymlink = &rrs
		} else if filepath.IsLocal(fsrv.Root) {
			phpsrv.Root = filepath.Join(frankenphp.EmbeddedAppPath, phpsrv.Root)
			fsrv.Root = phpsrv.Root
		}
	}

	// set up a route list that we'll append to
	routes := caddyhttp.RouteList{}

	// prepend routes from the 'worker match *' directives
	routes = prependWorkerRoutes(routes, h, phpsrv, fsrv, disableFsrv)

	// set the list of allowed path segments on which to split
	phpsrv.SplitPath = extensions

	// if the index is turned off, we skip the redirect and try_files
	if indexFile != "off" {
		dirRedir := false
		dirIndex := "{http.request.uri.path}/" + indexFile
		tryPolicy := "first_exist_fallback"

		// if tryFiles wasn't overridden, use a reasonable default
		if len(tryFiles) == 0 {
			if disableFsrv {
				tryFiles = []string{dirIndex, indexFile}
			} else {
				tryFiles = []string{"{http.request.uri.path}", dirIndex, indexFile}
			}

			dirRedir = true
		} else {
			if !strings.HasSuffix(tryFiles[len(tryFiles)-1], ".php") {
				// use first_exist strategy if the last file is not a PHP file
				tryPolicy = ""
			}

			if slices.Contains(tryFiles, dirIndex) {
				dirRedir = true
			}
		}

		// route to redirect to canonical path if index PHP file
		if dirRedir {
			redirMatcherSet := caddy.ModuleMap{
				"file": h.JSON(fileserver.MatchFile{
					TryFiles: []string{dirIndex},
					Root:     phpsrv.Root,
				}),
				"not": h.JSON(caddyhttp.MatchNot{
					MatcherSetsRaw: []caddy.ModuleMap{
						{
							"path": h.JSON(caddyhttp.MatchPath{"*/"}),
						},
					},
				}),
			}
			redirHandler := caddyhttp.StaticResponse{
				StatusCode: caddyhttp.WeakString(strconv.Itoa(http.StatusPermanentRedirect)),
				Headers:    http.Header{"Location": []string{"{http.request.orig_uri.path}/"}},
			}
			redirRoute := caddyhttp.Route{
				MatcherSetsRaw: []caddy.ModuleMap{redirMatcherSet},
				HandlersRaw:    []json.RawMessage{caddyconfig.JSONModuleObject(redirHandler, "handler", "static_response", nil)},
			}

			routes = append(routes, redirRoute)
		}

		// route to rewrite to PHP index file
		rewriteMatcherSet := caddy.ModuleMap{
			"file": h.JSON(fileserver.MatchFile{
				TryFiles:  tryFiles,
				TryPolicy: tryPolicy,
				SplitPath: extensions,
				Root:      phpsrv.Root,
			}),
		}
		rewriteHandler := rewrite.Rewrite{
			URI: "{http.matchers.file.relative}",
		}
		rewriteRoute := caddyhttp.Route{
			MatcherSetsRaw: []caddy.ModuleMap{rewriteMatcherSet},
			HandlersRaw:    []json.RawMessage{caddyconfig.JSONModuleObject(rewriteHandler, "handler", "rewrite", nil)},
		}

		routes = append(routes, rewriteRoute)
	}

	// route to actually pass requests to PHP files;
	// match only requests that are for PHP files
	var pathList []string
	for _, ext := range extensions {
		pathList = append(pathList, "*"+ext)
	}
	phpMatcherSet := caddy.ModuleMap{
		"path": h.JSON(pathList),
	}

	// create the PHP route which is
	// conditional on matching PHP files
	phpRoute := caddyhttp.Route{
		MatcherSetsRaw: []caddy.ModuleMap{phpMatcherSet},
		HandlersRaw:    []json.RawMessage{caddyconfig.JSONModuleObject(phpsrv, "handler", "php", nil)},
	}
	routes = append(routes, phpRoute)

	// create the file server route
	if !disableFsrv {
		fileRoute := caddyhttp.Route{
			MatcherSetsRaw: []caddy.ModuleMap{},
			HandlersRaw:    []json.RawMessage{caddyconfig.JSONModuleObject(fsrv, "handler", "file_server", nil)},
		}
		routes = append(routes, fileRoute)
	}

	subroute := caddyhttp.Subroute{
		Routes: routes,
	}

	// the user's matcher is a prerequisite for ours, so
	// wrap ours in a subroute and return that
	if userMatcherSet != nil {
		return []httpcaddyfile.ConfigValue{
			{
				Class: "route",
				Value: caddyhttp.Route{
					MatcherSetsRaw: []caddy.ModuleMap{userMatcherSet},
					HandlersRaw:    []json.RawMessage{caddyconfig.JSONModuleObject(subroute, "handler", "subroute", nil)},
				},
			},
		}, nil
	}

	// otherwise, return the literal subroute instead of
	// individual routes, to ensure they stay together and
	// are treated as a single unit, without necessarily
	// creating an actual subroute in the output
	return []httpcaddyfile.ConfigValue{
		{
			Class: "route",
			Value: subroute,
		},
	}, nil
}

// workers can also match a path without being in the public directory
// in this case we need to prepend the worker routes to the existing routes
func prependWorkerRoutes(routes caddyhttp.RouteList, h httpcaddyfile.Helper, f FrankenPHPModule, fsrv caddy.Module, disableFsrv bool) caddyhttp.RouteList {
	var allWorkerMatches caddyhttp.MatchPath
	for _, w := range f.Workers {
		for _, path := range w.MatchPath {
			allWorkerMatches = append(allWorkerMatches, path)
		}
	}

	if len(allWorkerMatches) == 0 {
		return routes
	}

	// if there are match patterns, we need to check for files beforehand
	if !disableFsrv {
		routes = append(routes, caddyhttp.Route{
			MatcherSetsRaw: []caddy.ModuleMap{
				{
					"file": h.JSON(fileserver.MatchFile{
						TryFiles: []string{"{http.request.uri.path}"},
						Root:     f.Root,
					}),
					"not": h.JSON(caddyhttp.MatchNot{
						MatcherSetsRaw: []caddy.ModuleMap{
							{"path": h.JSON(caddyhttp.MatchPath{"*.php"})},
						},
					}),
				},
			},
			HandlersRaw: []json.RawMessage{
				caddyconfig.JSONModuleObject(fsrv, "handler", "file_server", nil),
			},
		})
	}

	// forward matching routes to the PHP handler
	routes = append(routes, caddyhttp.Route{
		MatcherSetsRaw: []caddy.ModuleMap{
			{"path": h.JSON(allWorkerMatches)},
		},
		HandlersRaw: []json.RawMessage{
			caddyconfig.JSONModuleObject(f, "handler", "php", nil),
		},
	})

	return routes
}

// Interface guards
var (
	_ caddy.Provisioner           = (*FrankenPHPModule)(nil)
	_ caddyhttp.MiddlewareHandler = (*FrankenPHPModule)(nil)
	_ caddyfile.Unmarshaler       = (*FrankenPHPModule)(nil)
)
