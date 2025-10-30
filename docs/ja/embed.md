# PHPアプリのスタンドアロンバイナリ化

FrankenPHPには、PHPアプリケーションのソースコードやアセットを静的な自己完結型バイナリに埋め込む機能があります。

この機能により、PHPアプリケーション自体に加えて、PHPインタープリターや本番環境対応のWebサーバーCaddyも含んだスタンドアロンバイナリとして配布できます。

この機能について詳しくは、[SymfonyCon 2023でKévinが行ったプレゼンテーション](https://dunglas.dev/2023/12/php-and-symfony-apps-as-standalone-binaries/)をご覧ください。

Laravelアプリケーションの埋め込みについては、[こちらの専用ドキュメント](laravel.md#laravel-apps-as-standalone-binaries)をお読みください。

## アプリの準備

自己完結型バイナリを作成する前に、アプリが埋め込みに対応できる状態にあることを確認してください。

例えば、以下のような作業が必要です：

- 本番環境用の依存パッケージをインストールする
- オートローダーをダンプする
- アプリケーションの本番モードを有効にする（ある場合）
- 最終バイナリのサイズを減らすために`.git`やテストなどの不要なファイルを除外する

例えば、Symfonyアプリの場合、以下のコマンドを使用できます：

```console
# .git/ などを除去するためにプロジェクトをエクスポート
mkdir $TMPDIR/my-prepared-app
git archive HEAD | tar -x -C $TMPDIR/my-prepared-app
cd $TMPDIR/my-prepared-app

# 適切な環境変数を設定
echo APP_ENV=prod > .env.local
echo APP_DEBUG=0 >> .env.local

# テストやその他不要ファイルを削除して容量削減
# あるいは、 .gitattributes の export-ignore 属性にこれらを追加してもよい
rm -Rf tests/

# 依存パッケージをインストール
composer install --ignore-platform-reqs --no-dev -a

# .env を最適化
composer dump-env prod
```

### 設定のカスタマイズ

[設定](config.md) をカスタマイズするには、埋め込まれるアプリのメインディレクトリ
（前の例では`$TMPDIR/my-prepared-app`）に`Caddyfile`と`php.ini`ファイルを配置できます。

## Linux用バイナリの作成

Linux用バイナリを作成する最も簡単な方法は、提供されているDockerベースのビルダーを使用することです。

1. アプリのリポジトリに`static-build.Dockerfile`というファイルを作成します：

   ```dockerfile
   FROM dunglas/frankenphp:static-builder-gnu
   # バイナリをmusl-libcシステムで実行する場合は、static-builder-musl を使用してください

   # アプリをコピー
   WORKDIR /go/src/app/dist/app
   COPY . .

   # 静的バイナリをビルド
   WORKDIR /go/src/app/
   RUN EMBED=dist/app/ ./build-static.sh
   ```

   > [!CAUTION]
   >
   > 一部の`.dockerignore`ファイル（例：デフォルトの[Symfony Docker `.dockerignore`](https://github.com/dunglas/symfony-docker/blob/main/.dockerignore)）
   > は`vendor/`ディレクトリと`.env`ファイルを無視します。ビルド前に`.dockerignore`ファイルを調整または削除してください。

2. ビルドします：

   ```console
   docker build -t static-app -f static-build.Dockerfile .
   ```

3. バイナリを抽出します：

   ```console
   docker cp $(docker create --name static-app-tmp static-app):/go/src/app/dist/frankenphp-linux-x86_64 my-app ; docker rm static-app-tmp
   ```

生成されるバイナリは、現在のディレクトリの`my-app`というファイル名になります。

## 他のOS用のバイナリの作成

Dockerを使用したくない場合や、macOSバイナリを作成したい場合は、提供されているシェルスクリプトを使用してください：

```console
git clone https://github.com/php/frankenphp
cd frankenphp
EMBED=/path/to/your/app ./build-static.sh
```

生成されるバイナリは、`dist/`ディレクトリの`frankenphp-<os>-<arch>`という名前のファイルです。

## バイナリの使い方

これで完了です！`my-app`ファイル（または他のOSでは`dist/frankenphp-<os>-<arch>`）には、自己完結型アプリが含まれています！

Webアプリを起動するには、以下を実行します：

```console
./my-app php-server
```

アプリに[ワーカースクリプト](worker.md)が含まれている場合は、以下のようにワーカーを開始します：

```console
./my-app php-server --worker public/index.php
```

HTTPS（Let's Encrypt証明書は自動作成）、HTTP/2、HTTP/3を有効にするには、使用するドメイン名を指定してください：

```console
./my-app php-server --domain localhost
```

バイナリに埋め込まれたPHP CLIスクリプトも実行できます：

```console
./my-app php-cli bin/console
```

## PHP拡張モジュール

デフォルトでは、スクリプトはプロジェクトの`composer.json`ファイルで必要な拡張モジュールをビルドします（存在する場合）。
`composer.json`ファイルが存在しない場合、[静的ビルドのドキュメント](static.md)に記載されているデフォルトの拡張モジュールがビルドされます。

拡張モジュールをカスタマイズしたい場合は、`PHP_EXTENSIONS`環境変数を使用してください。

## ビルドのカスタマイズ

バイナリをカスタマイズする方法（拡張モジュール、PHPバージョンなど）については、[静的ビルドのドキュメント](static.md)をお読みください。

## バイナリの配布

Linuxでは、作成されたバイナリは[UPX](https://upx.github.io)を使用して圧縮されます。

Macでは、送信前にファイルサイズを減らすために圧縮できます。
`xz`の使用をお勧めします。
