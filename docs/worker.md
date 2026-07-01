---
title: FrankenPHP worker mode: keep your PHP app in memory
description: Run FrankenPHP in worker mode to keep your PHP application bootstrapped between requests, cut bootstrap overhead, and serve responses in milliseconds.
---

# Using FrankenPHP workers

Boot your application once and keep it in memory.
FrankenPHP will handle incoming requests in a few milliseconds.

## Starting FrankenPHP worker scripts

### Running a FrankenPHP worker with Docker

Set the value of the `FRANKENPHP_CONFIG` environment variable to `worker /path/to/your/worker/script.php`:

```bash
docker run \
    -e FRANKENPHP_CONFIG="worker /app/path/to/your/worker/script.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### Running a FrankenPHP worker with the standalone binary

Use the `--worker` option of the `php-server` command to serve the content of the current directory using a worker:

```bash
frankenphp php-server --worker /path/to/your/worker/script.php
```

If your PHP app is [embedded in the binary](embed.md), you can add a custom `Caddyfile` in the root directory of the app.
It will be used automatically.

It's also possible to [restart the worker on file changes](config.md#watching-for-file-changes) with the `--watch` option.
The following command will trigger a restart if any file ending in `.php` in the `/path/to/your/app/` directory or subdirectories is modified:

```bash
frankenphp php-server --worker /path/to/your/worker/script.php --watch="/path/to/your/app/**/*.php"
```

This feature is often used in combination with [hot reloading](hot-reload.md).

## Worker mode for Symfony

See [the FrankenPHP Symfony worker mode documentation](symfony.md#symfony-worker-mode-with-frankenphp).

## Worker mode for Laravel Octane

See [the FrankenPHP Laravel Octane documentation](laravel.md#laravel-octane).

## Writing a custom FrankenPHP worker script

The following example shows how to create your own worker script without relying on a third-party library:

```php
<?php
// public/index.php

// Boot your app
require __DIR__.'/vendor/autoload.php';

$myApp = new \App\Kernel();
$myApp->boot();

// Handler outside the loop for better performance (doing less work)
$handler = static function () use ($myApp) {
    try {
        // Called when a request is received,
        // superglobals, php://input and the like are reset
        echo $myApp->handle($_GET, $_POST, $_COOKIE, $_FILES, $_SERVER);
    } catch (\Throwable $exception) {
        // `set_exception_handler` is called only when the worker script ends,
        // which may not be what you expect, so catch and handle exceptions here
        (new \MyCustomExceptionHandler)->handleException($exception);
    }
};

$maxRequests = (int)($_SERVER['MAX_REQUESTS'] ?? 0);
for ($nbRequests = 0; !$maxRequests || $nbRequests < $maxRequests; ++$nbRequests) {
    $keepRunning = \frankenphp_handle_request($handler);

    // Do something after sending the HTTP response
    $myApp->terminate();

    // Call the garbage collector to reduce the chances of it being triggered in the middle of a page generation
    gc_collect_cycles();

    if (!$keepRunning) break;
}

// Cleanup
$myApp->shutdown();
```

Then, start your app and use the `FRANKENPHP_CONFIG` environment variable to configure your worker:

```bash
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

By default, 2 workers per CPU are started.
You can also configure the number of workers to start:

