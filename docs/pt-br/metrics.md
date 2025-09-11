# Métricas

Quando as [métricas do Caddy](https://caddyserver.com/docs/metrics) estão
habilitadas, o FrankenPHP expõe as seguintes métricas:

- `frankenphp_total_threads`: O número total de threads PHP.
- `frankenphp_busy_threads`: O número de threads PHP processando uma requisição
  no momento (workers em execução sempre consomem uma thread).
- `frankenphp_queue_depth`: O número de requisições regulares na fila.
- `frankenphp_total_workers{worker="[nome_do_worker]"}`: O número total de
  workers.
- `frankenphp_busy_workers{worker="[nome_do_worker]"}`: O número de workers
  processando uma requisição no momento.
- `frankenphp_worker_request_time{worker="[nome_do_worker]"}`: O tempo gasto no
  processamento de requisições por todos os workers.
- `frankenphp_worker_request_count{worker="[nome_do_worker]"}`: O número de
  requisições processadas por todos os workers.
- `frankenphp_ready_workers{worker="[nome_do_worker]"}`: O número de workers que
  chamaram `frankenphp_handle_request` pelo menos uma vez.
- `frankenphp_worker_crashes{worker="[nome_do_worker]"}`: O número de vezes que
  um worker foi encerrado inesperadamente.
- `frankenphp_worker_restarts{worker="[nome_do_worker]"}`: O número de vezes que
  um worker foi reiniciado deliberadamente.
- `frankenphp_worker_queue_depth{worker="[nome_do_worker]"}`: O número de
  requisições na fila.

Para métricas de worker, o placeholder `[nome_do_worker]` é substituído pelo
nome do worker no Caddyfile; caso contrário, o caminho absoluto do arquivo do
worker será usado.
