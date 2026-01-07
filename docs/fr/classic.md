# Utilisation du mode classique

Sans aucune configuration additionnelle, FrankenPHP fonctionne en mode classique. Dans ce mode, FrankenPHP fonctionne comme un serveur PHP traditionnel, en servant directement les fichiers PHP. Cela en fait un remplaçant transparent pour PHP-FPM ou Apache avec mod_php.

Comme Caddy, FrankenPHP accepte un nombre illimité de connexions et utilise un [nombre fixe de threads](config.md#caddyfile-config) pour les servir. Le nombre de connexions acceptées et mises en file d'attente n'est limité que par les ressources système disponibles.
Le pool de threads PHP fonctionne avec un nombre fixe de threads initialisés au démarrage, comparable au mode statique de PHP-FPM. Il est également possible de laisser les threads [s'adapter automatiquement à l'exécution](performance.md#max_threads), à l'instar du mode dynamique de PHP-FPM.

Les connexions mises en file d'attente attendront indéfiniment jusqu'à ce qu'un thread PHP soit disponible pour les servir. Pour éviter cela, vous pouvez utiliser la [configuration](config.md#caddyfile-config) `max_wait_time` dans la configuration globale de FrankenPHP afin de limiter la durée pendant laquelle une requête peut attendre un thread PHP libre avant d'être rejetée.
De plus, vous pouvez définir un [délai d'écriture raisonnable dans Caddy](https://caddyserver.com/docs/caddyfile/options#timeouts).

Chaque instance de Caddy ne lancera qu'un seul pool de threads FrankenPHP, qui sera partagé entre tous les blocs `php_server`.
