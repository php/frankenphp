# Метрики

При включении [метрик Caddy](https://caddyserver.com/docs/metrics) FrankenPHP предоставляет следующие метрики:

- `frankenphp_[worker]_total_workers`: Общее количество worker-скриптов.
- `frankenphp_[worker]_busy_workers`: Количество worker-скриптов, которые в данный момент обрабатывают запрос.
- `frankenphp_[worker]_worker_request_time`: Время, затраченное всеми worker-скриптами на обработку запросов.
- `frankenphp_[worker]_worker_request_count`: Количество запросов, обработанных всеми worker-скриптами.
- `frankenphp_[worker]_ready_workers`: Количество worker-скриптов, которые вызвали `frankenphp_handle_request` хотя бы один раз.
- `frankenphp_[worker]_worker_crashes`: Количество случаев неожиданного завершения worker-скриптов.
- `frankenphp_[worker]_worker_restarts`: Количество случаев, когда worker-скрипт был перезапущен целенаправленно.
- `frankenphp_total_threads`: Общее количество потоков PHP.
- `frankenphp_busy_threads`: Количество потоков PHP, которые в данный момент обрабатывают запрос (работающие worker-скрипты всегда используют поток).

Для метрик worker-скриптов плейсхолдер `[worker]` заменяется на путь к Worker-скрипту, указанному в Caddyfile.
