# Metrics

## Prometheus Export

When [Caddy metrics](https://caddyserver.com/docs/metrics) are enabled, FrankenPHP exposes the following metrics:

- `frankenphp_total_threads`: The total number of PHP threads.
- `frankenphp_busy_threads`: The number of PHP threads currently processing a request (running workers always consume a thread).
- `frankenphp_queue_depth`: The number of regular queued requests
- `frankenphp_total_workers{worker="[worker_name]"}`: The total number of workers.
- `frankenphp_busy_workers{worker="[worker_name]"}`: The number of workers currently processing a request.
- `frankenphp_worker_request_time{worker="[worker_name]"}`: The time spent processing requests by all workers.
- `frankenphp_worker_request_count{worker="[worker_name]"}`: The number of requests processed by all workers.
- `frankenphp_ready_workers{worker="[worker_name]"}`: The number of workers that have called `frankenphp_handle_request` at least once.
- `frankenphp_worker_crashes{worker="[worker_name]"}`: The number of times a worker has unexpectedly terminated.
- `frankenphp_worker_restarts{worker="[worker_name]"}`: The number of times a worker has been deliberately restarted.
- `frankenphp_worker_queue_depth{worker="[worker_name]"}`: The number of queued requests.

For worker metrics, the `[worker_name]` placeholder is replaced by the worker name in the Caddyfile, otherwise absolute path of worker file will be used.

## Threads State Endpoint

FrankenPHP exposes a `/frankenphp/threads` endpoint through the [Caddy admin API](https://caddyserver.com/docs/api).
It returns a JSON snapshot of all active PHP threads, useful for debugging and building observability tools.

```console
curl -s http://localhost:2019/frankenphp/threads | jq .
```

### Response Format

The endpoint returns a JSON object with the following structure:

```json
{
    "ThreadDebugStates": [
        {
            "Index": 0,
            "Name": "worker-/path/to/worker.php",
            "State": "ready",
            "IsWaiting": true,
            "IsBusy": false,
            "WaitingSinceMilliseconds": 1234,
            "CurrentURI": "",
            "CurrentMethod": "",
            "RequestStartedAt": 0,
            "RequestCount": 42,
            "MemoryUsage": 2097152
        }
    ],
    "ReservedThreadCount": 3
}
```

### Fields

| Field | Type | Description |
|---|---|---|
| `ReservedThreadCount` | integer | The number of threads reserved for autoscaling that are not yet active. |

Each entry in `ThreadDebugStates` contains:

| Field | Type | Description |
|---|---|---|
| `Index` | integer | The index of the thread. |
| `Name` | string | The name of the thread (e.g., the worker file path). |
| `State` | string | The internal state of the thread (e.g., `ready`, `shutting down`). |
| `IsWaiting` | boolean | Whether the thread is waiting for a request. |
| `IsBusy` | boolean | Whether the thread is currently processing a request. |
| `WaitingSinceMilliseconds` | integer | How long the thread has been idle, in milliseconds. `0` if the thread is busy. |
| `CurrentURI` | string | The URI currently being processed. Empty if the thread is idle. |
| `CurrentMethod` | string | The HTTP method of the current request (e.g., `GET`, `POST`). Empty if the thread is idle. |
| `RequestStartedAt` | integer | Unix timestamp in milliseconds of when the current request started. `0` if the thread is idle. |
| `RequestCount` | integer | The total number of requests this thread has processed since it started. |
| `MemoryUsage` | integer | The current PHP memory usage of the thread, in bytes. |
