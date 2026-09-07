---
title: 通过 frankenphp_log() 和 Caddy 实现 FrankenPHP 日志记录
description: 在 FrankenPHP 中使用 frankenphp_log() 或 error_log() 从 PHP 输出结构化日志，通过 Caddy 的日志系统以 JSON 格式路由到 Datadog、Loki 或 Elastic。
---

# 日志记录

> [!TIP]
> 日志记录是 FrankenPHP 可观测性体系的一部分。请参阅[可观测性](observability.md)页面了解完整信息，包括实时监控和指标。

FrankenPHP 与 [Caddy 的日志系统](https://caddyserver.com/docs/logging)无缝集成。
你可以使用标准 PHP 函数记录日志，也可以利用专用的 `frankenphp_log()` 函数实现高级
结构化日志功能。

## `frankenphp_log()`

`frankenphp_log()` 函数允许你直接从 PHP 应用输出结构化日志，
使日志接入 Datadog、Grafana Loki、Elastic 等平台以及 OpenTelemetry 支持变得更加容易。

在底层，`frankenphp_log()` 封装了 [Go 的 `log/slog` 包](https://pkg.go.dev/log/slog)，提供丰富的日志功能。

这些日志包含严重性级别和可选的上下文数据。

```php
function frankenphp_log(string $message, int $level = FRANKENPHP_LOG_LEVEL_INFO, array $context = []): void
```

### 参数

- **`message`**：日志消息字符串。
- **`level`**：日志的严重性级别。可以是任意整数。为常用级别提供了便捷常量：`FRANKENPHP_LOG_LEVEL_DEBUG`（`-4`）、`FRANKENPHP_LOG_LEVEL_INFO`（`0`）、`FRANKENPHP_LOG_LEVEL_WARN`（`4`）和 `FRANKENPHP_LOG_LEVEL_ERROR`（`8`）。默认为 `FRANKENPHP_LOG_LEVEL_INFO`。
- **`context`**：包含在日志条目中的附加数据的关联数组。

### 示例

```php
<?php

// 记录一条简单的信息消息
frankenphp_log("Hello from FrankenPHP!");

// 带上下文数据记录一条警告
frankenphp_log(
    "Memory usage high",
    FRANKENPHP_LOG_LEVEL_WARN,
    [
        'current_usage' => memory_get_usage(),
        'peak_usage' => memory_get_peak_usage(),
    ],
);

```

查看日志时（例如通过 `docker compose logs`），输出将显示为结构化 JSON：

```json
{"level":"info","ts":1704067200,"logger":"frankenphp","msg":"Hello from FrankenPHP!"}
{"level":"warn","ts":1704067200,"logger":"frankenphp","msg":"Memory usage high","current_usage":10485760,"peak_usage":12582912}
```

## `error_log()`

FrankenPHP 还允许使用标准的 `error_log()` 函数记录日志。如果 `$message_type` 参数为 `4`（SAPI），
这些消息将被路由到 Caddy 日志记录器。

默认情况下，通过 `error_log()` 发送的消息被视为非结构化文本。
它们适用于与依赖标准 PHP 库的现有应用或库的兼容性。

### error_log() 示例

```php
error_log("Database connection failed", 4);
```

这将出现在 Caddy 日志中，通常带有前缀以表明它来自 PHP。

> [!TIP]
> 在生产环境中为了更好的可观测性，建议优先使用 `frankenphp_log()`，
> 因为它允许你按级别（调试、错误等）过滤日志，
> 并在日志基础设施中查询特定字段。
