---
title: FrankenPHP worker mode: PHPアプリをメモリに保持する
description: FrankenPHPをワーカーモードで実行すると、PHPアプリケーションをリクエスト間で起動状態に保ち、起動オーバーヘッドを削減し、ミリ秒単位でレスポンスを提供できます。
---

# FrankenPHPワーカーの使用

アプリケーションを一度起動してメモリに保持します。
FrankenPHPは数ミリ秒で受信リクエストを処理します。

## FrankenPHPワーカー スクリプトの開始

### DockerでFrankenPHPワーカーを実行する

`FRANKENPHP_CONFIG`環境変数の値を`worker /path/to/your/worker/script.php`に設定します：

```bash
docker run \
    -e FRANKENPHP_CONFIG="worker /app/path/to/your/worker/script.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### スタンドアロンバイナリでFrankenPHPワーカーを実行する

`php-server`コマンドの`--worker`オプションを使って、現在のディレクトリのコンテンツをワーカーを通じて提供できます：

```bash
frankenphp php-server --worker /path/to/your/worker/script.php
```

PHPアプリが[バイナリに埋め込まれている](embed.md)場合は、アプリのルートディレクトリにカスタムの`Caddyfile`を追加することができます。
これが自動的に使用されます。

また、`--watch`オプションを使えば、[ファイルの変更に応じてワーカーを再起動](config.md#watching-for-file-changes)することも可能です。
以下のコマンドは、`/path/to/your/app/`ディレクトリおよびそのサブディレクトリ内の`.php`で終わるファイルが変更された場合に再起動をトリガーします：

```bash
frankenphp php-server --worker /path/to/your/worker/script.php --watch="/path/to/your/app/**/*.php"
```

この機能は、[ホットリロード](hot-reload.md)と組み合わせてよく使用されます。

## Symfonyのワーカーモード

[FrankenPHP Symfonyワーカーモードのドキュメント](symfony.md#symfony-worker-mode-with-frankenphp)を参照してください。

## Laravel Octaneのワーカーモード

[FrankenPHP Laravel Octaneのドキュメント](laravel.md#laravel-octane)を参照してください。

## カスタムFrankenPHPワーカー スクリプトの作成

以下の例は、サードパーティライブラリに依存せずに独自のワーカースクリプトを作成する方法を示しています：

```php
<?php
// public/index.php

// アプリケーションを起動
require __DIR__.'/vendor/autoload.php';

$myApp = new \App\Kernel();
$myApp->boot();

// パフォーマンス向上のため、ループの外側にハンドラーを配置（処理を減らす）
$handler = static function () use ($myApp) {
    try {
        // リクエストを受信すると呼び出され、
        // スーパーグローバル、php://inputなどがリセットされます。
        echo $myApp->handle($_GET, $_POST, $_COOKIE, $_FILES, $_SERVER);
    } catch (\Throwable $exception) {
        // `set_exception_handler`はワーカースクリプトが終了するときにのみ呼び出されるため、
        // 予期しない動作になる可能性があります。そのため、ここで例外をキャッチして処理します。
        (new \MyCustomExceptionHandler)->handleException($exception);
    }
};

$maxRequests = (int)($_SERVER['MAX_REQUESTS'] ?? 0);
for ($nbRequests = 0; !$maxRequests || $nbRequests < $maxRequests; ++$nbRequests) {
    $keepRunning = \frankenphp_handle_request($handler);

    // HTTPレスポンス送信後に何らかの処理を実行
    $myApp->terminate();

    // ページ生成中にガベージコレクタが起動する可能性を減らすため、ここでガベージコレクタを明示的に呼び出す
    gc_collect_cycles();

    if (!$keepRunning) break;
}

// クリーンアップ
$myApp->shutdown();
```

次に、アプリを開始し、`FRANKENPHP_CONFIG`環境変数を使用してワーカーを設定します：

```bash
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

デフォルトでは、CPU当たり2つのワーカーが開始されます。
開始するワーカー数を設定することもできます：

```bash
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php 42" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### PSR-15の使用

アプリケーションがスーパーグローバルではなく[PSR-15](https://www.php-fig.org/psr/psr-15/)を使用する場合、ハンドラーの境界で[`nyholm/psr7`](https://github.com/Nyholm/psr7)と[`nyholm/psr7-server`](https://github.com/Nyholm/psr7-server)を使用してリクエスト/レスポンスを変換します：

```console
composer require nyholm/psr7 nyholm/psr7-server psr/http-server-handler
```

```php
<?php
// public/index.php

require __DIR__.'/vendor/autoload.php';

use Nyholm\Psr7\Factory\Psr17Factory;
use Nyholm\Psr7Server\ServerRequestCreator;

$myApp = new \App\Kernel(); // Psr\Http\Server\RequestHandlerInterface を実装
$myApp->boot();

$psr17Factory = new Psr17Factory();
$creator = new ServerRequestCreator(
    $psr17Factory, // ServerRequestFactory
    $psr17Factory, // UriFactory
    $psr17Factory, // UploadedFileFactory
    $psr17Factory, // StreamFactory
);

