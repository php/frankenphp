# 使用经典模式

在没有任何额外配置的情况下，FrankenPHP 以经典模式运行。在此模式下，FrankenPHP 的功能类似于传统的 PHP 服务器，直接提供 PHP 文件服务。这使其成为 PHP-FPM 或 Apache with mod_php 的无缝替代品。

与 Caddy 类似，FrankenPHP 接受无限数量的连接，并使用[固定数量的线程](config.md#caddyfile-config)来为它们提供服务。接受和排队的连接数量仅受可用系统资源的限制。
PHP 线程池以启动时初始化的固定数量的线程运行，这类似于 PHP-FPM 的静态模式。线程也可以在[运行时自动扩展](performance.md#max_threads)，类似于 PHP-FPM 的动态模式。

排队的连接将无限期等待，直到有 PHP 线程可以为它们提供服务。为了避免这种情况，你可以在 FrankenPHP 的全局配置中使用 `max_wait_time` [配置](config.md#caddyfile-config)来限制请求在被拒绝之前等待空闲 PHP 线程的时间。
此外，你还可以在 Caddy 中设置合理的[写超时](https://caddyserver.com/docs/caddyfile/options#timeouts)。

每个 Caddy 实例只会启动一个 FrankenPHP 线程池，该线程池将在所有 `php_server` 块之间共享。
