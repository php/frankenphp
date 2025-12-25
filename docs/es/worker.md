# Usando los Workers de FrankenPHP

Inicia tu aplicación una vez y manténla en memoria.
FrankenPHP gestionará las peticiones entrantes en unos pocos milisegundos.

## Iniciando Scripts de Worker

### Docker

Establece el valor de la variable de entorno `FRANKENPHP_CONFIG` a `worker /ruta/a/tu/script/worker.php`:

```console
docker run \
    -e FRANKENPHP_CONFIG="worker /app/ruta/a/tu/script/worker.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### Binario Autónomo

Usa la opción `--worker` del comando `php-server` para servir el contenido del directorio actual usando un worker:

```console
frankenphp php-server --worker /ruta/a/tu/script/worker.php
```

Si tu aplicación PHP está [incrustada en el binario](embed.md), puedes agregar un `Caddyfile` personalizado en el directorio raíz de la aplicación.
Será usado automáticamente.

También es posible [reiniciar el worker al detectar cambios en archivos](config.md#watching-for-file-changes) con la opción `--watch`.
El siguiente comando activará un reinicio si algún archivo que termine en `.php` en el directorio `/ruta/a/tu/app/` o sus subdirectorios es modificado:

```console
frankenphp php-server --worker /ruta/a/tu/script/worker.php --watch="/ruta/a/tu/app/**/*.php"
```

Esta función se utiliza frecuentemente en combinación con [hot reloading](hot-reload.md).

## Symfony Runtime

> [!TIP]
> La siguiente sección es necesaria solo para versiones anteriores a Symfony 7.4, donde se introdujo soporte nativo para el modo worker de FrankenPHP.

El modo worker de FrankenPHP es soportado por el [Componente Runtime de Symfony](https://symfony.com/doc/current/components/runtime.html).
Para iniciar cualquier aplicación Symfony en un worker, instala el paquete FrankenPHP de [PHP Runtime](https://github.com/php-runtime/runtime):

```console
composer require runtime/frankenphp-symfony
```

Inicia tu servidor de aplicación definiendo la variable de entorno `APP_RUNTIME` para usar el Runtime de FrankenPHP Symfony:

```console
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php" \
    -e APP_RUNTIME=Runtime\\FrankenPhpSymfony\\Runtime \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

## Laravel Octane

Consulta [la documentación dedicada](laravel.md#laravel-octane).

## Aplicaciones Personalizadas

El siguiente ejemplo muestra cómo crear tu propio script de worker sin depender de una biblioteca de terceros:

```php
<?php
// public/index.php

// Prevenir la terminación del script de worker cuando una conexión de cliente se interrumpe
ignore_user_abort(true);

// Iniciar tu aplicación
require __DIR__.'/vendor/autoload.php';

$myApp = new \App\Kernel();
$myApp->boot();

// Manejador fuera del bucle para mejor rendimiento (menos trabajo)
$handler = static function () use ($myApp) {
    try {
        // Llamado cuando se recibe una petición,
        // las superglobales, php://input y similares se reinician
        echo $myApp->handle($_GET, $_POST, $_COOKIE, $_FILES, $_SERVER);
    } catch (\Throwable $exception) {
        // `set_exception_handler` se llama solo cuando el script de worker termina,
        // lo cual puede no ser lo que esperas, así que captura y maneja excepciones aquí
        (new \MyCustomExceptionHandler)->handleException($exception);
    }
};

$maxRequests = (int)($_SERVER['MAX_REQUESTS'] ?? 0);
for ($nbRequests = 0; !$maxRequests || $nbRequests < $maxRequests; ++$nbRequests) {
    $keepRunning = \frankenphp_handle_request($handler);

    // Haz algo después de enviar la respuesta HTTP
    $myApp->terminate();

    // Llama al recolector de basura para reducir las posibilidades de que se active en medio de la generación de una página
    gc_collect_cycles();

    if (!$keepRunning) break;
}

// Limpieza
$myApp->shutdown();
```

Luego, inicia tu aplicación y usa la variable de entorno `FRANKENPHP_CONFIG` para configurar tu worker:

```console
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

Por omisión, se inician 2 workers por CPU.
También puedes configurar el número de workers a iniciar:

```console
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php 42" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### Reiniciar el Worker Después de un Número Determinado de Peticiones

Como PHP no fue diseñado originalmente para procesos de larga duración, aún hay muchas bibliotecas y códigos heredados que generan fugas de memoria.
Una solución para usar este tipo de código en modo worker es reiniciar el script de worker después de procesar un cierto número de peticiones:

El fragmento de worker anterior permite configurar un número máximo de peticiones a manejar estableciendo una variable de entorno llamada `MAX_REQUESTS`.

### Reiniciar Workers Manualmente

Aunque es posible reiniciar workers [al detectar cambios en archivos](config.md#watching-for-file-changes), también es posible reiniciar todos los workers
de manera controlada a través de la [API de administración de Caddy](https://caddyserver.com/docs/api). Si el admin está habilitado en tu
[Caddyfile](config.md#caddyfile-config), puedes activar el endpoint de reinicio con una simple petición POST como esta:

```console
curl -X POST http://localhost:2019/frankenphp/workers/restart
```

### Fallos en Workers

Si un script de worker falla con un código de salida distinto de cero, FrankenPHP lo reiniciará con una estrategia de retroceso exponencial.
Si el script de worker permanece activo más tiempo que el último retroceso * 2,
no penalizará al script de worker y lo reiniciará nuevamente.
Sin embargo, si el script de worker continúa fallando con un código de salida distinto de cero en un corto período de tiempo
(por ejemplo, tener un error tipográfico en un script), FrankenPHP fallará con el error: `too many consecutive failures`.

El número de fallos consecutivos puede configurarse en tu [Caddyfile](config.md#caddyfile-config) con la opción `max_consecutive_failures`:

```caddyfile
frankenphp {
    worker {
        # ...
        max_consecutive_failures 10
    }
}
```

## Comportamiento de las Superglobales

Las [superglobales de PHP](https://www.php.net/manual/es/language.variables.superglobals.php) (`$_SERVER`, `$_ENV`, `$_GET`...)
se comportan de la siguiente manera:

- antes de la primera llamada a `frankenphp_handle_request()`, las superglobales contienen valores vinculados al script de worker en sí
- durante y después de la llamada a `frankenphp_handle_request()`, las superglobales contienen valores generados a partir de la petición HTTP procesada; cada llamada a `frankenphp_handle_request()` cambia los valores de las superglobales

Para acceder a las superglobales del script de worker dentro de la retrollamada, debes copiarlas e importar la copia en el ámbito de la retrollamada:

```php
<?php
// Copia la superglobal $_SERVER del worker antes de la primera llamada a frankenphp_handle_request()
$workerServer = $_SERVER;

$handler = static function () use ($workerServer) {
    var_dump($_SERVER); // $_SERVER vinculado a la petición
    var_dump($workerServer); // $_SERVER del script de worker
};

// ...
```
