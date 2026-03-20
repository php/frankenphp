# Background Workers

Background workers are long-running PHP scripts that run outside the HTTP request cycle.
They observe their environment and publish configuration that HTTP [workers](worker.md) can read in real time.

## How It Works

1. A background worker runs its own event loop (subscribe to Redis, watch files, poll an API...)
2. It calls `frankenphp_worker_set_vars()` to publish a snapshot of key-value pairs
3. HTTP workers call `frankenphp_worker_get_vars()` to read the latest snapshot
4. The first `get_vars()` call blocks until the background worker has published - no startup race condition

## Configuration

Add `worker` directives with `background` to your [`php_server` or `php` block](config.md#caddyfile-config):

```caddyfile
example.com {
    php_server {
        # Named background workers
        worker /app/bin/console {
            background
            name config-watcher
        }
        worker /app/bin/console {
            background
            name feature-flags
        }

        # Catch-all - handles any unlisted name via get_vars()
        worker /app/bin/console {
            background
        }
    }
}
```

- **Named** (with `name`): lazy-started on first `get_vars()` call, or auto-started at boot if `num 1` is set. Use `num` to run multiple threads for parallel task processing.
- **Catch-all** (no `name`): also lazy-started. Use `max_threads` to cap how many can be created (defaults to 16). Not declaring a catch-all forbids unlisted names.
- Each `php_server` block has its own isolated scope - two blocks can use the same worker names without conflict.
- `max_consecutive_failures`, `env`, and `watch` work the same as HTTP workers.

## PHP API

### `frankenphp_worker_get_vars(string|array $name, float $timeout = 30.0): array`

Starts a background worker (at-most-once) and returns its published variables.

```php
$redis = frankenphp_worker_get_vars('redis-watcher');
// ['MASTER_HOST' => '10.0.0.1', 'MASTER_PORT' => 6379]

$all = frankenphp_worker_get_vars(['redis-watcher', 'feature-flags']);
// ['redis-watcher' => [...], 'feature-flags' => [...]]
```

- First call blocks until the background worker calls `set_vars()` or the timeout expires
- Subsequent calls return the latest snapshot immediately
- Within a single HTTP request, repeated calls with the same name return the same cached array - `===` comparisons are O(1)
- Throws `RuntimeException` on timeout, missing entrypoint, or background worker crash
- Works in both worker and non-worker mode

### `frankenphp_worker_set_vars(array $vars): void`

Publishes a snapshot of key-value pairs from inside a background worker.
Each call **replaces** the entire snapshot atomically.
If the new value is identical (`===`) to the previous one, the call is a no-op.

Supported types: `null`, `bool`, `int`, `float`, `string`, `array` (nested), and **enums**.

- Throws `RuntimeException` if not called from a background worker context
- Throws `ValueError` if values contain objects, resources, or references

### `frankenphp_worker_get_signaling_stream(): resource`

Returns a readable stream for receiving signals from FrankenPHP.
Signals: `"stop\n"` (shutdown/restart), `"task\n"` (task available, see Task API below).
Use `stream_select()` to wait between iterations instead of `sleep()`:

```php
function background_worker_should_stop(float $timeout = 0): bool
{
    static $signalingStream;
    $signalingStream ??= frankenphp_worker_get_signaling_stream();
    $s = (int) $timeout;

    return match (@stream_select(...[[$signalingStream], [], [], $s, (int) (($timeout - $s) * 1e6)])) {
        0 => false, // timeout - keep going
        false => true, // pipe closed - stop
        default => "stop\n" === fgets($signalingStream),
    };
}
```

> [!WARNING]
> Avoid `sleep()` or `usleep()` in background workers - they block at the C level and cannot be interrupted.
> Use `stream_select()` with the signaling stream instead.

### Task API

While `set_vars`/`get_vars` pushes config from background workers to HTTP workers,
tasks enable the reverse: HTTP workers send work to background workers and stream results back.

#### `frankenphp_worker_task_send(string $name, array $payload, float $timeout = 30.0): resource`

Sends a task to a named background worker. Returns a readable stream for receiving results.

- `$payload` follows the same type constraints as `set_vars`: `null`, scalars, arrays, enums. No objects or resources.
- Blocks until a background worker thread picks up the task
- The returned stream wakes up on each `task_update()` - use with `stream_select()` or `fgets()`
- `fclose()` the stream to cancel the task or acknowledge completion
- Throws `RuntimeException` on timeout or if the background worker exits

#### `frankenphp_worker_task_receive(): ?array`

Called from a background worker. Dequeues a pending task. Returns `[$stream, $payload]` or `null`.

- Non-blocking: call it after receiving `"task\n"` on the signaling stream
- `$stream`: writable stream - send results back via `task_update()`
- `$payload`: the array sent by the HTTP worker
- `fclose($stream)` when processing is complete

#### `frankenphp_worker_task_update(resource $stream, array $data): void`

Sends a result or progress update from the background worker to the sender.

#### `frankenphp_worker_task_read(resource $stream): ?array`

Reads the next update on the sender's side. Returns `null` when the background worker closes the stream cleanly.
Throws `RuntimeException` if the background worker exited without completing the task.

#### Example

```php
// Background worker: process tasks
$signaling = frankenphp_worker_get_signaling_stream();

while (true) {
    $r = [$signaling];
    $w = $e = [];
    if (!stream_select($r, $w, $e, 30)) {
        continue;
    }

    if ("stop\n" === $signal = fgets($signaling)) {
        break;
    }

    if ("task\n" === $signal && [$stream, $payload] = frankenphp_worker_task_receive()) {
        frankenphp_worker_task_update($stream, ['result' => process($payload)]);
        fclose($stream);
    }
}
```

```php
// HTTP worker: send a task and read the result
$stream = frankenphp_worker_task_send('image-resizer', ['file' => 'photo.jpg']);
$result = frankenphp_worker_task_read($stream);
fclose($stream);
```

With `num 2` or more, multiple background threads share the same task queue - tasks are distributed automatically.

## Example

### Simple polling worker

```php
<?php
// bin/console (dispatches based on worker name)

$command = $_SERVER['FRANKENPHP_WORKER_NAME'] ?? '';

match ($command) {
    'config-watcher' => run_config_watcher(),
    'feature-flags' => run_feature_flags(),
    default => throw new \RuntimeException("Unknown background worker: $command"),
};

function run_config_watcher(): void
{
    $redis = new Redis();
    $redis->pconnect('127.0.0.1');

    do {
        frankenphp_worker_set_vars([
            'maintenance' => (bool) $redis->get('maintenance_mode'),
            'feature_flags' => json_decode($redis->get('features'), true),
        ]);
    } while (!background_worker_should_stop(5)); // check every 5s
}
```

### Event-driven worker (amphp)

For real-time subscriptions (Redis pub/sub, SSE, WebSocket), use an async library
to integrate the signaling stream into the event loop:

```php
function run_redis_watcher(): void
{
    $signalingStream = frankenphp_worker_get_signaling_stream();
    $sentinel = Amp\Redis\createRedisClient('tcp://sentinel-host:26379');

    $subscription = $sentinel->subscribe('+switch-master');

    Amp\async(function () use ($subscription) {
        foreach ($subscription as $message) {
            [$name, $oldIp, $oldPort, $newIp, $newPort] = explode(' ', $message);
            frankenphp_worker_set_vars([
                'MASTER_HOST' => $newIp,
                'MASTER_PORT' => (int) $newPort,
            ]);
        }
    });

    $master = $sentinel->rawCommand('SENTINEL', 'get-master-addr-by-name', 'mymaster');
    frankenphp_worker_set_vars([
        'MASTER_HOST' => $master[0],
        'MASTER_PORT' => (int) $master[1],
    ]);

    Amp\EventLoop::onReadable($signalingStream, function ($id) use ($signalingStream) {
        if ("stop\n" === fgets($signalingStream)) {
            Amp\EventLoop::cancel($id);
        }
    });
    Amp\EventLoop::run();
}
```

### HTTP Worker

```php
<?php
// public/index.php

$app = new App();
$app->boot();

while (frankenphp_handle_request(function () use ($app) {
    $config = frankenphp_worker_get_vars('config-watcher');

    $_SERVER += ['APP_REDIS_HOST' => $config['MASTER_HOST'], 'APP_REDIS_PORT' => $config['MASTER_PORT']];
    $app->handle($_GET, $_POST, $_COOKIE, $_FILES, $_SERVER);
})) {
    gc_collect_cycles();
}
```

### Graceful Degradation

```php
if (function_exists('frankenphp_worker_get_vars')) {
    $config = frankenphp_worker_get_vars('config-watcher');
} else {
    $config = ['MASTER_HOST' => getenv('REDIS_HOST') ?: '127.0.0.1'];
}
```

## Runtime Behavior

- Background workers get dedicated threads - they don't reduce HTTP capacity
- `max_execution_time` is automatically disabled
- Shebangs (`#!/usr/bin/env php`) are silently skipped
- `$_SERVER['FRANKENPHP_WORKER_NAME']` is set for all workers (HTTP and background)
- `$_SERVER['FRANKENPHP_WORKER_BACKGROUND']` is `true` for background workers, `false` for HTTP workers
- `$_SERVER['argv']` = `[entrypoint, name]` in background workers (for `bin/console` compatibility)
- Crash recovery with automatic restart and exponential backoff. During the restart window, `get_vars` returns the last published data (stale but available). A warning is logged on crash (`background worker exited, restarting`).
- On shutdown/restart: `"stop\n"` is sent on the signaling stream. Workers have 5 seconds to exit. Stuck workers are force-killed on Linux and Windows.
