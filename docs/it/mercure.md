# Real time

FrankenPHP viene fornito con un hub [Mercure](https://mercure.rocks) integrato!
Mercure consente di inviare eventi in tempo reale a tutti i dispositivi connessi: riceveranno immediatamente un evento JavaScript.

È una comoda alternativa ai WebSocket, semplice da usare e supportata nativamente da tutti i browser Web moderni!

![Mercure](../mercure-hub.png)

## Abilitare Mercure

Il supporto Mercure è disabilitato per impostazione predefinita.
Ecco un esempio minimo di `Caddyfile` che abilita sia FrankenPHP sia l'hub Mercure:

```caddyfile
# Hostname a cui rispondere
localhost

mercure {
    # Il secret usato per firmare i token JWT per i publisher
    publisher_jwt !ChangeThisMercureHubJWTSecretKey!
    # Se si imposta publisher_jwt, si deve impostare anche subscriber_jwt
    subscriber_jwt !ChangeThisMercureHubJWTSecretKey!
    # Consente subscriber anonimi (senza JWT)
    anonymous
}

root public/
php_server
```

> [!TIP]
>
> L'[esempio di `Caddyfile`](https://github.com/php/frankenphp/blob/main/caddy/frankenphp/Caddyfile)
> fornito dalle [immagini Docker](docker.md) include già una configurazione Mercure commentata
> con comode variabili d'ambiente per configurarlo.
>
> Decommentare la sezione Mercure in `/etc/frankenphp/Caddyfile` per abilitarla.

## Iscrizione agli aggiornamenti

Per impostazione predefinita, l'hub Mercure è disponibile nel percorso `/.well-known/mercure` del server FrankenPHP.
Per iscriversi agli aggiornamenti, utilizzare la classe JavaScript nativa [`EventSource`](https://developer.mozilla.org/docs/Web/API/EventSource):

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

## Pubblicazione aggiornamenti

### Utilizzo di `mercure_publish()`

FrankenPHP fornisce una comoda funzione `mercure_publish()` per pubblicare aggiornamenti sull'hub Mercure integrato:

```php
<?php
// public/publish.php

$updateID = mercure_publish('my-topic',  json_encode(['key' => 'value']));

// Scrive nel log di FrankenPHP
error_log("update $updateID published", 4);
```

La firma completa della funzione è:

```php
/**
 * @param string|string[] $topics
 */
function mercure_publish(string|array $topics, string $data = '', bool $private = false, ?string $id = null, ?string $type = null, ?int $retry = null): string {}
```

### Utilizzo di `file_get_contents()`

Per inviare un aggiornamento ai subscriber connessi, inviare una richiesta POST autenticata all'hub Mercure con i parametri `topic` e `data`:

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

// Scrive nel log di FrankenPHP
error_log("update $updateID published", 4);
```

La chiave passata come parametro dell'opzione `mercure.publisher_jwt` in `Caddyfile` deve essere utilizzata per firmare il token JWT utilizzato nell'header `Authorization`.

Il JWT deve includere un'attestazione `mercure` con un'autorizzazione `publish` per gli argomenti da pubblicare.
Consultare la [documentazione Mercure](https://mercure.rocks/spec#publishers) sull'autorizzazione.

Per generare i propri token, si può utilizzare [questo link jwt.io](https://www.jwt.io/#token=eyJhbGciOiJIUzI1NiJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiKiJdfX0.PXwpfIGng6KObfZlcOXvcnWCJOWTFLtswGI5DZuWSK4),
ma per le app di produzione, è consigliabile utilizzare token di breve durata generati dinamicamente utilizzando una [libreria JWT](https://www.jwt.io/libraries?programming_language=php) attendibile.

### Utilizzo di Symfony Mercure

In alternativa, si può utilizzare il [Componente Symfony Mercure](https://symfony.com/components/Mercure), una libreria PHP autonoma.

Questa libreria gestisce la generazione JWT, la pubblicazione degli aggiornamenti e l'autorizzazione basata su cookie per i subscriber.

Innanzitutto, installare la libreria utilizzando Composer:

```console
composer require symfony/mercure lcobucci/jwt
```

Si può quindi usare in questo modo:

```php
<?php
// public/publish.php

require __DIR__ . '/../vendor/autoload.php';

const JWT_SECRET = '!ChangeThisMercureHubJWTSecretKey!'; // Deve coincidere con mercure.publisher_jwt nel Caddyfile

// Imposta il provider JWT
$jwFactory = new \Symfony\Component\Mercure\Jwt\LcobucciFactory(JWT_SECRET);
$provider = new \Symfony\Component\Mercure\Jwt\FactoryTokenProvider($jwFactory, publish: ['*']);

$hub = new \Symfony\Component\Mercure\Hub('https://localhost/.well-known/mercure', $provider);
// Serializza l'aggiornamento e lo invio all'hub, che lo girerà ai client
$updateID = $hub->publish(new \Symfony\Component\Mercure\Update('my-topic', json_encode(['key' => 'value'])));

// Scrive nel log di FrankenPHP
error_log("update $updateID published", 4);
```

Mercure è nativamente supportato anche da:

- [Laravel](laravel.md#mercure-support)
- [Symfony](https://symfony.com/doc/current/mercure.html)
- [API Platform](https://api-platform.com/docs/core/mercure/)
