# Usando el Modo Clásico

Sin ninguna configuración adicional, FrankenPHP opera en modo clásico. En este modo, FrankenPHP funciona como un servidor PHP tradicional, sirviendo directamente archivos PHP. Esto lo convierte en un reemplazo directo para PHP-FPM o Apache con mod_php.

Al igual que Caddy, FrankenPHP acepta un número ilimitado de conexiones y utiliza un [número fijo de hilos](config.md#caddyfile-config) para atenderlas. La cantidad de conexiones aceptadas y en cola está limitada únicamente por los recursos disponibles del sistema.
El *pool* de hilos de PHP opera con un número fijo de hilos inicializados al inicio, comparable al modo estático de PHP-FPM. También es posible permitir que los hilos [escale automáticamente en tiempo de ejecución](performance.md#max_threads), similar al modo dinámico de PHP-FPM.

Las conexiones en cola esperarán indefinidamente hasta que un hilo de PHP esté disponible para atenderlas. Para evitar esto, puedes usar la configuración `max_wait_time` en la [configuración global de FrankenPHP](config.md#caddyfile-config) para limitar la duración que una petición puede esperar por un hilo de PHP libre antes de ser rechazada.
Adicionalmente, puedes establecer un [tiempo límite de escritura razonable en Caddy](https://caddyserver.com/docs/caddyfile/options#timeouts).

Cada instancia de Caddy iniciará solo un *pool* de hilos de FrankenPHP, el cual será compartido entre todos los bloques `php_server`.
