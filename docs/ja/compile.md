# ソースからのコンパイル

このドキュメントでは、PHPを動的ライブラリとしてロードするFrankenPHPバイナリの作成方法を説明します。
これが推奨される方法です。

または、[完全静的およびほぼ静的なビルド](static.md)も作成できます。

## PHPのインストール

FrankenPHPはPHP 8.2以上と互換性があります。

### Homebrewを使用する場合（LinuxとMac）

FrankenPHPと互換性のあるlibphpのバージョンをインストールする最も簡単な方法は、[Homebrew PHP](https://github.com/shivammathur/homebrew-php)が提供するZTSパッケージを使用することです。

まず、まだインストールしていない場合は[Homebrew](https://brew.sh)をインストールしてください。

次に、PHPのZTSバリアント、Brotli（オプション、圧縮サポート用）、およびwatcher（オプション、ファイル変更検出用）をインストールします：

```console
brew install shivammathur/php/php-zts brotli watcher
brew link --overwrite --force shivammathur/php/php-zts
```

### PHPをコンパイルする場合

別の方法として、FrankenPHPに必要なオプションを指定してPHPをソースからコンパイルすることもできます。以下の手順に従ってください。

まず、[PHPのソース](https://www.php.net/downloads.php)を取得して展開します：

```console
tar xf php-*
cd php-*/
```

次に、プラットフォームに応じて必要なオプションを指定して`configure`スクリプトを実行します。
以下の`./configure`フラグは必須ですが、例えば拡張機能や追加機能をコンパイルするために他のフラグを追加することもできます。

#### Linux

```console
./configure \
    --enable-embed \
    --enable-zts \
    --disable-zend-signals \
    --enable-zend-max-execution-timers
```

#### Mac

[Homebrew](https://brew.sh/)パッケージマネージャーを使用して、必須およびオプションの依存関係をインストールします：

```console
brew install libiconv bison brotli re2c pkg-config watcher
echo 'export PATH="/opt/homebrew/opt/bison/bin:$PATH"' >> ~/.zshrc
```

その後、`configure`スクリプトを実行します：

```console
./configure \
    --enable-embed \
    --enable-zts \
    --disable-zend-signals \
    --with-iconv=/opt/homebrew/opt/libiconv/
```

#### PHPのコンパイル

最後に、PHPをコンパイルしてインストールします：

```console
make -j"$(getconf _NPROCESSORS_ONLN)"
sudo make install
```

## オプション依存関係のインストール

FrankenPHPの一部の機能は、システムにインストールされているオプションの依存関係に依存しています。
または、Goコンパイラにビルドタグを渡すことで、これらの機能を無効にできます。

| 機能                           | 依存関係                                                                                                   | 無効にするためのビルドタグ |
| :----------------------------- | :----------------------------------------------------------------------------------------------------------- | :------------------------- |
| Brotli圧縮                     | [Brotli](https://github.com/google/brotli)                                                                   | `nobrotli`                 |
| ファイル変更時のワーカー再起動 | [Watcher C](https://github.com/e-dant/watcher/tree/release/watcher-c)                                        | `nowatcher`                |
| [Mercure](mercure.md)          | [Mercure Goライブラリ](https://pkg.go.dev/github.com/dunglas/mercure)（自動インストール、AGPLライセンス） | `nomercure`                |

## Goアプリのコンパイル

これで最終的なバイナリをビルドできます。

### `xcaddy`を使う場合

推奨される方法は、[xcaddy](https://github.com/caddyserver/xcaddy)を使用してFrankenPHPをコンパイルする方法です。
`xcaddy`を使うと、[Caddyのカスタムモジュール](https://caddyserver.com/docs/modules/)やFrankenPHP拡張を簡単に追加できます：

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/dunglas/frankenphp/caddy \
    --with github.com/dunglas/mercure/caddy \
    --with github.com/dunglas/vulcain/caddy \
    --with github.com/dunglas/caddy-cbrotli
    # 追加のCaddyモジュールとFrankenPHP拡張をここに追加
    # オプションで、FrankenPHPのソースからコンパイルしたい場合：
    # --with github.com/dunglas/frankenphp=$(pwd) \
    # --with github.com/dunglas/frankenphp/caddy=$(pwd)/caddy

```

> [!TIP]
>
> `musl libc`（Alpine Linuxのデフォルト）とSymfonyを使用している場合、
> デフォルトのスタックサイズを増やす必要がある場合があります。
> そうしないと、`PHP Fatal error: Maximum call stack size of 83360 bytes reached during compilation. Try splitting expression`のようなエラーが発生する可能性があります。
>
> これを行うには、`XCADDY_GO_BUILD_FLAGS`環境変数を
> `XCADDY_GO_BUILD_FLAGS=$'-ldflags "-w -s -extldflags \'-Wl,-z,stack-size=0x80000\'"'`のようなものに変更してください
> （アプリの要件に応じてスタックサイズの値を変更してください）。

### `xcaddy`を使用しない場合

代替として、`xcaddy`を使わずに`go`コマンドを直接使ってFrankenPHPをコンパイルすることも可能です：

```console
curl -L https://github.com/php/frankenphp/archive/refs/heads/main.tar.gz | tar xz
cd frankenphp-main/caddy/frankenphp
CGO_CFLAGS=$(php-config --includes) CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" go build -tags=nobadger,nomysql,nopgx
```
