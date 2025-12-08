# Laravel

## Docker

FrankenPHPを使用して[Laravel](https://laravel.com)のWebアプリケーションを配信するのは簡単で、公式Dockerイメージの`/app`ディレクトリにプロジェクトをマウントするだけです。

Laravelアプリのメインディレクトリからこのコマンドを実行してください：

```console
docker run -p 80:80 -p 443:443 -p 443:443/udp -v $PWD:/app dunglas/frankenphp
```

お楽しみください！

## ローカルインストール

または、ローカルマシンでFrankenPHPを使用してLaravelプロジェクトを実行することもできます：

1. [使用しているシステムに対応するバイナリをダウンロードします](../#standalone-binary)
2. Laravelプロジェクトのルートディレクトリに`Caddyfile`という名前のファイルを作成し、以下の設定を追加します：

   ```caddyfile
   {
   	frankenphp
   }

   # サーバーのドメイン名
   localhost {
   	# webroot を public/ ディレクトリに設定
   	root public/
   	# 圧縮を有効にする（任意）
   	encode zstd br gzip
   	# public/ ディレクトリ内の PHP ファイルを実行し、アセットを提供
   	php_server {
   		try_files {path} index.php
   	}
   }
   ```

3. LaravelプロジェクトのルートディレクトリからFrankenPHPを起動します： `frankenphp run`

## Laravel Octane

OctaneはComposerパッケージマネージャーを使用してインストールできます：

```console
composer require laravel/octane
```

Octaneをインストールした後、`octane:install` Artisanコマンドを実行すると、Octaneの設定ファイルがアプリケーションにインストールされます：

```console
php artisan octane:install --server=frankenphp
```

Octaneサーバーは`octane:frankenphp` Artisanコマンドで開始できます。

```console
php artisan octane:frankenphp
```

`octane:frankenphp`コマンドは以下のオプションが利用可能です：

- `--host`: サーバーがバインドするIPアドレス（デフォルト：`127.0.0.1`）
- `--port`: サーバーが使用するポート（デフォルト： `8000`）
- `--admin-port`: 管理サーバーが使用するポート（デフォルト： `2019`）
- `--workers`: リクエスト処理に使うワーカー数（デフォルト： `auto`）
- `--max-requests`: サーバーを再起動するまでに処理するリクエスト数（デフォルト： `500`）
- `--caddyfile`: FrankenPHPの`Caddyfile`ファイルのパス（デフォルト： [Laravel OctaneのスタブCaddyfile](https://github.com/laravel/octane/blob/2.x/src/Commands/stubs/Caddyfile)）
- `--https`: HTTPS、HTTP/2、HTTP/3を有効にし、証明書を自動的に生成・更新する
- `--http-redirect`: HTTPからHTTPSへのリダイレクトを有効にする（--httpsオプション指定時のみ有効）
- `--watch`: アプリケーションが変更されたときに自動的にサーバーをリロードする
- `--poll`: ネットワーク越しのファイル監視のためにファイルシステムポーリングを使用する
- `--log-level`: ネイティブCaddyロガーを使用して、指定されたログレベル以上でログメッセージを記録する

> [!TIP]
> 構造化されたJSONログ（ログ分析ソリューションを使用する際に便利）を取得するには、明示的に`--log-level`オプションを指定してください。

詳しくは[Laravel Octaneの公式ドキュメント](https://laravel.com/docs/octane)をご覧ください。

## Laravelアプリのスタンドアロンバイナリ化

[FrankenPHPのアプリケーション埋め込み機能](embed.md)を使用して、Laravelアプリをスタンドアロンバイナリとして
配布することが可能です。

LaravelアプリをLinux用のスタンドアロンバイナリとしてパッケージ化するには、以下の手順に従ってください：

1. アプリのリポジトリに`static-build.Dockerfile`という名前のファイルを作成します：

   ```dockerfile
   FROM --platform=linux/amd64 dunglas/frankenphp:static-builder-gnu
   # バイナリをmusl-libcシステムで実行する場合は、static-builder-musl を使用してください

   # アプリをコピー
   WORKDIR /go/src/app/dist/app
   COPY . .

   # スペースを節約するためにテストやその他の不要なファイルを削除
   # 代わりに .dockerignore に記述して除外することも可能
   RUN rm -Rf tests/

   # .envファイルをコピー
   RUN cp .env.example .env
   # APP_ENV と APP_DEBUG を本番用に変更
   RUN sed -i'' -e 's/^APP_ENV=.*/APP_ENV=production/' -e 's/^APP_DEBUG=.*/APP_DEBUG=false/' .env

   # 必要に応じて .env ファイルにさらに変更を加える

   # 依存関係をインストール
   RUN composer install --ignore-platform-reqs --no-dev -a

   # 静的バイナリをビルド
   WORKDIR /go/src/app/
   RUN EMBED=/go/src/app/dist/app/ ./build-static.sh
   ```

   > [!CAUTION]
   >
   > 一部の`.dockerignore`ファイルは
   > `vendor/`ディレクトリや`.env`ファイルを無視します。ビルド前に`.dockerignore`ファイルを調整または削除してください。

2. ビルドします：

   ```console
   docker build -t static-laravel-app -f static-build.Dockerfile .
   ```

3. バイナリを取り出します：

   ```console
   docker cp $(docker create --name static-laravel-app-tmp static-laravel-app):/go/src/app/dist/frankenphp-linux-x86_64 frankenphp ; docker rm static-laravel-app-tmp
   ```

4. キャッシュを構築します：

   ```console
   frankenphp php-cli artisan optimize
   ```

5. データベースマイグレーションを実行します（ある場合）：

   ```console
   frankenphp php-cli artisan migrate
   ```

6. アプリの秘密鍵を生成します：

   ```console
   frankenphp php-cli artisan key:generate
   ```

7. サーバーを起動します：

   ```console
   frankenphp php-server
   ```

これで、アプリの準備は完了です！

利用可能なオプションや他のOSでバイナリをビルドする方法については、[アプリケーション埋め込み](embed.md)ドキュメントをご覧ください。

### ストレージパスの変更

Laravelはアップロードされたファイルやキャッシュ、ログなどをデフォルトでアプリケーションの`storage/`ディレクトリに保存します。
しかし、これは埋め込みアプリケーションには適していません。なぜなら、アプリの新しいバージョンごとに異なる一時ディレクトリに展開されるためです。

この問題を回避するには、`LARAVEL_STORAGE_PATH`環境変数を設定（例：`.env`ファイル内）するか、 `Illuminate\Foundation\Application::useStoragePath()`メソッドを呼び出して、一時ディレクトリの外にある任意のディレクトリを使用してください。

### スタンドアロンバイナリでOctaneを実行する

Laravel Octaneアプリもスタンドアロンバイナリとしてパッケージ化することが可能です！

そのためには、[Octaneを正しくインストール](#laravel-octane)し、[前のセクション](#laravelアプリのスタンドアロンバイナリ化)で説明した手順に従ってください。

次に、Octaneを通じてワーカーモードでFrankenPHPを起動するには、以下を実行してください：

```console
PATH="$PWD:$PATH" frankenphp php-cli artisan octane:frankenphp
```

> [!CAUTION]
>
> コマンドを動作させるためには、スタンドアロンバイナリのファイル名が**必ず**`frankenphp`でなければなりません。
> Octaneは`frankenphp`という名前の実行ファイルがパス上に存在することを前提としています。
