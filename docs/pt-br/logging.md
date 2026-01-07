# Registro (Logging)

FrankenPHP se integra perfeitamente com o [sistema de registro do Caddy](https://caddyserver.com/docs/logging).
Você pode registrar mensagens usando funções PHP padrão ou aproveitar a função dedicada `frankenphp_log()` para recursos avançados
de registro estruturado.

## `frankenphp_log()`

A função `frankenphp_log()` permite que você emita logs estruturados diretamente de sua aplicação PHP,
facilitando muito a ingestão em plataformas como Datadog, Grafana Loki ou Elastic, bem como o suporte a OpenTelemetry.

Internamente, `frankenphp_log()` envolve o [pacote `log/slog` do Go](https://pkg.go.dev/log/slog) para fornecer recursos de registro
ricos.

Esses logs incluem o nível de severidade e dados de contexto opcionais.

```php
function frankenphp_log(string $message, int $level = FRANKENPHP_LOG_LEVEL_INFO, array $context = []): void
```

### Parâmetros

-   **`message`**: A string da mensagem de log.
-   **`level`**: O nível de severidade do log. Pode ser qualquer número inteiro arbitrário. Constantes de conveniência são fornecidas para níveis comuns: `FRANKENPHP_LOG_LEVEL_DEBUG` (`-4`), `FRANKENPHP_LOG_LEVEL_INFO` (`0`), `FRANKENPHP_LOG_LEVEL_WARN` (`4`) e `FRANKENPHP_LOG_LEVEL_ERROR` (`8`)). O padrão é `FRANKENPHP_LOG_LEVEL_INFO`.
-   **`context`**: Um array associativo de dados adicionais a serem incluídos na entrada de log.

### Exemplo

```php
<?php

// Registra uma mensagem informativa simples
frankenphp_log("Olá do FrankenPHP!");

// Registra um aviso com dados de contexto
frankenphp_log(
    "Uso de memória alto",
    FRANKENPHP_LOG_LEVEL_WARN,
    [
        'current_usage' => memory_get_usage(),
        'peak_usage' => memory_get_peak_usage(),
    ],
);

```

Ao visualizar os logs (por exemplo, via `docker compose logs`), a saída aparecerá como JSON estruturado:

```json
{"level":"info","ts":1704067200,"logger":"frankenphp","msg":"Hello from FrankenPHP!"}
{"level":"warn","ts":1704067200,"logger":"frankenphp","msg":"Memory usage high","current_usage":10485760,"peak_usage":12582912}
```

## `error_log()`

FrankenPHP também permite o registro usando a função padrão `error_log()`. Se o parâmetro `$message_type` for `4` (SAPI),
essas mensagens são roteadas para o logger do Caddy.

Por padrão, as mensagens enviadas via `error_log()` são tratadas como texto não estruturado.
Elas são úteis para compatibilidade com aplicações ou bibliotecas existentes que dependem da biblioteca PHP padrão.

### Exemplo com error_log()

```php
error_log("Falha na conexão com o banco de dados", 4);
```

Isso aparecerá nos logs do Caddy, muitas vezes prefixado para indicar que se originou do PHP.

> [!TIP]
> Para melhor observabilidade em ambientes de produção, prefira `frankenphp_log()`
> pois ele permite filtrar logs por nível (Debug, Error, etc.)
> e consultar campos específicos em sua infraestrutura de registro.
