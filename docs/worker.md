# Using FrankenPHP Workers

Boot your application once and keep it in memory.
FrankenPHP will handle incoming requests in a few milliseconds.

## Warning about stateful design

Unlike with the traditional PHP-FPM model, the application remains loaded in memory between requests. Consequently, any state stored in your services (object properties, singletons, etc.) will be preserved and shared across successive requests handled by the same worker. This can lead to data leaks and memory issues, or inconsistent states if your application is not designed for it.
The following article summarizes this issue and explains how to fix it, notably for Symfony applications using ResetInterface to ensure your services are "reset" for every new request.

**Additional Resources:**

- Article: [Getting your Symfony app ready for Swoole, RoadRunner, and FrankenPHP](https://dev.to/sergiid/getting-symfony-app-ready-for-swoole-roadrunner-and-frankenphp-no-ai-involved-2d0g)
- Symfony: [The Messenger](https://symfony.com/doc/current/messenger.html#stateless-worker) documentation also discusses this "stateless worker" concept.
- Tool: [phanalist](https://github.com/denzyldick/phanalist) is a static analyzer that can help you detect "stateful" services in your code.

## Starting Worker Scripts

### Docker

Set the value of the `FRANKENPHP_CONFIG` environment variable to `worker /path/to/your/worker/script.php`:

```console
docker run \
    -e FRANKENPHP_CONFIG="worker /app/path/to/your/worker/script.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### Standalone Binary

Use the `--worker` option of the `php-server` command to serve the content of the current directory using a worker:

```console
frankenphp php-server --worker /path/to/your/worker/script.php
```

If your PHP app is [embedded in the binary](embed.md), you can add a custom `Caddyfile` in the root directory of the app.
It will be used automatically.

It's also possible to [restart the worker on file changes](config.md#watching-for-file-changes) with the `--watch` option.
The following command will trigger a restart if any file ending in `.php` in the `/path/to/your/app/` directory or subdirectories is modified:

```console
frankenphp php-server --worker /path/to/your/worker/script.php --watch="/path/to/your/app/**/*.php"
```

## Symfony Runtime

The worker mode of FrankenPHP is supported by the [Symfony Runtime Component](https://symfony.com/doc/current/components/runtime.html).
To start any Symfony application in a worker, install the FrankenPHP package of [PHP Runtime](https://github.com/php-runtime/runtime):

```console
composer require runtime/frankenphp-symfony
```

Start your app server by defining the `APP_RUNTIME` environment variable to use the FrankenPHP Symfony Runtime:

```console
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php" \
    -e APP_RUNTIME=Runtime\\FrankenPhpSymfony\\Runtime \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

## Laravel Octane

See [the dedicated documentation](laravel.md#laravel-octane).

## Custom Apps

The following example shows how to create your own worker script without relying on a third-party library:

```php
<?php
// public/index.php

// Prevent worker script termination when a client connection is interrupted
ignore_user_abort(true);

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

```console
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

By default, 2 workers per CPU are started.
You can also configure the number of workers to start:

```console
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php 42" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### Restart the Worker After a Certain Number of Requests

As PHP was not originally designed for long-running processes, there are still many libraries and legacy codes that leak memory.
A workaround to using this type of code in worker mode is to restart the worker script after processing a certain number of requests:

The previous worker snippet allows configuring a maximum number of request to handle by setting an environment variable named `MAX_REQUESTS`.

### Restart Workers Manually

While it's possible to restart workers [on file changes](config.md#watching-for-file-changes), it's also possible to restart all workers
gracefully via the [Caddy admin API](https://caddyserver.com/docs/api). If the admin is enabled in your
[Caddyfile](config.md#caddyfile-config), you can ping the restart endpoint with a simple POST request like this:

```console
curl -X POST http://localhost:2019/frankenphp/workers/restart
```

### Worker Failures

If a worker script crashes with a non-zero exit code, FrankenPHP will restart it with an exponential backoff strategy.
If the worker script stays up longer than the last backoff \* 2,
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

## Superglobals Behavior

[PHP superglobals](https://www.php.net/manual/en/language.variables.superglobals.php) (`$_SERVER`, `$_ENV`, `$_GET`...)
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
