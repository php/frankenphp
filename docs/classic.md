# Using Classic Mode

Without any additional configuration, FrankenPHP operates in classic mode. In this mode, FrankenPHP functions like a traditional PHP server, directly serving PHP files. This makes it a seamless drop-in replacement for PHP-FPM or Apache with mod_php.

Similar to Caddy, FrankenPHP accepts an unlimited number of connections and uses a [fixed number of threads](config.md#caddyfile-config) to serve them. The number of accepted and queued connections is limited only by the available system resources.
The PHP thread pool operates with a fixed number of threads initialized at startup, comparable to the static mode of PHP-FPM. It's also possible to let threads [scale automatically at runtime](performance.md#max_threads), similar to the dynamic mode of PHP-FPM.

Queued connections will wait indefinitely until a PHP thread is available to serve them. To avoid this, you can use the max_wait_time [configuration](config.md#caddyfile-config) in FrankenPHP's global configuration to limit the duration a request can wait for a free PHP thread before being rejected.
Additionally, you can set a reasonable [write timeout in Caddy](https://caddyserver.com/docs/caddyfile/options#timeouts).

Each Caddy instance will only spin up one FrankenPHP thread pool, which will be shared across all `php_server` blocks.
