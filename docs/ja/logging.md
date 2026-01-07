# ロギング

FrankenPHP は、[Caddy のロギングシステム](https://caddyserver.com/docs/logging)とシームレスに統合されています。
標準の PHP 関数を使用してメッセージをログに記録するか、高度な構造化ロギング機能のために専用の `frankenphp_log()` 関数を利用できます。

## `frankenphp_log()`

`frankenphp_log()` 関数を使用すると、PHP アプリケーションから直接構造化ログを出力でき、
Datadog、Grafana Loki、Elastic などのプラットフォームへの取り込みや OpenTelemetry のサポートがはるかに容易になります。

内部的に、`frankenphp_log()` は [Go の `log/slog` パッケージ](https://pkg.go.dev/log/slog)をラップして、豊富なロギング機能を提供します。

これらのログには、深刻度レベルとオプションのコンテキストデータが含まれます。

```php
function frankenphp_log(string $message, int $level = FRANKENPHP_LOG_LEVEL_INFO, array $context = []): void
```

### パラメータ

-   **`message`**: ログメッセージ文字列。
-   **`level`**: ログの深刻度レベル。任意の整数値を指定できます。一般的なレベルには便利な定数が用意されています: `FRANKENPHP_LOG_LEVEL_DEBUG` (`-4`)、`FRANKENPHP_LOG_LEVEL_INFO` (`0`)、`FRANKENPHP_LOG_LEVEL_WARN` (`4`)、`FRANKENPHP_LOG_LEVEL_ERROR` (`8`)。デフォルトは `FRANKENPHP_LOG_LEVEL_INFO` です。
-   **`context`**: ログエントリに含める追加データの連想配列。

### 例

```php
<?php

// 単純な情報メッセージをログに記録
frankenphp_log("Hello from FrankenPHP!");

// コンテキストデータを含む警告をログに記録
frankenphp_log(
    "Memory usage high",
    FRANKENPHP_LOG_LEVEL_WARN,
    [
        'current_usage' => memory_get_usage(),
        'peak_usage' => memory_get_peak_usage(),
    ],
);

```

ログを表示すると (例: `docker compose logs` 経由)、出力は構造化された JSON として表示されます。

```json
{"level":"info","ts":1704067200,"logger":"frankenphp","msg":"Hello from FrankenPHP!"}
{"level":"warn","ts":1704067200,"logger":"frankenphp","msg":"Memory usage high","current_usage":10485760,"peak_usage":12582912}
```

## `error_log()`

FrankenPHP は、標準の `error_log()` 関数を使用したロギングも可能です。`$message_type` パラメータが `4` (SAPI) の場合、
これらのメッセージは Caddy ロガーにルーティングされます。

デフォルトでは、`error_log()` を介して送信されるメッセージは非構造化テキストとして扱われます。
これらは、標準の PHP ライブラリに依存する既存のアプリケーションやライブラリとの互換性のために有用です。

### `error_log()` の例

```php
error_log("Database connection failed", 4);
```

これは Caddy ログに表示され、多くの場合、PHP から発信されたことを示すプレフィックスが付けられます。

> [!TIP]
> 本番環境での可観測性を高めるには、`frankenphp_log()` を使用することをお勧めします。
> これは、レベル (デバッグ、エラーなど) でログをフィルタリングしたり、
> ロギングインフラストラクチャ内の特定のフィールドをクエリしたりできるためです。
