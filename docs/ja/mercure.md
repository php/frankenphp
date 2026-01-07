# リアルタイム

FrankenPHPには組み込みの[Mercure](https://mercure.rocks)ハブが付属しています！
Mercureを使用すると、接続されているすべてのデバイスにリアルタイムイベントをプッシュでき、各デバイスは即座にJavaScriptイベントを受信します。

これはWebSocketsに代わる便利な方法で、使い方が簡単で、すべてのモダンなウェブブラウザでネイティブにサポートされています！

![Mercure](mercure-hub.png)

## Mercureを有効にする

Mercureのサポートはデフォルトで無効になっています。
以下は、FrankenPHPとMercureハブの両方を有効にする`Caddyfile`の最小限の例です。

```caddyfile
# 応答するホスト名
localhost

mercure {
    # パブリッシャー用のJWTトークンを署名するために使用される秘密鍵
    publisher_jwt !ChangeThisMercureHubJWTSecretKey!
    # 匿名購読者（JWTなし）を許可する
    anonymous
}

root public/
php_server
```

> [!ヒント]
>
> [Dockerイメージ](docker.md)によって提供される[サンプル `Caddyfile`](https://github.com/php/frankenphp/blob/main/caddy/frankenphp/Caddyfile)には、便利な環境変数で設定できる、コメントアウトされたMercure設定がすでに含まれています。
>
> `/etc/frankenphp/Caddyfile`のMercureセクションのコメントを解除して有効にしてください。

## 更新を購読する

デフォルトでは、MercureハブはFrankenPHPサーバーの`/.well-known/mercure`パスで利用可能です。
更新を購読するには、ネイティブの[`EventSource`](https://developer.mozilla.org/docs/Web/API/EventSource) JavaScriptクラスを使用します。

```html
<!-- public/index.html -->
<!doctype html>
<title>Mercureの例</title>
<script>
  const eventSource = new EventSource("/.well-known/mercure?topic=my-topic");
  eventSource.onmessage = function (event) {
    console.log("新しいメッセージ:", event.data);
  };
</script>
```

## 更新を公開する

### `mercure_publish()`を使用する

FrankenPHPは、組み込みのMercureハブに更新を公開するための便利な`mercure_publish()`関数を提供します。

```php
<?php
// public/publish.php

$updateID = mercure_publish('my-topic',  json_encode(['key' => 'value']));

// FrankenPHPのログに書き込む
error_log("update $updateID published", 4);
```

関数の完全なシグネチャは次のとおりです。

```php
/**
 * @param string|string[] $topics
 */
function mercure_publish(string|array $topics, string $data = '', bool $private = false, ?string $id = null, ?string $type = null, ?int $retry = null): string {}
```

### `file_get_contents()`を使用する

接続された購読者に更新をディスパッチするには、`topic`および`data`パラメーターを含む認証済みPOSTリクエストをMercureハブに送信します。

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

// FrankenPHPのログに書き込む
error_log("update $updateID published", 4);
```

`Caddyfile`の`mercure.publisher_jwt`オプションのパラメーターとして渡されたキーは、`Authorization`ヘッダーで使用されるJWTトークンの署名に用いられなければなりません。

JWTには、公開したいトピックに対する`publish`権限を持つ`mercure`クレームを含める必要があります。認可については、[Mercureのドキュメント](https://mercure.rocks/spec#publishers)を参照してください。

独自のトークンを生成するには、[このjwt.ioリンク](https://www.jwt.io/#token=eyJhbGciOiJIUzI1NiJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiKiJdfX0.PXwpfIGng6KObfZlcOXvcnWCJOWTFLtswGI5DZuWSK4)を使用できますが、本番環境のアプリケーションでは、信頼できる[JWTライブラリ](https://www.jwt.io/libraries?programming_language=php)を使用して動的に生成される短命のトークンを使用することをお勧めします。

### Symfony Mercureを使用する

あるいは、スタンドアロンのPHPライブラリである[Symfony Mercureコンポーネント](https://symfony.com/components/Mercure)を使用することもできます。

このライブラリは、JWTの生成、更新の公開、および購読者向けのクッキーベースの認可を処理します。

まず、Composerを使用してライブラリをインストールします。

```console
composer require symfony/mercure lcobucci/jwt
```

その後、次のように使用できます。

```php
<?php
// public/publish.php

require __DIR__ . '/../vendor/autoload.php';

const JWT_SECRET = '!ChangeThisMercureHubJWTSecretKey!'; // Caddyfileのmercure.publisher_jwtと同じである必要があります

// JWTトークンプロバイダーを設定する
$jwFactory = new \Symfony\Component\Mercure\Jwt\LcobucciFactory(JWT_SECRET);
$provider = new \Symfony\Component\Mercure\Jwt\FactoryTokenProvider($jwFactory, publish: ['*']);

$hub = new \Symfony\Component\Mercure\Hub('https://localhost/.well-known/mercure', $provider);
// 更新をシリアライズし、ハブにディスパッチして、クライアントにブロードキャストします
$updateID = $hub->publish(new \Symfony\Component\Mercure\Update('my-topic', json_encode(['key' => 'value'])));

// FrankenPHPのログに書き込む
error_log("update $updateID published", 4);
```

Mercureは、以下によってもネイティブにサポートされています。

- [Laravel](laravel.md#mercure-support)
- [Symfony](https://symfony.com/doc/current/mercure.html)
- [API Platform](https://api-platform.com/docs/core/mercure/)
