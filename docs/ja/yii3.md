---
title: FrankenPHPでYii3を実行する (Docker、ワーカーモード)
description: Dockerイメージ、ローカルインストール、または yii-runner-frankenphp パッケージを使用したワーカーモードで、Yii3アプリケーションをFrankenPHPで実行する方法。
---

# Yii3

## FrankenPHP DockerイメージでYii3を実行する

FrankenPHPで[Yii](https://www.yiiframework.com/)ウェブアプリケーションを配信することは、公式Dockerイメージの`/app`ディレクトリにプロジェクトをマウントするのと同じくらい簡単です。

Yiiアプリケーションのメインディレクトリから以下のコマンドを実行してください。

```console
docker run -p 80:80 -p 443:443 -p 443:443/udp -v $PWD:/app dunglas/frankenphp
```

お楽しみください！

## FrankenPHPをローカルにインストールしてYii3を実行する

または、ローカルマシンからFrankenPHPでYiiプロジェクトを実行することもできます。

1. [お使いのシステムに対応するバイナリをダウンロードしてください](../#standalone-binary)
2. Yiiプロジェクトのルートディレクトリにある`Caddyfile`というファイルに、以下の設定を追加してください。

   ```caddyfile
   {
   	frankenphp
   }

   # サーバーのドメイン名
   localhost {
   	# 圧縮を有効にする (オプション)
   	encode zstd br gzip
   	# public/ ディレクトリからPHPファイルを実行し、アセットを配信する
   	php_server {
   		root public/
   		try_files {path} index.php
   	}
   }
   ```

3. YiiプロジェクトのルートディレクトリからFrankenPHPを起動します: `frankenphp run`

## Yii3 ワーカーモード

Yiiアプリケーションを[ワーカーモード](worker.md)で実行するには、[Yii FrankenPHPランナー](https://github.com/yiisoft/yii-runner-frankenphp)をインストールしてください。

```console
composer require yiisoft/yii-runner-frankenphp
```

次に、アプリケーションのルートディレクトリに`worker.php`という名前のファイルを作成します。

```php
<?php

declare(strict_types=1);

use Yiisoft\Yii\Runner\FrankenPHP\FrankenPHPApplicationRunner;

require_once __DIR__ . '/vendor/autoload.php';

(new FrankenPHPApplicationRunner(rootPath: __DIR__))->run();
```

アプリケーションをワーカーモードで起動するように`Caddyfile`を更新します。

```caddyfile
{
	frankenphp
}

localhost {
	encode zstd br gzip
	php_server {
		root public/
		worker ./worker.php {
			# すべてのリクエストをワーカーに送信する
			match *
			# PHPファイルが変更されたときにワーカーをリロードする (開発時のみ)
			watch ./**/*.php
		}
	}
}
```

アプリケーションが[公式Yiiアプリテンプレート](https://github.com/yiisoft/app)に基づいている場合、デバッグ、環境、エラーハンドラの構成を含む完全な`worker.php`の例については、[パッケージのREADME](https://github.com/yiisoft/yii-runner-frankenphp)を参照してください。

ワーカーが再起動されるまでに処理するリクエストの数を制限するには (メモリリークの軽減に役立ちます)、`MAX_REQUESTS`環境変数を設定してください。デフォルトでは、ワーカーは無期限にリクエストを処理します。

ワーカーモードを使用する場合、ステートフルなサービスが各リクエスト後にリセットされることを確認してください。詳細については、[ワーカーモードのドキュメント](worker.md)と[Yii DIの`StateResetter`ドキュメント](https://github.com/yiisoft/di#resetting-services-state)を参照してください。
