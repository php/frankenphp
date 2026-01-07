# Gerçek Zamanlı

FrankenPHP yerleşik bir [Mercure](https://mercure.rocks) hub ile birlikte gelir!
Mercure, tüm bağlı cihazlara gerçek zamanlı olaylar göndermenizi sağlar: anında bir JavaScript olayı alırlar.

WebSockets'e uygun, kullanımı kolay ve tüm modern web tarayıcıları tarafından doğal olarak desteklenen pratik bir alternatiftir!

![Mercure](mercure-hub.png)

## Mercure'ü Etkinleştirme

Mercure desteği varsayılan olarak devre dışıdır.
İşte hem FrankenPHP'yi hem de Mercure hub'ını etkinleştiren minimal bir `Caddyfile` örneği:

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
> [Docker imajları](docker.md) tarafından sağlanan [örnek `Caddyfile`](https://github.com/php/frankenphp/blob/main/caddy/frankenphp/Caddyfile),
> Mercure yapılandırmasını kolay ortam değişkenleriyle yapılandırmak için zaten yorumlanmış bir şekilde içerir.
>
> Etkinleştirmek için `/etc/frankenphp/Caddyfile` içindeki Mercure bölümünün yorumunu kaldırın.

## Güncellemelere Abone Olma

Varsayılan olarak, Mercure hub'ı FrankenPHP sunucunuzun `/.well-known/mercure` yolu üzerinde mevcuttur.
Güncellemelere abone olmak için yerel [`EventSource`](https://developer.mozilla.org/docs/Web/API/EventSource) JavaScript sınıfını kullanın:

```html
<!-- public/index.html -->
<!doctype html>
<title>Mercure Örneği</title>
<script>
  const eventSource = new EventSource("/.well-known/mercure?topic=my-topic");
  eventSource.onmessage = function (event) {
    console.log("Yeni mesaj:", event.data);
  };
</script>
```

## Güncellemeleri Yayınlama

### `mercure_publish()` Kullanarak

FrankenPHP, yerleşik Mercure hub'ına güncellemeler yayınlamak için kullanışlı bir `mercure_publish()` işlevi sunar:

```php
<?php
// public/publish.php

$updateID = mercure_publish('my-topic',  json_encode(['key' => 'value']));

// FrankenPHP'nin loglarına yaz
error_log("update $updateID published", 4);
```

Tam işlev imzası şöyledir:

```php
/**
 * @param string|string[] $topics
 */
function mercure_publish(string|array $topics, string $data = '', bool $private = false, ?string $id = null, ?string $type = null, ?int $retry = null): string {}
```

### `file_get_contents()` Kullanarak

Bağlı abonelere bir güncelleme göndermek için Mercure hub'ına `topic` ve `data` parametreleriyle kimliği doğrulanmış bir POST isteği gönderin:

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

// FrankenPHP'nin loglarına yaz
error_log("update $updateID published", 4);
```

`Caddyfile` içindeki `mercure.publisher_jwt` seçeneğinin parametresi olarak geçirilen anahtar, `Authorization` başlığında kullanılan JWT tokenını imzalamak için kullanılmalıdır.

JWT, yayınlamak istediğiniz konular için `publish` izni içeren bir `mercure` talebi içermelidir.
Yetkilendirme hakkında [Mercure dokümantasyonuna](https://mercure.rocks/spec#publishers) bakın.

Kendi tokenlarınızı oluşturmak için [bu jwt.io bağlantısını](https://www.jwt.io/#token=eyJhbGciOiJIUzI1NiJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiKiJdfX0.PXwpfIGng6KObfZlcOXvcnWCJOWTFLtswGI5DZuWSK4) kullanabilirsiniz,
ancak üretim uygulamaları için, güvenilir bir [JWT kütüphanesi](https://www.jwt.io/libraries?programming_language=php) kullanılarak dinamik olarak oluşturulan kısa ömürlü tokenlar kullanılması önerilir.

### Symfony Mercure Kullanarak

Alternatif olarak, bağımsız bir PHP kütüphanesi olan [Symfony Mercure Bileşenini](https://symfony.com/components/Mercure) kullanabilirsiniz.

Bu kütüphane, JWT oluşturma, güncelleme yayınlama ve aboneler için çerez tabanlı yetkilendirme işlemlerini ele alır.

İlk olarak, kütüphaneyi Composer kullanarak yükleyin:

```console
composer require symfony/mercure lcobucci/jwt
```

Daha sonra, bu şekilde kullanabilirsiniz:

```php
<?php
// public/publish.php

require __DIR__ . '/../vendor/autoload.php';

const JWT_SECRET = '!ChangeThisMercureHubJWTSecretKey!'; // Caddyfile'daki mercure.publisher_jwt ile aynı olmalıdır

// JWT token sağlayıcısını ayarla
$jwFactory = new \Symfony\Component\Mercure\Jwt\LcobucciFactory(JWT_SECRET);
$provider = new \Symfony\Component\Mercure\Jwt\FactoryTokenProvider($jwFactory, publish: ['*']);

$hub = new \Symfony\Component\Mercure\Hub('https://localhost/.well-known/mercure', $provider);
// Güncellemeyi serileştir ve hub'a gönder, hub da bunu istemcilere yayınlayacak
$updateID = $hub->publish(new \Symfony\Component\Mercure\Update('my-topic', json_encode(['key' => 'value'])));

// FrankenPHP'nin loglarına yaz
error_log("update $updateID published", 4);
```

Mercure ayrıca aşağıdaki tarafından doğal olarak desteklenir:

- [Laravel](laravel.md#mercure-support)
- [Symfony](https://symfony.com/doc/current/mercure.html)
- [API Platform](https://api-platform.com/docs/core/mercure/)
