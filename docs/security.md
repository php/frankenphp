---
title: "FrankenPHP security model: trust boundaries between Go and PHP"
description: "Where FrankenPHP's trust boundary lives: which inputs are untrusted, what the Go/C runtime is responsible for, and what falls to the PHP application or to upstream projects."
---

# Security model

This document describes FrankenPHP's trust model: which inputs are trusted, which are not, and where the boundary between them lives.
It is meant to help security audits and automated scanners reason about **FrankenPHP itself**, not the PHP applications it serves.

For the internal mechanics referenced here (threads, the CGO boundary, environment sandboxing), see [Internals](internals.md).
For state persistence in long-running processes, see [Worker Mode](worker.md).

## Trust boundaries

FrankenPHP runs in a stack with four distinct actors:

| Actor                   | Trust                  | Notes                                                                                                                                                |
| ----------------------- | ---------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Remote client**       | Untrusted              | The HTTP request (method, URI, headers, cookies, body, uploads) is the primary source of tainted input.                                             |
| **Operator**            | Trusted                | Supplies the deployment configuration: the `Caddyfile`, environment variables, `php.ini`, installed PHP extensions and Caddy modules, and the application code itself. |
| **PHP application code** | Trusted *by provenance* | Deployed by the operator, so FrankenPHP never executes attacker-supplied code, but this code *consumes* untrusted request data.                    |
| **FrankenPHP (Go + C)** | Trusted computing base | Embeds PHP, transports data in and out, and isolates requests and threads. Its own defects are what this document scopes.                            |

## Code provenance vs. data taint

This is the single most important distinction, and the one that resolves most "do we trust PHP or not?" confusion:

- **Code provenance is trusted.** FrankenPHP only executes PHP files deployed by the operator: the script resolved under the document root, or the configured worker script. It never evaluates code carried in the request itself (body, query string, headers): the SAPI boundary carries *data*, never untrusted *code*.
- **Request data is tainted.** Everything the PHP code reads from the request (`$_GET`, `$_POST`, `$_COOKIE`, `$_FILES`, `$_SERVER`, `php://input`) is untrusted, exactly as in any PHP SAPI. Sanitizing it is the application's job.

So "we trust what comes from PHP" is true for the *code*, while "the SAPI carries untrusted input" is true for the *data*: the two are not in conflict.
FrankenPHP's job is to carry that tainted data faithfully and to keep one request's data from leaking into another.

## What FrankenPHP is responsible for

The trusted computing base has three jobs. Security defects in FrankenPHP live in one of them:

1. **Faithful transport**: map the request into PHP superglobals and `php://input`, and carry PHP's output and headers back to the client, without introducing injection (header/CRLF injection, request smuggling, execution of the wrong file).
2. **Isolation**: keep request-scoped state from crossing between requests, between PHP threads, and between worker iterations.
3. **Memory safety**: manage the CGO boundary (Go ↔ C/PHP) without corrupting memory.

## In scope: FrankenPHP's own attack surface

These are the surfaces FrankenPHP owns. A vulnerability here is a FrankenPHP vulnerability:

- **Request to superglobal mapping** (`cgi.go`, `frankenphp_register_server_vars`): building `$_SERVER`, `REMOTE_ADDR`, `SCRIPT_NAME`, `PATH_INFO`, and the other CGI variables from the request.
- **PHP script-path resolution**: the request path is split on `split_path` (`.php` by default) into `SCRIPT_NAME` / `PATH_INFO`, then joined to the document root with `sanitizedPathJoin` (`filepath.Join(root, filepath.Clean("/"+reqPath))`), which keeps `SCRIPT_FILENAME` from escaping the document root (path traversal). The `php_server` directive additionally sets a default `try_files` rewrite that routes requests to existing files or the front controller, mitigating the classic PHP-FPM pitfall of executing the wrong file.
- **Worker-mode state isolation**: FrankenPHP resets `$_GET`, `$_POST`, `$_COOKIE`, `$_FILES`, `$_SERVER`, and `$_REQUEST` between requests, and explicitly clears `$_SESSION` (which would otherwise leak between requests), but **`$_ENV` is not reset**, and `putenv()` writes, `static` variables, class static properties, and globals persist across requests on the same thread. Request- or user-specific data left in that state can leak into a later request (see [Worker Mode](worker.md#state-persistence)).
- **Per-thread environment sandboxing**: `frankenphp_putenv()` / `frankenphp_getenv()` operate on a thread-local `sandboxed_env` so concurrent threads don't race on the global C environment (see [Internals](internals.md#per-thread-environment-sandboxing)).
- **CGO memory boundary**: Go string pinning and `C.CString()` / `free()` lifetimes across the Go ↔ C boundary.
- **Caddy admin API**: the `/frankenphp/workers/restart` and `/frankenphp/threads` endpoints, exposed through Caddy's admin API (which listens on `localhost:2019` by default). Exposing that endpoint beyond localhost is an operator decision.
- **Trusted proxy handling**: incoming `X-Forwarded-*` headers always reach PHP as tainted `$_SERVER['HTTP_X_FORWARDED_*']` values; they are only trusted to derive the real client IP and scheme when [`trusted_proxies`](production.md#running-behind-a-reverse-proxy) is configured.
- **Slow request bodies**: a client that announces a body then dribbles or stalls it holds the handling thread for the duration. With a bounded thread pool, enough such connections exhaust it (slow-POST DoS). FrankenPHP applies a 60s idle timeout on body reads by default ([`request_body_timeout`](config.md#caddyfile-config)), resetting the deadline before each read so a steady upload of any size succeeds while a stalled one is cut off and the thread released.

## Out of scope

- **Vulnerabilities in the application's PHP code** (SQL injection, XSS, insecure deserialization, etc.). FrankenPHP delivers untrusted request data to the application unchanged; defending against it is the application's responsibility, just as with any SAPI.
- **Flaws in upstream components** used by FrankenPHP (PHP, Caddy, Go) or in projects built on top of it (Laravel Octane, Symfony Runtime). Report those to the relevant project.

## Reporting a vulnerability

See [`SECURITY.md`](../SECURITY.md) for how to report a security issue affecting FrankenPHP.
