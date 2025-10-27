# Real-time

FrankenPHP comes with a built-in [Mercure](https://mercure.rocks) hub!
Mercure allows you to push real-time events to all the connected devices: they will receive a JavaScript event instantly.

It's a convenient alternative to WebSockets that is simple to use and is natively supported by all modern web browsers!

![Mercure](mercure-hub.png)

## Enabling Mercure

Mercure support is disabled by default.
Here is a minimal example of a `Caddyfile` enabling both FrankenPHP and the Mercure hub:

```caddyfile
# The hostname to respond to
localhost

mercure {
    # The secret key used to sign the JWT tokens for publishers
    publisher_jwt !ChangeThisMercureHubJWTSecretKey!
    # Allows anonymous subscribers (without JWT)
    anonymous
}

root public/
php_server
```

> [!TIP]
>
> The [sample `Caddyfile`](https://github.com/php/frankenphp/blob/main/caddy/frankenphp/Caddyfile)
> provided by [the Docker images](docker.md) already includes a commented Mercure configuration
> with convenient environment variables to configure it.
>
> Uncomment the Mercure section in `/etc/frankenphp/Caddyfile` to enable it.

## Subscribing to Updates

By default, the Mercure hub is available on the `/.well-known/mercure` path of your FrankenPHP server.
To subscribe to updates, use the native [`EventSource`](https://developer.mozilla.org/docs/Web/API/EventSource) JavaScript class:

```html
<!-- public/index.html -->
<!doctype html>
<title>Mercure Example</title>
<script>
  const eventSource = new EventSource("/.well-known/mercure?topic=my-topic");
  eventSource.onmessage = function (event) {
    console.log("New message:", event.data);
  };
</script>
```

## Publishing Updates

### Using `file_get_contents()`

To dispatch an update to connected subscribers, send an authenticated POST request to the Mercure hub with the `topic` and `data` parameters:

```php
<?php
// public/publish.php

const JWT = 'eyJhbGciOiJIUzI1NiJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiKiJdfX0.PXwpfIGng6KObfZlcOXvcnWCJOWTFLtswGI5DZuWSK4';

$updateID = file_get_contents('https://localhost/.well-known/mercure', context: stream_context_create(['http' => [
    'method'  => 'POST',
    'header'  => "Content-type: application/x-www-form-urlencoded\r\nAuthorization: Bearer " . JWT,
    'content' => http_build_query([
        'topic' => 'my-topic',
        'data' => json_encode(['key' => 'value']),
    ]),
]]));

// Write to FrankenPHP's logs
error_log("update $updateID published", 4);
```

The key passed as parameter of the `mercure.publisher_jwt` option in the `Caddyfile` must used to sign the JWT token used in the `Authorization` header.

The JWT must include a `mercure` claim with a `publish` permission for the topics you want to publish to.
See [the Mercure documentation](https://mercure.rocks/spec#publishers) about authorization.

To generate your own tokens, you can use [this jwt.io link](https://www.jwt.io/#token=eyJhbGciOiJIUzI1NiJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiKiJdfX0.PXwpfIGng6KObfZlcOXvcnWCJOWTFLtswGI5DZuWSK4),
but for production apps, it's recommended to use short-lived tokens generated aerodynamically using with a trusted [JWT library](https://www.jwt.io/libraries?programming_language=php).

### Using Symfony Mercure

Alternatively, you can use the [Symfony Mercure Component](https://symfony.com/components/Mercure), a standalone PHP library.

This library handled the JWT generation, update publishing as well as cookie-based authorization for subscribers.

First, install the library using Composer:

```console
composer require symfony/mercure lcobucci/jwt
```

Then, you can use it like this:

```php
<?php
// public/publish.php

require __DIR__ . '/../vendor/autoload.php';

const JWT_SECRET = '!ChangeThisMercureHubJWTSecretKey!'; // Must be the same as mercure.publisher_jwt in Caddyfile

// Set up the JWT token provider
$jwFactory = new \Symfony\Component\Mercure\Jwt\LcobucciFactory(JWT_SECRET);
$provider = new \Symfony\Component\Mercure\Jwt\FactoryTokenProvider($jwFactory, publish: ['*']);

$hub = new \Symfony\Component\Mercure\Hub('https://localhost/.well-known/mercure', $provider);
// Serialize the update, and dispatch it to the hub, that will broadcast it to the clients
$updateID = $hub->publish(new \Symfony\Component\Mercure\Update('my-topic', json_encode(['key' => 'value'])));

// Write to FrankenPHP's logs
error_log("update $updateID published", 4);
```

Mercure is also natively supported by:

- [Laravel](laravel.md#mercure-support)
- [Symfony](https://symfony.com/doc/current/mercure.html)
- [API Platform](https://api-platform.com/docs/core/mercure/)