$handler = static function () use ($myApp, $creator) {
    $response = $myApp->handle($creator->fromGlobals());

    http_response_code($response->getStatusCode());
    foreach ($response->getHeaders() as $name => $values) {
        foreach ($values as $value) {
            header("$name: $value", false);
        }
    }
    echo $response->getBody();
};

$maxRequests = (int)($_SERVER['MAX_REQUESTS'] ?? 0);
for ($nbRequests = 0; !$maxRequests || $nbRequests < $maxRequests; ++$nbRequests) {
    $keepRunning = \frankenphp_handle_request($handler);
    gc_collect_cycles();
    if (!$keepRunning) break;
}
```

### 一定数のリクエスト処理後にワーカーを再起動する

PHPはもともと長時間実行されるプロセス向けに設計されていなかったため、メモリリークを引き起こすライブラリやレガシーコードがいまだに多く存在します。
こうしたコードをワーカーモードで利用するための回避策として、一定数のリクエストを処理した後にワーカースクリプトを再起動する方法があります：

前述のワーカー用スニペットでは、`MAX_REQUESTS`という名前の環境変数を設定することで、処理する最大リクエスト数を設定できます。

### ワーカーの手動再起動

[ファイルの変更を監視](config.md#watching-for-file-changes)してワーカーを再起動することも可能ですが、
[Caddy admin API](https://caddyserver.com/docs/api)を使用してすべてのワーカーをグレースフルに（安全に）再起動することも可能です。adminが
[Caddyfile](config.md#caddyfile-config)で有効になっている場合、次のような単純なPOSTリクエストで再起動エンドポイントにpingできます：

```bash
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

### スレッド数

ワーカーモードが有効な場合、受信リクエストを処理するために追加のスレッドが必要になるため、
デフォルトの`num_threads`の値は`1 + 2x(利用可能なCPU数)`になります。

したがって、このモードで`num_threads`と`num`が明示的に設定されている場合、`num_threads`は`num`よりも厳密に大きくする必要があります。

## スーパーグローバルの動作

[PHPのスーパーグローバル](https://www.php.net/manual/language.variables.superglobals.php)（`$_SERVER`、`$_ENV`、`$_GET`など）
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

ほとんどのスーパーグローバル（`$_GET`、`$_POST`、`$_COOKIE`、`$_FILES`、`$_SERVER`、`$_REQUEST`）はリクエスト間で自動的にリセットされます。
しかし、**`$_ENV`は現在、リクエスト間でリセットされません**。
これは、リクエスト中に`$_ENV`に加えられた変更が永続し、同じワーカーのスレッドで処理される後続のリクエストに表示されることを意味します。
リクエスト固有のデータや機密データを`$_ENV`に保存することは避けてください。

## 状態の永続性

ワーカーモードではPHPプロセスがリクエスト間で起動状態を保つため、以下の状態がリクエスト間で永続します：

- **静的変数**: 関数やメソッド内で`static`として宣言された変数は、リクエスト間でその値を保持します。
- **クラスの静的プロパティ**: クラスの静的プロパティはリクエスト間で永続します。
- **グローバル変数**: ワーカー スクリプトのグローバルスコープ内の変数はリクエスト間で永続します。
- **インメモリキャッシュ**: リクエストハンドラーの外部にメモリ（配列、オブジェクト）に保存されたデータは永続します。
- **ランタイムエンジン設定**: `ini_set()`、`stream_context_set_default()`、`date_default_timezone_set()`、`chdir()`、`stream_wrapper_register()`、`set_error_handler()`、または`set_exception_handler()`で行われた変更は、それ以降のリクエストでも有効なままになります。デフォルトのストリームコンテキストに資格情報などのリクエスト固有またはユーザー固有の値を保存しないでください。代わりに、各呼び出しに明示的なコンテキストを渡してください。
- **プロセス全体の設定**: `umask()`と`setlocale()`は、現在のスレッドだけでなくFrankenPHPプロセス全体を変更します。ワーカーモードではこれらの設定がリクエスト間で復元されることはなく、どのモードにおいても同時に実行されているリクエストに影響を与えます。起動時に一度設定するか、umaskを変更する代わりに作成されたファイルに対して`chmod()`を呼び出してください。

これは意図された設計であり、ワーカーモードを高速にする要因です。ただし、意図しない副作用を避けるためには注意が必要です：

```php
<?php
function getCounter(): int {
    static $count = 0;
    return ++$count; // リクエスト間で増加します！
}

$handler = static function () {
    echo getCounter(); // このスレッドへの各リクエストに対して 1, 2, 3, ...
};

while (\frankenphp_handle_request($handler)) {
    // ...
}
```

ワーカー スクリプトを作成する際は、リクエスト固有の状態をリクエスト間でリセットするようにしてください。
[Symfony](symfony.md)や[Laravel Octane](laravel.md)のようなフレームワークは、ほとんどの状態のリセットを処理してくれますが、独自のサービスをリセットする必要がある場合があります。Symfonyの場合、リクエスト固有の状態を保持するサービスは[`Symfony\Contracts\Service\ResetInterface`](https://github.com/symfony/contracts/blob/main/Service/ResetInterface.php)を実装する必要があります。これにより、カーネルによってリクエスト間でリセットされます。
