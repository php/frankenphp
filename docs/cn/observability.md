---
title: 使用指标、日志和 Ember TUI 实现 FrankenPHP 可观测性
description: 通过 Prometheus 指标、结构化日志、Ember TUI 仪表盘以及自定义 Grafana 抓取配置，在开发和生产环境中监控 FrankenPHP。
---

# 可观测性

FrankenPHP 提供内置的可观测性功能：[兼容 Prometheus 的指标](metrics.md)和[结构化日志](logging.md)。
这些功能与以下推荐工具结合使用，可让你在开发和生产环境中全面了解 PHP 应用的运行状态。

## Ember TUI 和 Prometheus 导出器

[Ember](https://github.com/alexandre-daubois/ember) 是监控 FrankenPHP 最友好的方式。

它连接到 Caddy 的管理 API，并与 FrankenPHP 深度集成，提供零配置、无需外部基础设施的实时可见性。

它专为开发和生产环境设计，提供用于本地使用的 TUI 仪表盘，以及用于生产监控的 Prometheus 导出守护进程模式。

> [!TIP]
> 请参阅 [Ember 文档](https://github.com/alexandre-daubois/ember)了解完整功能列表和设置详情。

## 指标

当启用 [Caddy 指标](https://caddyserver.com/docs/metrics)时，FrankenPHP 会暴露兼容 Prometheus 的指标，涵盖线程、worker、请求处理和队列深度。

请参阅[指标](metrics.md)页面获取可用指标的完整列表。

## 日志

FrankenPHP 集成了 Caddy 的日志系统，并提供 `frankenphp_log()` 函数用于结构化日志记录，支持严重级别和上下文数据，便于接入 Datadog、Grafana Loki 或 Elastic 等平台。

请参阅[日志](logging.md)页面获取使用详情。

## 自定义 Prometheus/Grafana 配置

如果你更倾向于自定义监控方案，可以直接抓取 FrankenPHP 的指标。
有两种方式：

1. **直接抓取 Caddy**：Caddy 在其管理端点暴露指标（默认：`localhost:2019/metrics`）
2. **通过 Ember 抓取**：当 Ember 以 `--expose` 模式运行时，它会在专用端点上暴露 FrankenPHP 指标以及从 Caddy 数据计算得出的指标（RPS、延迟百分位数、错误率）。
