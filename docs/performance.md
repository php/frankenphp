# Performance

By default, FrankenPHP tries to offer a good compromise between performance and ease of use.
However, it is possible to substantially improve performance using an appropriate configuration.

## Number of Threads and Workers

By default, FrankenPHP starts 2 times more threads and workers (in worker mode) than the available numbers of CPU.

The appropriate values depend heavily on how your application is written, what it does and your hardware.
We strongly recommend changing these values. For best system stability, it is recommended to have `num_threads` x `memory_limit` < `available_memory`.

To find the right values, it's best to run load tests simulating real traffic.
[k6](https://k6.io) and [Gatling](https://gatling.io) are good tools for this.

To configure the number of threads, use the `num_threads` option of the `php_server` and `php` directives.
To change the number of workers, use the `num` option of the `worker` section of the `frankenphp` directive.

### `max_threads`

While it's always better to know exactly what your traffic will look like, real-life applications tend to be more
unpredictable. The `max_threads` [configuration](config.md#caddyfile-config) allows FrankenPHP to automatically spawn additional threads at runtime up to the specified limit.
`max_threads` can help you figure out how many threads you need to handle your traffic and can make the server more resilient to latency spikes.
If set to `auto`, the limit will be estimated based on the `memory_limit` in your `php.ini`. If not able to do so,
`auto` will instead default to 2x `num_threads`. Keep in mind that `auto` might strongly underestimate the number of threads needed.
`max_threads` is similar to PHP FPM's [pm.max_children](https://www.php.net/manual/en/install.fpm.configuration.php#pm.max-children). The main difference is that FrankenPHP uses threads instead of
processes and automatically delegates them across different worker scripts and 'classic mode' as needed.

## Worker Mode

Enabling [the worker mode](worker.md) dramatically improves performance,
but your app must be adapted to be compatible with this mode:
you need to create a worker script and to be sure that the app is not leaking memory.

## Don't Use musl

The Alpine Linux variant of the official Docker images and the default binaries we provide are using [the musl libc](https://musl.libc.org).

PHP is known to be [slower](https://gitlab.alpinelinux.org/alpine/aports/-/issues/14381) when using this alternative C library instead of the traditional GNU library,
especially when compiled in ZTS mode (thread-safe), which is required for FrankenPHP. The difference can be significant in a heavily threaded environment.

Also, [some bugs only happen when using musl](https://github.com/php/php-src/issues?q=sort%3Aupdated-desc+is%3Aissue+is%3Aopen+label%3ABug+musl).

In production environments, we recommend using FrankenPHP linked against glibc.

This can be achieved by using the Debian Docker images (the default), downloading the -gnu suffix binary from our [Releases](https://github.com/php/frankenphp/releases), or by [compiling FrankenPHP from sources](compile.md).

Alternatively, we provide static musl binaries compiled with [the mimalloc allocator](https://github.com/microsoft/mimalloc), which alleviates the problems in threaded scenarios.

## Go Runtime Configuration

FrankenPHP is written in Go.

In general, the Go runtime doesn't require any special configuration, but in certain circumstances,
specific configuration improves performance.

You likely want to set the `GODEBUG` environment variable to `cgocheck=0` (the default in the FrankenPHP Docker images).

If you run FrankenPHP in containers (Docker, Kubernetes, LXC...) and limit the memory available for the containers,
set the `GOMEMLIMIT` environment variable to the available amount of memory.

For more details, [the Go documentation page dedicated to this subject](https://pkg.go.dev/runtime#hdr-Environment_Variables) is a must-read to get the most out of the runtime.

## `file_server`

By default, the `php_server` directive automatically sets up a file server to
serve static files (assets) stored in the root directory.

This feature is convenient, but comes with a cost.
To disable it, use the following config:

```caddyfile
php_server {
    file_server off
}
```

## `try_files`

Besides static files and PHP files, `php_server` will also try to serve your application's index
and directory index files (`/path/` -> `/path/index.php`). If you don't need directory indices,
you can disable them by explicitly defining `try_files` like this:

```caddyfile
php_server {
    try_files {path} index.php
    root /root/to/your/app # explicitly adding the root here allows for better caching
}
```

This can significantly reduce the number of unnecessary file operations.

An alternate approach with 0 unnecessary file system operations would be to instead use the `php` directive and split
files from PHP by path. This approach works well if your entire application is served by one entry file.
An example [configuration](config.md#caddyfile-config) that serves static files behind an `/assets` folder could look like this:

```caddyfile
route {
    @assets {
        path /assets/*
    }

    # everything behind /assets is handled by the file server
    file_server @assets {
        root /root/to/your/app
    }

    # everything that is not in /assets is handled by your index or worker PHP file
    rewrite index.php
    php {
        root /root/to/your/app # explicitly adding the root here allows for better caching
    }
}
```

## Placeholders

You can use [placeholders](https://caddyserver.com/docs/conventions#placeholders) in the `root` and `env` directives.
However, this prevents caching these values, and comes with a significant performance cost.

If possible, avoid placeholders in these directives.

## `resolve_root_symlink`

By default, if the document root is a symbolic link, it is automatically resolved by FrankenPHP (this is necessary for PHP to work properly).
If the document root is not a symlink, you can disable this feature.

```caddyfile
php_server {
    resolve_root_symlink false
}
```

This will improve performance if the `root` directive contains [placeholders](https://caddyserver.com/docs/conventions#placeholders).
The gain will be negligible in other cases.

## Logs

Logging is obviously very useful, but, by definition,
it requires I/O operations and memory allocations, which considerably reduces performance.
Make sure you [set the logging level](https://caddyserver.com/docs/caddyfile/options#log) correctly,
and only log what's necessary.

## PHP Performance

FrankenPHP uses the official PHP interpreter.
All usual PHP-related performance optimizations apply with FrankenPHP.

In particular:

- check that [OPcache](https://www.php.net/manual/en/book.opcache.php) is installed, enabled and properly configured
- enable [Composer autoloader optimizations](https://getcomposer.org/doc/articles/autoloader-optimization.md)
- ensure that the `realpath` cache is big enough for the needs of your application
- use [preloading](https://www.php.net/manual/en/opcache.preloading.php)

For more details, read [the dedicated Symfony documentation entry](https://symfony.com/doc/current/performance.html)
(most tips are useful even if you don't use Symfony).

## Splitting the thread pool

It is not uncommon for applications to interact with slow external services, like an
API that tends to be unreliable under load or consistently takes 10+ seconds to respond.
In such cases, it can be beneficial to split the thread pool to have dedicated 'slow' pools.

This prevents the slow endpoints from consuming all server resources/threads and
limits the concurrency of requests going towards the slow endpoint, similar to a
connection pool.

```caddyfile

{
    frankenphp {
        max_threads 100 # max 100 threads shared by all workers
    }
}

https://example.com {
    php_server {
        root /app/public # the root of your application
        worker index.php {
            match /slow-endpoint/* # all requests with path /slow-endpoint/* are handled by this thread pool
            num 10 # minimum 10 threads for requests matching /slow-endpoint/*
        }
        worker index.php {
            match * # all other requests are handled separately
            num 20 # minimum 20 threads for other requests, even if the slow endppoints start hanging
        }
    }
}
```

Generally it's also advisable to handle very slow endpoints asynchronously, for example by using
message queues.
