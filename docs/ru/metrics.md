# Метрики

При включении [метрик Caddy](https://caddyserver.com/docs/metrics) FrankenPHP предоставляет следующие метрики:

- `frankenphp_total_threads`: Общее количество потоков PHP.
- `frankenphp_busy_threads`: Количество потоков PHP, которые в данный момент обрабатывают запрос (работающие воркеры всегда используют поток).
- `frankenphp_queue_depth`: Количество обычных запросов в очереди.
- `frankenphp_total_workers{worker="[worker_name]"}`: Общее количество воркеров.
- `frankenphp_busy_workers{worker="[worker_name]"}`: Количество воркеров, которые в данный момент обрабатывают запрос.
- `frankenphp_worker_request_time{worker="[worker_name]"}`: Время, затраченное всеми воркерами на обработку запросов.
- `frankenphp_worker_request_count{worker="[worker_name]"}`: Количество запросов, обработанных всеми воркерами.
- `frankenphp_ready_workers{worker="[worker_name]"}`: Количество воркеров, которые вызвали `frankenphp_handle_request` хотя бы один раз.
- `frankenphp_worker_crashes{worker="[worker_name]"}`: Количество случаев неожиданного завершения работы воркера.
- `frankenphp_worker_restarts{worker="[worker_name]"}`: Количество случаев, когда воркер был целенаправленно перезапущен.
- `frankenphp_worker_queue_depth{worker="[worker_name]"}`: Количество запросов в очереди.

Для метрик воркеров плейсхолдер `[worker_name]` заменяется на имя воркера в Caddyfile, иначе будет использоваться абсолютный путь к файлу воркера.
