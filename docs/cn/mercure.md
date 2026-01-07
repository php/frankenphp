# 实时

FrankenPHP 内置了 [Mercure](https://mercure.rocks) 中心！
Mercure 允许你将实时事件推送到所有连接的设备：它们将立即收到 JavaScript 事件。

它是 WebSockets 的便捷替代方案，使用简单，并原生支持所有现代网络浏览器！

![Mercure](mercure-hub.png)

## 启用 Mercure

Mercure 支持默认是禁用的。
这是一个启用 FrankenPHP 和 Mercure 中心的 `Caddyfile` 最小示例：

```caddyfile
# 要响应的主机名
localhost

mercure {
    # 用于签署发布者 JWT 令牌的密钥
    publisher_jwt !ChangeThisMercureHubJWTSecretKey!
    # 允许匿名订阅者（无需 JWT）
    anonymous
}

root public/
php_server
```

> [!TIP]
>
> [Docker 镜像](docker.md) 提供的 [示例 `Caddyfile`](https://github.com/php/frankenphp/blob/main/caddy/frankenphp/Caddyfile)
> 已经包含一个被注释掉的 Mercure 配置，其中带有方便的环境变量来配置它。
>
> 取消注释 `/etc/frankenphp/Caddyfile` 中的 Mercure 部分即可启用它。

## 订阅更新

默认情况下，Mercure 中心在你的 FrankenPHP 服务器的 `/.well-known/mercure` 路径上可用。
要订阅更新，请使用原生的 [`EventSource`](https://developer.mozilla.org/docs/Web/API/EventSource) JavaScript 类：

```html
<!-- public/index.html -->
<!doctype html>
<title>Mercure 示例</title>
<script>
  const eventSource = new EventSource("/.well-known/mercure?topic=my-topic");
  eventSource.onmessage = function (event) {
    console.log("新消息:", event.data);
  };
</script>
```

## 发布更新

### 使用 `mercure_publish()`

FrankenPHP 提供了一个方便的 `mercure_publish()` 函数来将更新发布到内置的 Mercure 中心：

```php
<?php
// public/publish.php

$updateID = mercure_publish('my-topic',  json_encode(['key' => 'value']));

// 写入 FrankenPHP 的日志
error_log("update $updateID published", 4);
```

完整的函数签名是：

```php
/**
 * @param string|string[] $topics
 */
function mercure_publish(string|array $topics, string $data = '', bool $private = false, ?string $id = null, ?string $type = null, ?int $retry = null): string {}
```

### 使用 `file_get_contents()`

要将更新分发给连接的订阅者，请向 Mercure 中心发送一个带 `topic` 和 `data` 参数的认证 POST 请求：

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

// 写入 FrankenPHP 的日志
error_log("update $updateID published", 4);
```

在 `Caddyfile` 中作为 `mercure.publisher_jwt` 选项参数传入的密钥必须用于签署 `Authorization` 头中使用的 JWT 令牌。

JWT 必须包含一个 `mercure` 声明，其中包含你希望发布到的 topic 的 `publish` 权限。
有关授权，请参阅 [Mercure 文档](https://mercure.rocks/spec#publishers)。

要生成自己的令牌，你可以使用 [这个 jwt.io 链接](https://www.jwt.io/#token=eyJhbGciOiJIUzI1NiJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiKiJdfX0.PXwpfIGng6KObfZlcOXvcnWCJOWTFLtswGI5DZuWSK4)，
但对于生产应用程序，建议使用受信任的 [JWT 库](https://www.jwt.io/libraries?programming_language=php) 动态生成的短期令牌。

### 使用 Symfony Mercure

或者，你可以使用 [Symfony Mercure Component](https://symfony.com/components/Mercure)，这是一个独立的 PHP 库。

该库处理 JWT 生成、更新发布以及订阅者的基于 cookie 的授权。

首先，使用 Composer 安装该库：

```console
composer require symfony/mercure lcobucci/jwt
```

然后，你可以像这样使用它：

```php
<?php
// public/publish.php

require __DIR__ . '/../vendor/autoload.php';

const JWT_SECRET = '!ChangeThisMercureHubJWTSecretKey!'; // 必须与 Caddyfile 中 mercure.publisher_jwt 相同

// 设置 JWT 令牌提供者
$jwFactory = new \Symfony\Component\Mercure\Jwt\LcobucciFactory(JWT_SECRET);
$provider = new \Symfony\Component\Mercure\Jwt\FactoryTokenProvider($jwFactory, publish: ['*']);

$hub = new \Symfony\Component\Mercure\Hub('https://localhost/.well-known/mercure', $provider);
// 序列化更新，并将其分派到中心，中心会将其广播给客户端
$updateID = $hub->publish(new \Symfony\Component\Mercure\Update('my-topic', json_encode(['key' => 'value'])));

// 写入 FrankenPHP 的日志
error_log("update $updateID published", 4);
```

Mercure 也被以下框架原生支持：

-   [Laravel](laravel.md#mercure-support)
-   [Symfony](https://symfony.com/doc/current/mercure.html)
-   [API Platform](https://api-platform.com/docs/core/mercure/)
