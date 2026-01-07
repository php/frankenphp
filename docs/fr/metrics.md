# Métriques

Lorsque les [métriques Caddy](https://caddyserver.com/docs/metrics) sont activées, FrankenPHP expose les métriques suivantes :

- `frankenphp_total_threads` : Le nombre total de threads PHP.
- `frankenphp_busy_threads` : Le nombre de threads PHP en cours de traitement d'une requête (les workers en cours d'exécution consomment toujours un thread).
- `frankenphp_queue_depth` : Le nombre de requêtes régulières en file d'attente
- `frankenphp_total_workers{worker="[nom_du_worker]"}` : Le nombre total de workers.
- `frankenphp_busy_workers{worker="[nom_du_worker]"}` : Le nombre de workers qui traitent actuellement une requête.
- `frankenphp_worker_request_time{worker="[nom_du_worker]"}` : Le temps passé à traiter les requêtes par tous les workers.
- `frankenphp_worker_request_count{worker="[nom_du_worker]"}` : Le nombre de requêtes traitées par tous les workers.
- `frankenphp_ready_workers{worker="[nom_du_worker]"}` : Le nombre de workers qui ont appelé `frankenphp_handle_request` au moins une fois.
- `frankenphp_worker_crashes{worker="[nom_du_worker]"}` : Le nombre de fois où un worker s'est arrêté de manière inattendue.
- `frankenphp_worker_restarts{worker="[nom_du_worker]"}` : Le nombre de fois où un worker a été délibérément redémarré.
- `frankenphp_worker_queue_depth{worker="[nom_du_worker]"}` : Le nombre de requêtes en file d'attente.

Pour les métriques de worker, l'espace réservé `[nom_du_worker]` est remplacé par le nom du worker dans le Caddyfile, sinon le chemin absolu du fichier du worker sera utilisé.
