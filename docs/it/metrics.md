# Metriche

> [!TIP]
> Per una configurazione completa dell'osservabilità, compresi dashboard in tempo reale e monitoraggio della produzione, consultare la pagina [Osservabilità](../docs/observability.md).

Quando le [metriche Caddy](https://caddyserver.com/docs/metrics) sono abilitate, FrankenPHP espone le seguenti metriche:

- `frankenphp_total_threads`: il numero totale di thread PHP.
- `frankenphp_busy_threads`: il numero di thread PHP che attualmente elaborano una richiesta (i worker in esecuzione consumano sempre un thread).
- `frankenphp_queue_depth`: il numero di richieste regolari in coda
- `frankenphp_total_workers{worker="[worker_name]"}`: numero totale di worker.
- `frankenphp_busy_workers{worker="[worker_name]"}`: il numero di worker che attualmente elaborano una richiesta.
- `frankenphp_worker_request_time{worker="[worker_name]"}`: tempo impiegato per elaborare le richieste di tutti i worker.
- `frankenphp_worker_request_count{worker="[worker_name]"}`: numero di richieste elaborate da tutti i worker.
- `frankenphp_ready_workers{worker="[worker_name]"}`: il numero di worker che hanno chiamato `frankenphp_handle_request` almeno una volta.
- `frankenphp_worker_crashes{worker="[worker_name]"}`: il numero di volte in cui un worker è stato licenziato in modo imprevisto.
- `frankenphp_worker_restarts{worker="[worker_name]"}`: numero di volte in cui un worker è stato riavviato deliberatamente.
- `frankenphp_worker_queue_depth{worker="[worker_name]"}`: il numero di richieste in coda.

Per le metriche dei worker, il segnaposto `[worker_name]` viene sostituito dal nome del worker nel Caddyfile, altrimenti verrà utilizzato il percorso assoluto del file del worker.
