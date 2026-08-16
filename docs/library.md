---
title: Use FrankenPHP as a Go library
description: Embed PHP in any Go program with the FrankenPHP library, serve PHP scripts through net/http, and scope workers to server instances.
---

# Use FrankenPHP as a Go library

FrankenPHP is not only a Caddy module: it can be embedded as a library in any Go program to execute PHP scripts with `net/http`.

The compilation requirements are the same as for [building FrankenPHP from source](compile.md): a PHP built with the embed SAPI and ZTS enabled, and the matching CGO flags.

## Getting started

Create a `Server`, register it while initializing FrankenPHP, and use it as an `http.Handler`:

For a minimal example see [https://pkg.go.dev](https://pkg.go.dev/github.com/dunglas/frankenphp#example-ServeHTTP)

`NewServer()` takes a human-readable name used to attribute workers, metrics and logs to the server (defaults to `server_<idx>` at registration when empty), the document root, the split path suffixes (defaults to `[".php"]`), environment variables made available to every request, and a `*slog.Logger` (defaults to the global logger).

`Init()` starts the PHP runtime and must be called exactly once before serving requests; `Shutdown()` stops it. Calling `Server.ServeHTTP()` before `Init()` or after `Shutdown()` returns `ErrNotRunning`. The same `*Server` may be passed to `Init()` again after a `Shutdown()`, for instance to reload the configuration.

## Multiple servers

Several servers can be registered at once, each with its own document root, environment and logger. This mirrors what multiple `php_server` blocks do in a Caddyfile:

```go
// Registering two servers with separate document roots
api, _ := frankenphp.NewServer("api/public/")
admin, _ := frankenphp.NewServer("admin/public/")

err := frankenphp.Init(
	frankenphp.WithServer(api),
	frankenphp.WithServer(admin),
)
```

Requests served through `api.ServeHTTP()` only see the configuration (and, see below, the workers) of that server.

## Workers

[Worker scripts](worker.md) are declared with `WithWorkers()`. A worker can be scoped to a server with `WithWorkerServerScope()`: only requests handled by this server instance will reach the worker. Requests are matched by script path, or by a custom matcher registered with `WithWorkerMatcher()`:

```go
// Scoping workers to a server
server, _ := frankenphp.NewServer("public/")

err := frankenphp.Init(
	frankenphp.WithServer(server),
	frankenphp.WithWorkers("app", "public/index.php", 4,
		frankenphp.WithWorkerServerScope(server),
	),
	frankenphp.WithWorkers("api", "public/api.php", 2,
		frankenphp.WithWorkerServerScope(server),
		frankenphp.WithWorkerMatcher(func(r *http.Request) bool {
			return strings.HasPrefix(r.URL.Path, "/api/")
		}),
	),
)
```

Workers declared without a server scope are global: they match by file path on any server. Since a global worker has no set of requests to match against, combining `WithWorkerMatcher()` with a global worker is a configuration error and `Init()` rejects it.

## Per-request options

`Server.ServeHTTP()` accepts `RequestOption`s to override the server configuration for a single request, e.g. `WithRequestDocumentRoot()`, `WithRequestSplitPath()`, `WithRequestEnv()` or `WithRequestLogger()`.

## Compatibility with the pre-Server API

The package-level `frankenphp.ServeHTTP()` function keeps working without registering any server: requests prepared with `frankenphp.NewRequestWithContext()` are executed on an internal fallback server carrying the global configuration. New code should prefer explicit `Server` instances.
