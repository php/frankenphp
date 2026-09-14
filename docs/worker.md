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

### Using PSR-15

If your app speaks [PSR-15](https://www.php-fig.org/psr/psr-15/) instead of superglobals, convert the request/response at the edges of the handler with [`nyholm/psr7`](https://github.com/Nyholm/psr7) and [`nyholm/psr7-server`](https://github.com/Nyholm/psr7-server):

```console
composer require nyholm/psr7 nyholm/psr7-server psr/http-server-handler
```

```php
<?php
// public/index.php

require __DIR__.'/vendor/autoload.php';

use Nyholm\Psr7\Factory\Psr17Factory;
use Nyholm\Psr7Server\ServerRequestCreator;

$myApp = new \App\Kernel(); // implements Psr\Http\Server\RequestHandlerInterface
$myApp->boot();

$psr17Factory = new Psr17Factory();
$creator = new ServerRequestCreator(
    $psr17Factory, // ServerRequestFactory
    $psr17Factory, // UriFactory
    $psr17Factory, // UploadedFileFactory
    $psr17Factory, // StreamFactory
);

$handler = static function () use ($myApp, $creator) {
    $response = $myApp->handle($creator->fromGlobals());

    http_response_code($response->getStatusCode());
    foreach ($response->getHeaders() as $name => $values) {
        foreach ($values as $value) {
            header("$name: $value", false);
        }
    }
    echo $response->getBody();
};

$maxRequests = (int)($_SERVER['MAX_REQUESTS'] ?? 0);
for ($nbRequests = 0; !$maxRequests || $nbRequests < $maxRequests; ++$nbRequests) {
    $keepRunning = \frankenphp_handle_request($handler);
    gc_collect_cycles();
    if (!$keepRunning) break;
}
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

## Background workers

This feature is experimental.

A background worker runs its script in a loop outside the HTTP request cycle, on its own PHP thread. It is declared like any worker, with the `background` option; `name` is required and `num` defaults to one thread:

```caddyfile
php_server {
	worker {
		file jobs.php
		num 1
		name jobs
		background
	}
}
```

The script calls `frankenphp_worker_tick()` once it is set up. The first call marks the worker ready: the server start waits for that point, and an exit before it counts as a failure. Until that call, `max_execution_time` applies as in any request, 30 seconds by default: a setup that outlives it is ended and counts as a boot failure, so raise the limit in `php_ini`, or call `set_time_limit()` in the script, when the setup legitimately takes longer. From the first call on, the run has no time limit, like the CLI. Every call returns `false` once FrankenPHP drains the worker, on shutdown, reboot or restart, and `true` otherwise. It never blocks and never hands out work: like `frankenphp_handle_request()`, it is where the runtime and the script meet. Between two calls, the script waits on the stream returned by `frankenphp_get_worker_handle()`, alone or together with its own streams. The stream becomes readable when FrankenPHP needs the script's attention, and `frankenphp_worker_tick()` consumes whatever was written on it, so the script does not read the stream itself and the stream is quiet again until the next wake-up. It is readable once right after the script starts, so a loop that services it ticks by itself and the worker is ready as soon as its loop runs.

```php
<?php

$handle = frankenphp_get_worker_handle();

while (frankenphp_worker_tick()) {
    $read = [$handle]; // plus the streams the script waits on
    $write = $except = null;
    stream_select($read, $write, $except, 1);

    doSomeWork();
}

// drained: return, FrankenPHP re-runs or stops the script
```

With an event loop, register the stream as readable and call `frankenphp_worker_tick()` from the callback. With [Revolt](https://revolt.run), the loop of amphp:

```php
<?php

use Revolt\EventLoop;

$handle = frankenphp_get_worker_handle();
EventLoop::onReadable($handle, function (): void {
    if (!frankenphp_worker_tick()) {
        // drained: stop the loop, the script returns and FrankenPHP moves on
        EventLoop::getDriver()->stop();
    }
});

// the script's own watchers go here

EventLoop::run();
```

The wake-up sent at start makes the callback run as soon as the loop does, which is when the worker becomes ready. The polling API of PHP 8.6 works the same way: wrap the stream in a `StreamPollHandle`, add it to a context, and call `frankenphp_worker_tick()` when it triggers.

`$_SERVER['FRANKENPHP_WORKER_BACKGROUND']` holds the declared name. `FRANKENPHP_WORKER`, the variable of HTTP workers, is not set, so a script serving both roles tests which of the two is set. Background threads come on top of `num_threads` and `max_threads`; they don't autoscale, so `max_threads` is not allowed on them. From Go, declare one with `WithWorkerBackground()`.

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
