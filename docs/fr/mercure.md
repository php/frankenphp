# Temps Réel

FrankenPHP est livré avec un hub [Mercure](https://mercure.rocks) intégré !
Mercure permet de pousser des événements en temps réel vers tous les appareils connectés : ils recevront un événement JavaScript instantanément.

C'est une alternative pratique aux WebSockets, simple à utiliser et nativement prise en charge par tous les navigateurs web modernes !

![Mercure](mercure-hub.png)

## Activer Mercure

Le support de Mercure est désactivé par défaut.
Voici un exemple minimal de `Caddyfile` activant à la fois FrankenPHP et le hub Mercure :

```caddyfile
# L'hostname auquel répondre
localhost

mercure {
    # La clé secrète utilisée pour signer les jetons JWT pour les éditeurs
    publisher_jwt !ChangeThisMercureHubJWTSecretKey!
    # Autorise les abonnés anonymes (sans JWT)
    anonymous
}

root public/
php_server
```

> [!TIP]
>
> Le [fichier Caddyfile d'exemple](https://github.com/php/frankenphp/blob/main/caddy/frankenphp/Caddyfile)
> fourni par [les images Docker](docker.md) inclut déjà une configuration Mercure commentée
> avec des variables d'environnement pratiques pour le configurer.
>
> Décommentez la section Mercure dans `/etc/frankenphp/Caddyfile` pour l'activer.

## S'abonner aux mises à jour

Par défaut, le hub Mercure est disponible sur le chemin `/.well-known/mercure` de votre serveur FrankenPHP.
Pour vous abonner aux mises à jour, utilisez la classe JavaScript native [`EventSource`](https://developer.mozilla.org/docs/Web/API/EventSource) :

```html
<!-- public/index.html -->
<!doctype html>
<title>Exemple Mercure</title>
<script>
  const eventSource = new EventSource("/.well-known/mercure?topic=my-topic");
  eventSource.onmessage = function (event) {
    console.log("Nouveau message :", event.data);
  };
</script>
```

## Publier des mises à jour

### Utilisation de `mercure_publish()`

FrankenPHP fournit une fonction pratique `mercure_publish()` pour publier des mises à jour vers le hub Mercure intégré :

```php
<?php
// public/publish.php

$updateID = mercure_publish('my-topic',  json_encode(['key' => 'value']));

// Écrit dans les logs de FrankenPHP
error_log("update $updateID published", 4);
```

La signature complète de la fonction est :

```php
/**
 * @param string|string[] $topics
 */
function mercure_publish(string|array $topics, string $data = '', bool $private = false, ?string $id = null, ?string $type = null, ?int $retry = null): string {}
```

### Utilisation de `file_get_contents()`

Pour distribuer une mise à jour aux abonnés connectés, envoyez une requête POST authentifiée au hub Mercure avec les paramètres `topic` et `data` :

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

// Écrit dans les logs de FrankenPHP
error_log("update $updateID published", 4);
```

La clé passée en paramètre de l'option `mercure.publisher_jwt` dans le `Caddyfile` doit être utilisée pour signer le jeton JWT utilisé dans l'en-tête `Authorization`.

Le JWT doit inclure une revendication `mercure` avec une permission `publish` pour les sujets auxquels vous souhaitez publier.
Consultez [la documentation Mercure](https://mercure.rocks/spec#publishers) concernant l'autorisation.

Pour générer vos propres jetons, vous pouvez utiliser [ce lien jwt.io](https://www.jwt.io/#token=eyJhbGciOiJIUzI1NiJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiKiJdfX0.PXwpfIGng6KObfZlcOXvcnWCJOWTFLtswGI5DZuWSK4),
mais pour les applications en production, il est recommandé d'utiliser des jetons de courte durée générés dynamiquement à l'aide d'une [bibliothèque JWT](https://www.jwt.io/libraries?programming_language=php) fiable.

### Utilisation de Symfony Mercure

Alternativement, vous pouvez utiliser le [Composant Mercure de Symfony](https://symfony.com/components/Mercure), une bibliothèque PHP autonome.

Cette bibliothèque gère la génération de JWT, la publication de mises à jour ainsi que l'autorisation basée sur les cookies pour les abonnés.

Tout d'abord, installez la bibliothèque à l'aide de Composer :

```console
composer require symfony/mercure lcobucci/jwt
```

Ensuite, vous pouvez l'utiliser comme ceci :

```php
<?php
// public/publish.php

require __DIR__ . '/../vendor/autoload.php';

const JWT_SECRET = '!ChangeThisMercureHubJWTSecretKey!'; // Doit être identique à mercure.publisher_jwt dans le Caddyfile

// Configure le fournisseur de jetons JWT
$jwFactory = new \Symfony\Component\Mercure\Jwt\LcobucciFactory(JWT_SECRET);
$provider = new \Symfony\Component\Mercure\Jwt\FactoryTokenProvider($jwFactory, publish: ['*']);

$hub = new \Symfony\Component\Mercure\Hub('https://localhost/.well-known/mercure', $provider);
// Sérialise la mise à jour et la distribue au hub, qui la diffusera aux clients
$updateID = $hub->publish(new \Symfony\Component\Mercure\Update('my-topic', json_encode(['key' => 'value'])));

// Écrit dans les logs de FrankenPHP
error_log("update $updateID published", 4);
```

Mercure est également pris en charge nativement par :

- [Laravel](laravel.md#mercure-support)
- [Symfony](https://symfony.com/doc/current/mercure.html)
- [API Platform](https://api-platform.com/docs/core/mercure/)
