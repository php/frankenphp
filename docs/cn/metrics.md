# 指标

当启用 [Caddy 指标](https://caddyserver.com/docs/metrics) 时，FrankenPHP 公开以下指标：

- `frankenphp_total_threads`：PHP 线程的总数。
- `frankenphp_busy_threads`：当前正在处理请求的 PHP 线程数（运行中的 worker 始终占用一个线程）。
- `frankenphp_queue_depth`：常规排队请求的数量
- `frankenphp_total_workers{worker="[worker_name]"}`：worker 的总数。
- `frankenphp_busy_workers{worker="[worker_name]"}`：当前正在处理请求的 worker 数量。
- `frankenphp_worker_request_time{worker="[worker_name]"}`：所有 worker 处理请求所花费的时间。
- `frankenphp_worker_request_count{worker="[worker_name]"}`：所有 worker 处理的请求数量。
- `frankenphp_ready_workers{worker="[worker_name]"}`：至少调用过一次 `frankenphp_handle_request` 的 worker 数量。
- `frankenphp_worker_crashes{worker="[worker_name]"}`：worker 意外终止的次数。
- `frankenphp_worker_restarts{worker="[worker_name]"}`：worker 被故意重启的次数。
- `frankenphp_worker_queue_depth{worker="[worker_name]"}`：排队请求的数量。

对于 worker 指标，`[worker_name]` 占位符被 Caddyfile 中的 worker 名称替换，否则将使用 worker 文件的绝对路径。
