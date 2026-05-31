---
title: FrankenPHP observability with metrics, logs, and Ember TUI
description: Monitor FrankenPHP in development and production using Prometheus metrics, structured logs, the Ember TUI dashboard, and custom Grafana scraping setups.
---

# Observability

FrankenPHP provides built-in observability features: [Prometheus-compatible metrics](metrics.md) and [structured logging](logging.md).
These features, combined with the recommended tools below, give you full visibility into your PHP application's behavior in development and production.

## Ember TUI and Prometheus exporter

[Ember](https://github.com/alexandre-daubois/ember) is the most user-friendly way to monitor FrankenPHP.

It connects to Caddy's admin API and deeply integrates with FrankenPHP, providing real-time visibility with zero configuration and no external infrastructure.

It is designed to be used in development and production, with a TUI dashboard for local use and a Prometheus export daemon mode for production monitoring.

> [!TIP]
> See the [Ember documentation](https://github.com/alexandre-daubois/ember) for the full list of features and setup details.

## Metrics

FrankenPHP exposes Prometheus-compatible metrics for threads, workers, request processing, and queue depth when [Caddy metrics](https://caddyserver.com/docs/metrics) are enabled.

See the [Metrics](metrics.md) page for the full list of available metrics.

## Logging

FrankenPHP integrates with Caddy's logging system and provides `frankenphp_log()` for structured logging with severity levels and context data, making ingestion into platforms like Datadog, Grafana Loki, or Elastic straightforward.

See the [Logging](logging.md) page for usage details.

## Custom Prometheus/Grafana setup

If you prefer a custom monitoring stack, you can scrape FrankenPHP metrics directly.
There are two options:

1. **Scrape Caddy directly**: Caddy exposes metrics at its admin endpoint (default: `localhost:2019/metrics`)
2. **Scrape via Ember**: when running Ember with `--expose`, it exposes FrankenPHP metrics along with computed metrics derived from Caddy data (RPS, latency percentiles, error rates) on a dedicated endpoint.
