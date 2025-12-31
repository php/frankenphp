# FrankenPHPワーカーの使用

アプリケーションを一度起動してメモリに保持します。
FrankenPHPは数ミリ秒で受信リクエストを処理します。

## ワーカースクリプトの開始

### Docker

`FRANKENPHP_CONFIG`環境変数の値を`worker /path/to/your/worker/script.php`に設定します：

```console
docker run \
    -e FRANKENPHP_CONFIG="worker /app/path/to/your/worker/script.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### スタンドアロンバイナリ

`php-server`コマンドの`--worker`オプションを使って、現在のディレクトリのコンテンツをワーカーを通じて提供できます：

```console
frankenphp php-server --worker /path/to/your/worker/script.php
```

PHPアプリが[バイナリに埋め込まれている](embed.md)場合は、アプリのルートディレクトリにカスタムの`Caddyfile`を追加することができます。
これが自動的に使用されます。

また、`--watch`オプションを使えば、[ファイルの変更に応じてワーカーを再起動](config.md#watching-for-file-changes)することも可能です。
以下のコマンドは、`/path/to/your/app/`ディレクトリおよびそのサブディレクトリ内の`.php`で終わるファイルが変更された場合に再起動をトリガーします：

```console
frankenphp php-server --worker /path/to/your/worker/script.php --watch="/path/to/your/app/**/*.php"
```

## Symfonyランタイム

FrankenPHPのワーカーモードは[Symfony Runtime Component](https://symfony.com/doc/current/components/runtime.html)によってサポートされています。
ワーカーでSymfonyアプリケーションを開始するには、FrankenPHP用の[PHP Runtime](https://github.com/php-runtime/runtime)パッケージをインストールします：

```console
composer require runtime/frankenphp-symfony
```

アプリケーションサーバーを起動するには、FrankenPHP Symfony Runtimeを使用するように`APP_RUNTIME`環境変数を定義します：

```console
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php" \
    -e APP_RUNTIME=Runtime\\FrankenPhpSymfony\\Runtime \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

## Laravel Octane

[専用ドキュメント](laravel.md#laravel-octane)を参照してください。

## カスタムアプリ

以下の例は、サードパーティライブラリに依存せずに独自のワーカースクリプトを作成する方法を示しています：

```php
<?php
// public/index.php

// クライアント接続が中断されたときのワーカースクリプト終了を防ぐ
ignore_user_abort(true);

// アプリを起動
require __DIR__.'/vendor/autoload.php';

$myApp = new \App\Kernel();
$myApp->boot();

// ループの外側にハンドラーを配置してパフォーマンスを向上（処理量を減らす）
$handler = static function () use ($myApp) {
    // リクエストを受信した際に呼び出され、
    // スーパーグローバルや php://input などがリセットされます。
    echo $myApp->handle($_GET, $_POST, $_COOKIE, $_FILES, $_SERVER);
};

$maxRequests = (int)($_SERVER['MAX_REQUESTS'] ?? 0);
for ($nbRequests = 0; !$maxRequests || $nbRequests < $maxRequests; ++$nbRequests) {
    $keepRunning = \frankenphp_handle_request($handler);

    // HTTPレスポンスの送信後に何か処理を行います
    $myApp->terminate();

    // ページ生成の途中でガベージコレクタが起動する可能性を減らすために、ここでガベージコレクタを明示的に呼び出す。
    gc_collect_cycles();

    if (!$keepRunning) break;
}

// クリーンアップ
$myApp->shutdown();
```

次に、アプリを開始し、`FRANKENPHP_CONFIG`環境変数を使用してワーカーを設定します：

```console
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

デフォルトでは、CPU当たり2つのワーカーが開始されます。
開始するワーカー数を設定することもできます：

```console
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php 42" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### 一定数のリクエスト処理後にワーカーを再起動する

PHPはもともと長時間実行されるプロセス向けに設計されていなかったため、メモリリークを引き起こすライブラリやレガシーコードがいまだに多く存在します。
こうしたコードをワーカーモードで利用するための回避策として、一定数のリクエストを処理した後にワーカースクリプトを再起動する方法があります：

前述のワーカー用スニペットでは、`MAX_REQUESTS`という名前の環境変数を設定することで、処理する最大リクエスト数を設定できます。

### ワーカーの手動再起動

[ファイルの変更を監視](config.md#watching-for-file-changes)してワーカーを再起動することも可能ですが、
[Caddy admin API](https://caddyserver.com/docs/api)を使用してすべてのワーカーをグレースフルに（安全に）再起動することも可能です。adminが
[Caddyfile](config.md#caddyfile-config)で有効になっている場合、次のような単純なPOSTリクエストで再起動エンドポイントにpingできます：

```console
curl -X POST http://localhost:2019/frankenphp/workers/restart
```

### ワーカーの失敗

ワーカースクリプトがゼロ以外の終了コードでクラッシュした場合、FrankenPHP は指数的バックオフ戦略を用いて再起動を行います。
ワーカースクリプトが最後のバックオフ時間 × 2 より長く稼働し続けた場合、
それ以降の再起動ではペナルティを科しません。
しかし、スクリプトにタイプミスがあるなど短時間で何度もゼロ以外の終了コードで失敗し続ける場合、
FrankenPHP は`too many consecutive failures`というエラーとともにクラッシュします。

連続失敗の回数上限は、[Caddyfile](config.md#caddyfile-config)の`max_consecutive_failures`オプションで設定できます:

```caddyfile
frankenphp {
    worker {
        # ...
        max_consecutive_failures 10
    }
}
```

## スーパーグローバルの動作

[PHPのスーパーグローバル](https://www.php.net/manual/en/language.variables.superglobals.php)（`$_SERVER`、`$_ENV`、`$_GET`など）
は以下のように動作します：

- `frankenphp_handle_request()`が最初に呼び出される前は、スーパーグローバルにはワーカースクリプト自体にバインドされた値が格納されています
- `frankenphp_handle_request()`の呼び出し中および呼び出し後は、スーパーグローバルには処理されたHTTPリクエストから生成された値が格納され、`frankenphp_handle_request()`を呼び出すたびにスーパーグローバルの値が変更されます

コールバック内でワーカースクリプトのスーパーグローバルにアクセスするには、それらをコピーしてコールバックのスコープにコピーをインポートする必要があります：

```php
<?php
// frankenphp_handle_request()を最初に呼び出す前に、ワーカーの $_SERVER スーパーグローバルをコピー
$workerServer = $_SERVER;

$handler = static function () use ($workerServer) {
    var_dump($_SERVER); // リクエストにバインドされた $_SERVER
    var_dump($workerServer); // ワーカースクリプトの $_SERVER
};

// ...
```
