# Logging

FrankenPHP integrates seamlessly with [Caddy's logging system](https://caddyserver.com/docs/logging).
You can log messages using standard PHP functions or leverage the dedicated `frankenphp_log()` function for advanced
structured logging capabilities.

## `frankenphp_log()`

The `frankenphp_log()` function allows you to emit structured logs directly from your PHP application,
making ingestion into platforms like Datadog, Grafana Loki, or Elastic, as well as OpenTelemetry support, much easier.

Under the hood, `frankenphp_log()` wraps [Go's `log/slog` package](https://pkg.go.dev/log/slog) to provide rich logging
features.

These logs include the severity level and optional context data.

```php
function frankenphp_log(string $message, int $level = FRANKENPHP_LOG_LEVEL_INFO, array $context = []): void
```

### Parameters

- **`message`**: The log message string.
- **`level`**: The severity level of the log. Can be any arbitrary integer. Convenience constants are provided for common levels: `FRANKENPHP_LOG_LEVEL_DEBUG` (`-4`), `FRANKENPHP_LOG_LEVEL_INFO` (`0`), `FRANKENPHP_LOG_LEVEL_WARN` (`4`) and `FRANKENPHP_LOG_LEVEL_ERROR` (`8`)). Default is `FRANKENPHP_LOG_LEVEL_INFO`.
- **`context`**: An associative array of additional data to include in the log entry.

### Example

```php
<?php

// Log a simple informational message
frankenphp_log("Hello from FrankenPHP!");

// Log a warning with context data
frankenphp_log(
    "Memory usage high",
    FRANKENPHP_LOG_LEVEL_WARN,
    [
        'current_usage' => memory_get_usage(),
        'peak_usage' => memory_get_peak_usage(),
    ],
);

```

When viewing the logs (e.g., via `docker compose logs`), the output will appear as structured JSON:

```json
{"level":"info","ts":1704067200,"logger":"frankenphp","msg":"Hello from FrankenPHP!"}
{"level":"warn","ts":1704067200,"logger":"frankenphp","msg":"Memory usage high","current_usage":10485760,"peak_usage":12582912}
```

## `error_log()`

FrankenPHP also allows logging using the standard `error_log()` function. If the `$message_type` parameter is `4` (SAPI),
these messages are routed to the Caddy logger.

By default, messages sent via `error_log()` are treated as unstructured text.
They are useful for compatibility with existing applications or libraries that rely on the standard PHP library.

### Usage

```php
error_log("Database connection failed", 4);
```

This will appear in the Caddy logs, often prefixed to indicate it originated from PHP.

> [!TIP]
> For better observability in production environments, prefer `frankenphp_log()`
> as it allows you to filter logs by level (Debug, Error, etc.)
> and query specific fields in your logging infrastructure.
