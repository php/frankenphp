# 静的ビルドの作成

PHPライブラリのローカルインストールを使用する代わりに、
[static-php-cli プロジェクト](https://github.com/crazywhalecc/static-php-cli)を利用して、FrankenPHPの静的またはほぼ静的なビルドを作成することが可能です（プロジェクト名に「CLI」とありますが、CLIだけでなく全てのSAPIをサポートしています）。

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
docker cp $(docker create --name static-builder-musl dunglas/frankenphp:static-builder-musl):/go/src/app/dist/frankenphp-linux-$(uname -m) frankenphp ; docker rm static-builder-musl
```

高い並行性が求められるシナリオでは、より良いパフォーマンスのため、[mimalloc](https://github.com/microsoft/mimalloc)アロケーターの使用を検討してください。

```console
docker buildx bake --load --set static-builder-musl.args.MIMALLOC=1 static-builder-musl
```

### glibcベースのほぼ静的なビルド（動的拡張モジュールのサポートあり）

選択した拡張モジュールを静的にコンパイルしながら、さらにPHP拡張モジュールを動的にロードできるバイナリを作成するには、以下を実行します：

```console
docker buildx bake --load static-builder-gnu
docker cp $(docker create --name static-builder-gnu dunglas/frankenphp:static-builder-gnu):/go/src/app/dist/frankenphp-linux-$(uname -m) frankenphp ; docker rm static-builder-gnu
```

このバイナリは、glibcバージョン2.17以上をすべてサポートしますが、muslベースシステム（Alpine Linuxなど）では動作しません。

生成されたほぼ静的（`glibc`を除く）バイナリは`frankenphp`という名前で、カレントディレクトリに出力されます。

Dockerを使わずに静的バイナリをビルドしたい場合は、macOS向けの手順を参照してください。これらの手順はLinuxでも使用できます。

### カスタム拡張モジュール

デフォルトでは、よく使われるPHP拡張モジュールがコンパイルされます。

バイナリのサイズを削減したり、攻撃対象領域（アタックサーフェス）を減らすために、`PHP_EXTENSIONS`というDocker引数を使用してビルドする拡張モジュールを明示的に指定できます。

例えば、`opcache`と`pdo_sqlite`拡張モジュールのみをビルドするには、以下のように実行します：

```console
docker buildx bake --load --set static-builder-musl.args.PHP_EXTENSIONS=opcache,pdo_sqlite static-builder-musl
# ...
```

有効にした拡張に必要なライブラリを追加するには、`PHP_EXTENSION_LIBS`というDocker引数を渡すことができます：

```console
docker buildx bake \
  --load \
  --set static-builder-musl.args.PHP_EXTENSIONS=gd \
  --set static-builder-musl.args.PHP_EXTENSION_LIBS=libjpeg,libwebp \
  static-builder-musl
```

### 追加のCaddyモジュール

Caddyの拡張モジュールを追加したい場合は、`XCADDY_ARGS`というDocker引数を使用して、[xcaddy](https://github.com/caddyserver/xcaddy)に渡す引数を以下のように指定できます：

```console
docker buildx bake \
  --load \
  --set static-builder-musl.args.XCADDY_ARGS="--with github.com/darkweak/souin/plugins/caddy --with github.com/dunglas/caddy-cbrotli --with github.com/dunglas/mercure/caddy --with github.com/dunglas/vulcain/caddy" \
  static-builder-musl
```

この例では、Caddy用の[Souin](https://souin.io)HTTPキャッシュモジュールと[cbrotli](https://github.com/dunglas/caddy-cbrotli)、[Mercure](https://mercure.rocks)、[Vulcain](https://vulcain.rocks)モジュールを追加しています。

> [!TIP]
>
> cbrotli、Mercure、Vulcainモジュールは、`XCADDY_ARGS`が空または設定されていない場合はデフォルトで含まれます。
> `XCADDY_ARGS`の値をカスタマイズする場合、デフォルトのモジュールは含まれなくなるため、必要なものは明示的に記述してください。

[ビルドのカスタマイズ](#ビルドのカスタマイズ)も参照してください

### GitHubトークン

GitHub API レート制限に達した場合は、`GITHUB_TOKEN`という名前の環境変数にGitHub Personal Access Tokenを設定してください：

```console
GITHUB_TOKEN="xxx" docker --load buildx bake static-builder-musl
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

## ビルドのカスタマイズ

以下の環境変数を`docker build`や`build-static.sh`
スクリプトに渡すことで、静的ビルドをカスタマイズできます：

- `FRANKENPHP_VERSION`: 使用するFrankenPHPのバージョン
- `PHP_VERSION`: 使用するPHPのバージョン
- `PHP_EXTENSIONS`: ビルドするPHP拡張（[サポートされる拡張のリスト](https://static-php.dev/en/guide/extensions.html)）
- `PHP_EXTENSION_LIBS`: 拡張モジュールに追加機能を持たせるためにビルドする追加ライブラリ
- `XCADDY_ARGS`: 追加のCaddyモジュールを導入するなど[xcaddy](https://github.com/caddyserver/xcaddy)に渡す引数
- `EMBED`: バイナリに埋め込むPHPアプリケーションのパス
- `CLEAN`: 指定するとlibphpおよびそのすべての依存関係がスクラッチからビルドされます（キャッシュなし）
- `NO_COMPRESS`: UPXを使用して結果のバイナリを圧縮しない
- `DEBUG_SYMBOLS`: 指定すると、デバッグシンボルが除去されず、バイナリに含まれます
- `MIMALLOC`: （実験的、Linuxのみ）パフォーマンス向上のためにmuslのmallocngを[mimalloc](https://github.com/microsoft/mimalloc)に置き換えます。muslをターゲットとするビルドにのみこれを使用することをお勧めします。glibcの場合は、このオプションを無効にして、代わりにバイナリを実行する際に[`LD_PRELOAD`](https://microsoft.github.io/mimalloc/overrides.html)を使用することをお勧めします。
- `RELEASE`: （メンテナー用）指定すると、生成されたバイナリがGitHubにアップロードされます

## 拡張モジュール

glibcまたはmacOSベースのバイナリでは、PHP拡張モジュールを動的にロードできます。ただし、これらの拡張はZTSサポートでコンパイルされている必要があります。
ほとんどのパッケージマネージャーは現在、拡張のZTSバージョンを提供していないため、自分でコンパイルする必要があります。

このために、`static-builder-gnu`Dockerコンテナをビルドして実行し、リモートでアクセスし、`./configure --with-php-config=/go/src/app/dist/static-php-cli/buildroot/bin/php-config`で拡張をコンパイルできます。

[Xdebug拡張モジュール](https://xdebug.org)の場合：

```console
docker build -t gnu-ext -f static-builder-gnu.Dockerfile --build-arg FRANKENPHP_VERSION=1.0 .
docker create --name static-builder-gnu -it gnu-ext /bin/sh
docker start static-builder-gnu
docker exec -it static-builder-gnu /bin/sh
cd /go/src/app/dist/static-php-cli/buildroot/bin
git clone https://github.com/xdebug/xdebug.git && cd xdebug
source scl_source enable devtoolset-10
../phpize
./configure --with-php-config=/go/src/app/dist/static-php-cli/buildroot/bin/php-config
make
exit
docker cp static-builder-gnu:/go/src/app/dist/static-php-cli/buildroot/bin/xdebug/modules/xdebug.so xdebug-zts.so
docker cp static-builder-gnu:/go/src/app/dist/frankenphp-linux-$(uname -m) ./frankenphp
docker stop static-builder-gnu
docker rm static-builder-gnu
docker rmi gnu-ext
```

これにより、現在のディレクトリに`frankenphp`と`xdebug-zts.so`が作成されます。
`xdebug-zts.so`を拡張ディレクトリに移動し、php.iniに`zend_extension=xdebug-zts.so`を追加してFrankenPHPを実行すると、Xdebugがロードされます。
