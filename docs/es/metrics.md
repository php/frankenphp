# Métricas

Cuando las [métricas de Caddy](https://caddyserver.com/docs/metrics) están habilitadas, FrankenPHP expone las siguientes métricas:

- `frankenphp_total_threads`: El número total de hilos PHP.
- `frankenphp_busy_threads`: El número de hilos PHP procesando actualmente una solicitud (los workers en ejecución siempre consumen un hilo).
- `frankenphp_queue_depth`: El número de solicitudes regulares en cola
- `frankenphp_total_workers{worker="[nombre_worker]"}`: El número total de workers.
- `frankenphp_busy_workers{worker="[nombre_worker]"}`: El número de workers procesando actualmente una solicitud.
- `frankenphp_worker_request_time{worker="[nombre_worker]"}`: El tiempo dedicado al procesamiento de solicitudes por todos los workers.
- `frankenphp_worker_request_count{worker="[nombre_worker]"}`: El número de solicitudes procesadas por todos los workers.
- `frankenphp_ready_workers{worker="[nombre_worker]"}`: El número de workers que han llamado a `frankenphp_handle_request` al menos una vez.
- `frankenphp_worker_crashes{worker="[nombre_worker]"}`: El número de veces que un worker ha terminado inesperadamente.
- `frankenphp_worker_restarts{worker="[nombre_worker]"}`: El número de veces que un worker ha sido reiniciado deliberadamente.
- `frankenphp_worker_queue_depth{worker="[nombre_worker]"}`: El número de solicitudes en cola.

Para las métricas de los workers, el marcador de posición `[nombre_worker]` es reemplazado por el nombre del worker en el Caddyfile; de lo contrario, se usará la ruta absoluta del archivo del worker.
