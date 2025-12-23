# Tiempo Real

¡FrankenPHP incluye un hub [Mercure](https://mercure.rocks) integrado!
Mercure te permite enviar eventos en tiempo real a todos los dispositivos conectados: recibirán un evento JavaScript al instante.

¡Es una alternativa conveniente a WebSockets que es simple de usar y es soportada nativamente por todos los navegadores web modernos!

![Mercure](../mercure-hub.png)

## Habilitando Mercure

El soporte para Mercure está deshabilitado por defecto.
Aquí tienes un ejemplo mínimo de un `Caddyfile` que habilita tanto FrankenPHP como el hub Mercure:

```caddyfile
# El nombre de host al que responder
localhost

mercure {
    # La clave secreta usada para firmar los tokens JWT para los publicadores
    publisher_jwt !CambiaEstaClaveSecretaJWTDelHubMercure!
    # Permite suscriptores anónimos (sin JWT)
    anonymous
}

root public/
php_server
```

> [!TIP]
>
> El [`Caddyfile` de ejemplo](https://github.com/php/frankenphp/blob/main/caddy/frankenphp/Caddyfile)
> proporcionado por [las imágenes Docker](docker.md) ya incluye una configuración comentada de Mercure
> con variables de entorno convenientes para configurarlo.
>
> Descomenta la sección Mercure en `/etc/frankenphp/Caddyfile` para habilitarla.

## Suscribiéndose a Actualizaciones

Por defecto, el hub Mercure está disponible en la ruta `/.well-known/mercure` de tu servidor FrankenPHP.
Para suscribirte a actualizaciones, usa la clase nativa [`EventSource`](https://developer.mozilla.org/es/docs/Web/API/EventSource) de JavaScript:

```html
<!-- public/index.html -->
<!doctype html>
<title>Ejemplo Mercure</title>
<script>
  const eventSource = new EventSource("/.well-known/mercure?topic=mi-tema");
  eventSource.onmessage = function (event) {
    console.log("Nuevo mensaje:", event.data);
  };
</script>
```

## Publicando Actualizaciones

### Usando `mercure_publish()`

FrankenPHP proporciona una función conveniente `mercure_publish()` para publicar actualizaciones en el hub Mercure integrado:

```php
<?php
// public/publish.php

$updateID = mercure_publish('mi-tema', json_encode(['clave' => 'valor']));

// Escribir en los registros de FrankenPHP
error_log("actualización $updateID publicada", 4);
```

La firma completa de la función es:

```php
/**
 * @param string|string[] $topics
 */
function mercure_publish(string|array $topics, string $data = '', bool $private = false, ?string $id = null, ?string $type = null, ?int $retry = null): string {}
```

### Usando `file_get_contents()`

Para enviar una actualización a los suscriptores conectados, envía una solicitud POST autenticada al hub Mercure con los parámetros `topic` y `data`:

```php
<?php
// public/publish.php

const JWT_SECRET = '!ChangeThisMercureHubJWTSecretKey!'; // Debe ser la misma que mercure.publisher_jwt en Caddyfile

$updateID = file_get_contents('https://localhost/.well-known/mercure', context: stream_context_create(['http' => [
    'method'  => 'POST',
    'header'  => "Content-type: application/x-www-form-urlencoded\r\nAuthorization: Bearer " . JWT,
    'content' => http_build_query([
        'topic' => 'mi-tema',
        'data' => json_encode(['clave' => 'valor']),
    ]),
]]));

// Escribir en los registros de FrankenPHP
error_log("actualización $updateID publicada", 4);
```

La clave pasada como parámetro de la opción `mercure.publisher_jwt` en el `Caddyfile` debe usarse para firmar el token JWT usado en el encabezado `Authorization`.

El JWT debe incluir un reclamo `mercure` con un permiso `publish` para los temas a los que deseas publicar.
Consulta [la documentación de Mercure](https://mercure.rocks/spec#publishers) sobre autorización.

Para generar tus propios tokens, puedes usar [este enlace de jwt.io](https://www.jwt.io/#token=eyJhbGciOiJIUzI1NiJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiKiJdfX0.PXwpfIGng6KObfZlcOXvcnWCJOWTFLtswGI5DZuWSK4),
pero para aplicaciones en producción, se recomienda usar tokens de corta duración generados dinámicamente usando una biblioteca [JWT](https://www.jwt.io/libraries?programming_language=php) confiable.

### Usando Symfony Mercure

Alternativamente, puedes usar el [Componente Symfony Mercure](https://symfony.com/components/Mercure), una biblioteca PHP independiente.

Esta biblioteca maneja la generación de JWT, la publicación de actualizaciones así como la autorización basada en cookies para los suscriptores.

Primero, instala la biblioteca usando Composer:

```console
composer require symfony/mercure lcobucci/jwt
```

Luego, puedes usarla de la siguiente manera:

```php
<?php
// public/publish.php

require __DIR__ . '/../vendor/autoload.php';

const JWT_SECRET = '!CambiaEstaClaveSecretaJWTDelHubMercure!'; // Debe ser la misma que mercure.publisher_jwt en Caddyfile

// Configurar el proveedor de tokens JWT
$jwFactory = new \Symfony\Component\Mercure\Jwt\LcobucciFactory(JWT_SECRET);
$provider = new \Symfony\Component\Mercure\Jwt\FactoryTokenProvider($jwFactory, publish: ['*']);

$hub = new \Symfony\Component\Mercure\Hub('https://localhost/.well-known/mercure', $provider);
// Serializar la actualización y enviarla al hub, que la transmitirá a los clientes
$updateID = $hub->publish(new \Symfony\Component\Mercure\Update('mi-tema', json_encode(['clave' => 'valor'])));

// Escribir en los registros de FrankenPHP
error_log("actualización $updateID publicada", 4);
```

Mercure también es soportado nativamente por:

- [Laravel](laravel.md#soporte-para-mercure)
- [Symfony](https://symfony.com/doc/current/mercure.html)
- [API Platform](https://api-platform.com/docs/core/mercure/)