```bash
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php 42" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### Restart the worker after a certain number of requests

As PHP was not originally designed for long-running processes, many libraries and legacy code still leak memory.
A workaround to using this type of code in worker mode is to restart the worker script after processing a certain number of requests:

The previous worker snippet allows configuring a maximum number of requests to handle by setting an environment variable named `MAX_REQUESTS`.

### Restart workers manually

While it's possible to restart workers [on file changes](config.md#watching-for-file-changes), it's also possible to restart all workers
gracefully via the [Caddy admin API](https://caddyserver.com/docs/api). If the admin is enabled in your
[Caddyfile](config.md#caddyfile-config), you can ping the restart endpoint with a simple POST request like this:

```bash
curl -X POST http://localhost:2019/frankenphp/workers/restart
```

### Worker failures

If a worker script crashes with a non-zero exit code, FrankenPHP will restart it with an exponential backoff strategy.
If the worker script stays up longer than the last backoff × 2,
it will not penalize the worker script and restart it again.
However, if the worker script continues to fail with a non-zero exit code in a short period of time
(for example, having a typo in a script), FrankenPHP will crash with the error: `too many consecutive failures`.

The number of consecutive failures can be configured in your [Caddyfile](config.md#caddyfile-config) with the `max_consecutive_failures` option:

```caddyfile
frankenphp {
    worker {
        # ...
        max_consecutive_failures 10
    }
}
```

### Request timeout (experimental)

By default a worker thread blocked on a slow external call (a hung MySQL query, a
stuck HTTP client, a Redis call, a long `sleep()`) holds that thread until the call
returns on its own. The `worker_timeout` option sets a hard per-request timeout —
the worker-mode equivalent of PHP-FPM's `request_terminate_timeout` — after which
FrankenPHP interrupts the PHP thread so the request bails out and the worker is
reclaimed:

```caddyfile
frankenphp {
    worker {
        # ...
        worker_timeout 30s
    }
}
```

When the timeout elapses, the request is aborted with a fatal error whose message
is `Worker request timeout of N second(s) exceeded`. The worker script then
restarts cleanly and serves the next request — no special userland code is
required. Note that `max_execution_time` does **not** count time spent inside a
blocking call such as a database query, which is exactly the case `worker_timeout`
is designed to cover.

How it works (and its limits):

- A blocking syscall (a stuck database query, a hung Redis/Elasticsearch/HTTP
  read, a black-holed `connect()`) cannot be aborted by PHP's timeout flag
  alone, because PHP retries the interrupted read. On **Linux**, FrankenPHP
  inspects what the worker thread is blocked on and shuts down the socket(s)
  involved, so the read fails and the request unwinds. Only sockets are
  aborted this way (a read blocked on a file or pipe is not). It recognises:
  - `read`/`recvfrom`/`recvmsg` and a blocking `connect` — the descriptor is the
    syscall's first argument;
  - `poll`/`ppoll` — the descriptors are read out of the poll set (PHP's stream
    layer, and thus most Redis/HTTP/DB clients built on it, always poll before
    reading). This is what lets a stuck `SELECT SLEEP(30)` actually stop at the
    timeout instead of running to completion;
  - `epoll_wait`/`epoll_pwait` — the watched descriptors are enumerated from the
    epoll instance (covers clients running their own event loop, such as
    `curl_multi` and gRPC).

  Every descriptor is confirmed to be a socket before it is shut down.
- A long `sleep()`/`usleep()` (no socket) is interrupted by a realtime signal on
  **Linux and FreeBSD**.
- On **macOS** and **Windows**, and for a tight CPU loop inside a C extension that
  swallows `EINTR`, only PHP's VM-interrupt flag is set: a CPU-bound overrun is
  still caught at the next opcode boundary, but a blocking syscall already in
  progress cannot be unblocked. A client blocked in a `select`-based loop (rare on
  Linux, where `poll` is preferred) is likewise not aborted.
- The socket abort needs no extra privilege (all inspection is of the process
  itself), but it relies on `/proc` and — for poll-based waits, the common case —
  on [`process_vm_readv(2)`](https://man7.org/linux/man-pages/man2/process_vm_readv.2.html).
  Docker's default seccomp profile allows this syscall on kernels ≥ 4.8
  ([moby#42083](https://github.com/moby/moby/pull/42083)); under an older or
  stricter policy (gVisor, custom profiles) the call fails closed: FrankenPHP
  logs a warning once and a request blocked in a poll-based socket read can then
  not be aborted (sleeps and CPU-bound overruns still are).
- `worker_timeout` aborts the request hard, like `request_terminate_timeout`
  does in PHP-FPM. The database server rolls back an open transaction when its
  connection is shut down, and PHP's request shutdown still runs (sessions are
  released as usual). But application-level sequences are not rolled back: an
  e-mail already sent, a file already written or an external lock with a TTL
  stay as they are. Set the timeout comfortably above your slowest legitimate
  request.
- `worker_timeout` defaults to `0` (disabled).

## Superglobals behavior

[PHP superglobals](https://www.php.net/manual/language.variables.superglobals.php) (`$_SERVER`, `$_ENV`, `$_GET`...)
behave as follows:

- before the first call to `frankenphp_handle_request()`, superglobals contain values bound to the worker script itself
- during and after the call to `frankenphp_handle_request()`, superglobals contain values generated from the processed HTTP request, each call to `frankenphp_handle_request()` changes the superglobals values

To access the superglobals of the worker script inside the callback, you must copy them and import the copy in the scope of the callback:

```php
<?php
// Copy worker's $_SERVER superglobal before the first call to frankenphp_handle_request()
$workerServer = $_SERVER;

$handler = static function () use ($workerServer) {
    var_dump($_SERVER); // Request-bound $_SERVER
    var_dump($workerServer); // $_SERVER of the worker script
};

// ...
```

Most superglobals (`$_GET`, `$_POST`, `$_COOKIE`, `$_FILES`, `$_SERVER`, `$_REQUEST`) are automatically reset between requests.
However, **`$_ENV` is currently not reset between requests**.
This means that any modifications made to `$_ENV` during a request will persist and be visible to subsequent requests handled by the same worker thread.
Avoid storing request-specific or sensitive data in `$_ENV`.

## State persistence

Because worker mode keeps the PHP process alive between requests, the following state persists across requests:

- **Static variables**: Variables declared with `static` inside functions or methods retain their values between requests.
- **Class static properties**: Static properties on classes persist between requests.
- **Global variables**: Variables in the global scope of the worker script persist between requests.
- **In-memory caches**: Any data stored in memory (arrays, objects) outside the request handler persists.

This is by design and is what makes worker mode fast. However, it requires attention to avoid unintended side effects:

```php
<?php
function getCounter(): int {
    static $count = 0;
    return ++$count; // Increments across requests!
}

$handler = static function () {
    echo getCounter(); // 1, 2, 3, ... for each request on this thread
};

while (\frankenphp_handle_request($handler)) {
    // ...
}
```

When writing worker scripts, make sure to reset any request-specific state between requests.
Frameworks like [Symfony](symfony.md) and [Laravel Octane](laravel.md) take care of resetting most state for you, but you may still need to reset your own services. With Symfony, services that hold request-specific state should implement [`Symfony\Contracts\Service\ResetInterface`](https://github.com/symfony/contracts/blob/main/Service/ResetInterface.php) so they're reset by the kernel between requests.
