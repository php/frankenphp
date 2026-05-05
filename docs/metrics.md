---
title: FrankenPHP Prometheus metrics for threads and workers
description: List of Prometheus-compatible metrics exposed by FrankenPHP via Caddy, covering PHP threads, workers, request times, queue depth, crashes, and restarts.
---

# Metrics

> [!TIP]
> For a complete observability setup including real-time dashboards and production monitoring, see the [Observability](observability.md) page.

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

For worker metrics, the `[worker_name]` placeholder is replaced by the worker name in the Caddyfile, otherwise the absolute path of the worker file will be used.
