---
title: FrankenPHP static build: single-binary PHP app server
description: Create static or mostly static FrankenPHP binaries with embedded PHP and Caddy on Linux and macOS, customize extensions, and load dynamic modules with glibc.
---

# 静的ビルドの作成

PHPライブラリのローカルインストールを使用する代わりに、
素晴らしい[static-php-cli プロジェクト](https://github.com/crazywhalecc/static-php-cli)のおかげで、FrankenPHPの静的またはほぼ静的なビルドを作成することが可能です（プロジェクト名に「CLI」とありますが、CLIだけでなく全てのSAPIをサポートしています）。

この方法を使えば、PHPインタープリター、Caddy Webサーバー、FrankenPHPをすべて含んだ単一でポータブルなバイナリを作成できます！

完全に静的なネイティブ実行ファイルは依存関係を全く必要とせず、[`scratch` Dockerイメージ](https://docs.docker.com/build/building/base-images/#create-a-minimal-base-image-using-scratch)上でも実行可能です。
ただし、動的PHP拡張モジュール（Xdebugなど）をロードできず、musl libcを使用しているため、いくつかの制限があります。

ほぼ静的なバイナリは`glibc`のみを必要とし、動的拡張モジュールをロードできます。

可能であれば、glibcベースのほぼ静的ビルドの使用をお勧めします。

また、FrankenPHPは[静的バイナリへのPHPアプリの埋め込み](embed.md)もサポートしています。

## Linux

静的なLinuxバイナリをビルドするためのDockerイメージを提供しています：

### muslベースの完全静的ビルド

依存関係なしにあらゆるLinuxディストリビューションで動作する完全静的バイナリ（ただし拡張モジュールの動的ロードはサポートしない）を作成するには、以下を実行します：

```console
docker buildx bake --load static-builder-musl
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log dunglas/frankenphp:static-builder-musl
cp "output/frankenphp-linux-$(uname -m)" frankenphp
```

高い並行性が求められるシナリオでは、より良いパフォーマンスのため、[mimalloc](https://github.com/microsoft/mimalloc)アロケーターの使用を検討してください。

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log -e MIMALLOC=1 dunglas/frankenphp:static-builder-musl
```

### glibcベースのほぼ静的なビルド（動的拡張モジュールのサポートあり）

選択した拡張モジュールを静的にコンパイルしながら、さらにPHP拡張モジュールを動的にロードできるバイナリを作成するには、以下を実行します：

```console
docker buildx bake --load static-builder-gnu
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log dunglas/frankenphp:static-builder-gnu
cp "output/frankenphp-linux-$(uname -m)" frankenphp
```

このバイナリはglibcバージョン2.28以上をすべてサポートしますが、muslベースシステム（Alpine Linuxなど）では動作しません。

生成されたほぼ静的（`glibc`を除く）バイナリは`frankenphp`という名前で、カレントディレクトリに出力されます。

ビルドが失敗した場合、static-php-cliのログは`output/log/`にあります。

Dockerを使わずに静的バイナリをビルドしたい場合は、macOS向けの手順を参照してください。これらの手順はLinuxでも使用できます。

### 静的ビルドでのカスタムPHP拡張モジュール

デフォルトでは、よく使われるPHP拡張モジュールがコンパイルされます。デフォルトの設定は[`craft.yml`](https://github.com/php/frankenphp/blob/main/craft.yml)にあります。

バイナリのサイズを削減したり、攻撃対象領域（アタックサーフェス）を減らすために、`PHP_EXTENSIONS`変数を使用してビルドする拡張モジュールを指定できます。

例えば、`opcache`と`pdo_sqlite`拡張モジュールのみをビルドするには、以下のように実行します：

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log -e PHP_EXTENSIONS=opcache,pdo_sqlite dunglas/frankenphp:static-builder-musl
# ...
```

有効にした拡張モジュールに追加機能を持たせるライブラリを追加するには、`PHP_EXTENSION_LIBS`変数を渡すことができます：

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log \
  -e PHP_EXTENSIONS=gd \
  -e PHP_EXTENSION_LIBS=libjpeg,libwebp \
  dunglas/frankenphp:static-builder-musl
```

### 追加のCaddyモジュール

追加のCaddyモジュールを追加したり、[xcaddy](https://github.com/caddyserver/xcaddy)に他の引数を渡したりするには、`XCADDY_ARGS`変数を使用します：

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log \
  -e XCADDY_ARGS="--with github.com/darkweak/souin/plugins/caddy --with github.com/dunglas/caddy-cbrotli --with github.com/dunglas/mercure/caddy --with github.com/dunglas/vulcain/caddy" \
  dunglas/frankenphp:static-builder-musl
```

この例では、Caddy用の[Souin](https://souin.io)HTTPキャッシュモジュールと[cbrotli](https://github.com/dunglas/caddy-cbrotli)、[Mercure](https://mercure.rocks)、[Vulcain](https://vulcain.rocks)モジュールを追加しています。

> [!TIP]
>
> cbrotli、Mercure、Vulcainモジュールは、`XCADDY_ARGS`が空または設定されていない場合はデフォルトで含まれます。
> `XCADDY_ARGS`の値をカスタマイズする場合、それらを含めたいのであれば明示的に指定する必要があります。

[FrankenPHP静的ビルドのカスタマイズ](#frankenphp静的ビルドのカスタマイズ)も参照してください

### GitHubトークン

GitHub API レート制限に達した場合は、`GITHUB_TOKEN`という名前の環境変数にGitHub Personal Access Tokenを設定してください：

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log -e GITHUB_TOKEN="xxx" dunglas/frankenphp:static-builder-musl
# ...
```

## macOS

macOS用の静的バイナリを作成するには以下のスクリプトを実行してください（[Homebrew](https://brew.sh/)がインストールされている必要があります）：

```console
git clone https://github.com/php/frankenphp
cd frankenphp
./build-static.sh
```

なお、このスクリプトはLinux（おそらく他のUnix系OS）でも動作し、私たちが提供するDockerイメージ内部でも使用されています。

Apple Silicon向けにのみmacOSバイナリを公開しています。Intel Macでは、このスクリプトを使用してソースからビルドしてください。

## FrankenPHP静的ビルドのカスタマイズ

以下の環境変数を`docker run`や`build-static.sh`
スクリプトに渡すことで、静的ビルドをカスタマイズできます：

- `FRANKENPHP_VERSION`: 使用するFrankenPHPのバージョン
- `PHP_VERSION`: 使用するPHPのバージョン
- `PHP_EXTENSIONS`: ビルドするPHP拡張（[サポートされる拡張のリスト](https://static-php.dev/en/guide/extensions.html)）
- `PHP_EXTENSION_LIBS`: 拡張モジュールに追加機能を持たせるためにビルドする追加ライブラリ
- `XCADDY_ARGS`: 追加のCaddyモジュールを導入するなど[xcaddy](https://github.com/caddyserver/xcaddy)に渡す引数
- `EMBED`: バイナリに埋め込むPHPアプリケーションのパス
- `CLEAN`: 指定するとlibphpおよびそのすべての依存関係がスクラッチからビルドされます（キャッシュなし）
- `COMPRESS`: `1` に設定すると UPX を使用して結果のバイナリを圧縮します（Linux のみ。`DEBUG_SYMBOLS` が設定されている場合は無視されます）
- `DEBUG_SYMBOLS`: 指定すると、デバッグシンボルが除去されず、バイナリに含まれます
- `MIMALLOC`: （実験的、Linuxのみ）パフォーマンス向上のためにmuslのmallocngを[mimalloc](https://github.com/microsoft/mimalloc)に置き換えます。muslをターゲットとするビルドにのみこれを使用することをお勧めします。glibcの場合は、このオプションを無効にして、代わりにバイナリを実行する際に[`LD_PRELOAD`](https://microsoft.github.io/mimalloc/overrides.html)を使用することをお勧めします。
- `RELEASE`: （メンテナー用）指定すると、生成されたバイナリがGitHubにアップロードされます
- `LIBC`: (Linuxのみ) 完全静的バイナリの場合は`musl`（デフォルト）、システムglibcに動的にリンクする場合は`gnu`。これはstatic-php-cliの`SPC_TARGET`および`SPC_TOOLCHAIN`の短縮形であり、これらは直接設定することもできます。static-php-cliは`craft.yml`を読み込む前にツールチェーンを解決するため、これらは実際の環境変数である必要があります。`musl`のデフォルトはアーキテクチャのベースラインCPU（`x86_64-linux-musl`または`aarch64-linux-musl`）をターゲットとするため、バイナリはどのCPUでも実行できます。代わりにビルドマシン用に最適化するには`SPC_TARGET=native-native-musl`を設定してください。

## 静的バイナリでPHP拡張モジュールを動的にロードする

glibcまたはmacOSベースのバイナリでは、PHP拡張モジュールを動的にロードできます。ただし、これらの拡張はZTSサポートでコンパイルされている必要があります。
ほとんどのパッケージマネージャーは現在、拡張のZTSバージョンを提供していないため、自分でコンパイルする必要があります。

このために、`static-builder-gnu` Dockerコンテナを実行し、それにリモートで接続して、`./configure --with-php-config=/go/src/app/dist/buildroot/bin/php-config`で拡張をコンパイルできます。

[Xdebug拡張モジュール](https://xdebug.org)の場合：

```console
docker buildx bake --load static-builder-gnu
docker run --rm -v "$PWD/dist:/go/src/app/dist" -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log dunglas/frankenphp:static-builder-gnu
docker run --rm -it -v "$PWD/dist:/go/src/app/dist" dunglas/frankenphp:static-builder-gnu /bin/bash
cd /go/src/app/dist/buildroot/bin
git clone https://github.com/xdebug/xdebug.git && cd xdebug
../phpize
./configure --with-php-config=/go/src/app/dist/buildroot/bin/php-config
make
exit
cp dist/buildroot/bin/xdebug/modules/xdebug.so xdebug-zts.so
cp "output/frankenphp-linux-$(uname -m)" frankenphp
```

これにより、現在のディレクトリに`frankenphp`と`xdebug-zts.so`が作成されます。
もし`xdebug-zts.so`を拡張ディレクトリに移動し、php.iniに`zend_extension=xdebug-zts.so`を追加してFrankenPHPを実行すると、Xdebugがロードされます。
