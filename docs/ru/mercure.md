# Режим реального времени

FrankenPHP поставляется со встроенным хабом [Mercure](https://mercure.rocks)!
Mercure позволяет отправлять события в режиме реального времени на все подключенные устройства: они мгновенно получат JavaScript-событие.

Это удобная альтернатива WebSockets, простая в использовании и нативно поддерживаемая всеми современными веб-браузерами!

![Mercure](mercure-hub.png)

## Включение Mercure

Поддержка Mercure по умолчанию отключена.
Вот минимальный пример `Caddyfile`, включающий FrankenPHP и хаб Mercure:

```caddyfile
# Имя хоста, на который будет отвечать
localhost

mercure {
    # Секретный ключ, используемый для подписи JWT-токенов для издателей
    publisher_jwt !ChangeThisMercureHubJWTSecretKey!
    # Разрешает анонимных подписчиков (без JWT)
    anonymous
}

root public/
php_server
```

> [!TIP]
>
> [Пример `Caddyfile`](https://github.com/php/frankenphp/blob/main/caddy/frankenphp/Caddyfile),
> предоставляемый [образами Docker](docker.md), уже содержит закомментированную конфигурацию Mercure
> с удобными переменными окружения для настройки.
>
> Раскомментируйте раздел Mercure в `/etc/frankenphp/Caddyfile`, чтобы включить его.

## Подписка на обновления

По умолчанию хаб Mercure доступен по пути `/.well-known/mercure` на вашем сервере FrankenPHP.
Для подписки на обновления используйте нативный JavaScript-класс [`EventSource`](https://developer.mozilla.org/docs/Web/API/EventSource):

```html
<!-- public/index.html -->
<!doctype html>
<title>Пример Mercure</title>
<script>
  const eventSource = new EventSource("/.well-known/mercure?topic=my-topic");
  eventSource.onmessage = function (event) {
    console.log("Новое сообщение:", event.data);
  };
</script>
```

## Публикация обновлений

### Использование `mercure_publish()`

FrankenPHP предоставляет удобную функцию `mercure_publish()` для публикации обновлений во встроенный хаб Mercure:

```php
<?php
// public/publish.php

$updateID = mercure_publish('my-topic',  json_encode(['key' => 'value']));

// Запись в логи FrankenPHP
error_log("update $updateID published", 4);
```

Полная сигнатура функции:

```php
/**
 * @param string|string[] $topics
 */
function mercure_publish(string|array $topics, string $data = '', bool $private = false, ?string $id = null, ?string $type = null, ?int $retry = null): string {}
```

### Использование `file_get_contents()`

Для отправки обновления подключенным подписчикам отправьте аутентифицированный POST-запрос к хабу Mercure с параметрами `topic` и `data`:

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

// Запись в логи FrankenPHP
error_log("update $updateID published", 4);
```

Ключ, переданный в качестве параметра опции `mercure.publisher_jwt` в `Caddyfile`, должен использоваться для подписи JWT-токена, применяемого в заголовке `Authorization`.

JWT должен включать утверждение `mercure` с разрешением `publish` для тем, в которые вы хотите публиковать обновления. См. [документацию Mercure](https://mercure.rocks/spec#publishers) по авторизации.

Чтобы сгенерировать свои собственные токены, вы можете использовать [эту ссылку jwt.io](https://www.jwt.io/#token=eyJhbGciOiJIUzI1NiJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiKiJdfX0.PXwpfIGng6KObfZlcOXvcnWCJOWTFLtswGI5DZuWSK4), но для производственных приложений рекомендуется использовать короткоживущие токены, генерируемые динамически с помощью надежной [библиотеки JWT](https://www.jwt.io/libraries?programming_language=php).

### Использование Symfony Mercure

В качестве альтернативы вы можете использовать [компонент Symfony Mercure](https://symfony.com/components/Mercure), автономную PHP-библиотеку.

Эта библиотека обрабатывает генерацию JWT, публикацию обновлений, а также авторизацию на основе файлов cookie для подписчиков.

Сначала установите библиотеку с помощью Composer:

```console
composer require symfony/mercure lcobucci/jwt
```

Затем вы можете использовать её так:

```php
<?php
// public/publish.php

require __DIR__ . '/../vendor/autoload.php';

const JWT_SECRET = '!ChangeThisMercureHubJWTSecretKey!'; // Должен быть таким же, как mercure.publisher_jwt в Caddyfile

// Настраиваем провайдер JWT-токенов
$jwFactory = new \Symfony\Component\Mercure\Jwt\LcobucciFactory(JWT_SECRET);
$provider = new \Symfony\Component\Mercure\Jwt\FactoryTokenProvider($jwFactory, publish: ['*']);

$hub = new \Symfony\Component\Mercure\Hub('https://localhost/.well-known/mercure', $provider);
// Сериализуем обновление и отправляем его в хаб, который будет транслировать его клиентам
$updateID = $hub->publish(new \Symfony\Component\Mercure\Update('my-topic', json_encode(['key' => 'value'])));

// Запись в логи FrankenPHP
error_log("update $updateID published", 4);
```

Mercure также нативно поддерживается:

- [Laravel](laravel.md#mercure-support)
- [Symfony](https://symfony.com/doc/current/mercure.html)
- [API Platform](https://api-platform.com/docs/core/mercure/)
