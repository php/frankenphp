# Registro de actividad

FrankenPHP se integra perfectamente con [el sistema de registro de Caddy](https://caddyserver.com/docs/logging).
Puede registrar mensajes usando funciones estándar de PHP o aprovechar la función dedicada `frankenphp_log()` para capacidades avanzadas de registro estructurado.

## `frankenphp_log()`

La función `frankenphp_log()` le permite emitir registros estructurados directamente desde su aplicación PHP,
facilitando la ingesta en plataformas como Datadog, Grafana Loki o Elastic, así como el soporte para OpenTelemetry.

Internamente, `frankenphp_log()` envuelve [el paquete `log/slog` de Go](https://pkg.go.dev/log/slog) para proporcionar funciones avanzadas de registro.

Estos registros incluyen el nivel de gravedad y datos de contexto opcionales.

```php
function frankenphp_log(string $message, int $level = FRANKENPHP_LOG_LEVEL_INFO, array $context = []): void
```

### Parámetros

- **`message`**: El string del mensaje de registro.
- **`level`**: El nivel de gravedad del registro. Puede ser cualquier entero arbitrario. Se proporcionan constantes de conveniencia para niveles comunes: `FRANKENPHP_LOG_LEVEL_DEBUG` (`-4`), `FRANKENPHP_LOG_LEVEL_INFO` (`0`), `FRANKENPHP_LOG_LEVEL_WARN` (`4`) y `FRANKENPHP_LOG_LEVEL_ERROR` (`8`)). Por omisión es `FRANKENPHP_LOG_LEVEL_INFO`.
- **`context`**: Un array asociativo de datos adicionales para incluir en la entrada del registro.

### Ejemplo

```php
<?php

// Registrar un mensaje informativo simple
frankenphp_log("¡Hola desde FrankenPHP!");

// Registrar una advertencia con datos de contexto
frankenphp_log(
    "Uso de memoria alto",
    FRANKENPHP_LOG_LEVEL_WARN,
    [
        'uso_actual' => memory_get_usage(),
        'uso_pico' => memory_get_peak_usage(),
    ],
);

```

Al ver los registros (por ejemplo, mediante `docker compose logs`), la salida aparecerá como JSON estructurado:

```json
{"level":"info","ts":1704067200,"logger":"frankenphp","msg":"¡Hola desde FrankenPHP!"}
{"level":"warn","ts":1704067200,"logger":"frankenphp","msg":"Uso de memoria alto","uso_actual":10485760,"uso_pico":12582912}
```

## `error_log()`

FrankenPHP también permite el registro mediante la función estándar `error_log()`. Si el parámetro `$message_type` es `4` (SAPI),
estos mensajes se redirigen al registrador de Caddy.

Por omisión, los mensajes enviados a través de `error_log()` se tratan como texto no estructurado.
Son útiles para la compatibilidad con aplicaciones o bibliotecas existentes que dependen de la biblioteca estándar de PHP.

### Ejemplo

```php
error_log("Fallo en la conexión a la base de datos", 4);
```

Esto aparecerá en los registros de Caddy, a menudo con un prefijo que indica que se originó desde PHP.

> [!TIP]
> Para una mejor observabilidad en entornos de producción, prefiera `frankenphp_log()`
> ya que permite filtrar registros por nivel (Depuración, Error, etc.)
> y consultar campos específicos en su infraestructura de registro.
