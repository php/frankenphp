# 日志

FrankenPHP 与 [Caddy 的日志系统](https://caddyserver.com/docs/logging)无缝集成。
您可以使用标准的 PHP 函数记录消息，也可以利用专用的 `frankenphp_log()` 函数进行高级结构化日志记录。

## `frankenphp_log()`

`frankenphp_log()` 函数允许您直接从 PHP 应用程序发出结构化日志，从而更容易地将其摄取到 Datadog、Grafana Loki 或 Elastic 等平台中，并支持 OpenTelemetry。

在底层，`frankenphp_log()` 封装了 [Go 的 `log/slog` 包](https://pkg.go.dev/log/slog)以提供丰富的日志功能。

这些日志包括严重性级别和可选的上下文数据。

```php
function frankenphp_log(string $message, int $level = FRANKENPHP_LOG_LEVEL_INFO, array $context = []): void
```

### 参数

- **`message`**: 日志消息字符串。
- **`level`**: 日志的严重性级别。可以是任意整数。提供了用于常见级别的便捷常量：`FRANKENPHP_LOG_LEVEL_DEBUG` (`-4`)、`FRANKENPHP_LOG_LEVEL_INFO` (`0`)、`FRANKENPHP_LOG_LEVEL_WARN` (`4`) 和 `FRANKENPHP_LOG_LEVEL_ERROR` (`8`)。默认值为 `FRANKENPHP_LOG_LEVEL_INFO`。
- **`context`**: 一个关联数组，包含要包含在日志条目中的附加数据。

### 示例

```php
<?php

// 记录一条简单的信息消息
frankenphp_log("Hello from FrankenPHP!");

// 记录一条带有上下文数据的警告
frankenphp_log(
    "Memory usage high",
    FRANKENPHP_LOG_LEVEL_WARN,
    [
        'current_usage' => memory_get_usage(),
        'peak_usage' => memory_get_peak_usage(),
    ],
);

```

当查看日志时（例如，通过 `docker compose logs`），输出将显示为结构化 JSON：

```json
{"level":"info","ts":1704067200,"logger":"frankenphp","msg":"Hello from FrankenPHP!"}
{"level":"warn","ts":1704067200,"logger":"frankenphp","msg":"Memory usage high","current_usage":10485760,"peak_usage":12582912}
```

## `error_log()`

FrankenPHP 也允许使用标准的 `error_log()` 函数进行日志记录。如果 `$message_type` 参数为 `4` (SAPI)，这些消息将被路由到 Caddy 日志记录器。

默认情况下，通过 `error_log()` 发送的消息被视为非结构化文本。它们对于与依赖标准 PHP 库的现有应用程序或库的兼容性非常有用。

### `error_log()` 示例

```php
error_log("Database connection failed", 4);
```

这将在 Caddy 日志中显示，通常带有前缀以表明它源自 PHP。

> [!TIP]
> 为了在生产环境中获得更好的可观测性，建议优先使用 `frankenphp_log()`，因为它允许您按级别（调试、错误等）过滤日志并在您的日志基础设施中查询特定字段。
