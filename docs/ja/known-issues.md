# 既知の問題

## 未対応のPHP拡張モジュール

以下の拡張モジュールはFrankenPHPと互換性がないことが確認されています：

| 名前                                                                                                        | 理由                 | 代替手段                                                                                                             |
| ----------------------------------------------------------------------------------------------------------- | -------------------- | -------------------------------------------------------------------------------------------------------------------- |
| [imap](https://www.php.net/manual/en/imap.installation.php)                                                 | スレッドセーフでない | [javanile/php-imap2](https://github.com/javanile/php-imap2), [webklex/php-imap](https://github.com/Webklex/php-imap) |
| [newrelic](https://docs.newrelic.com/docs/apm/agents/php-agent/getting-started/introduction-new-relic-php/) | スレッドセーフでない | -                                                                                                                    |

## バグのあるPHP拡張モジュール

以下の拡張モジュールはFrankenPHPとの組み合わせで既知のバグや予期しない動作が確認されています：

| 名前                                                          | 問題                                                                                                                                                                                                                   |
| ------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [ext-openssl](https://www.php.net/manual/en/book.openssl.php) | musl libcを使用している場合、OpenSSL拡張モジュールは高負荷時にクラッシュすることがあります。この問題は、より普及しているGNU libcを使用する場合には発生しません。このバグは[PHPによって追跡されています](https://github.com/php/php-src/issues/13648)。 |

## get_browser

[get_browser()](https://www.php.net/manual/en/function.get-browser.php)関数は継続使用するとパフォーマンスが悪化することが確認されています。回避策として、User Agentごとの結果をキャッシュ（例：[APCu](https://www.php.net/manual/en/book.apcu.php)を利用）してください。User Agentごとの結果は静的なためです。

## スタンドアロンバイナリおよびAlpineベースのDockerイメージ

スタンドアロンバイナリおよびAlpineベースのDockerイメージ（`dunglas/frankenphp:*-alpine`）は、バイナリサイズを小さく保つために[glibc and friends](https://www.etalabs.net/compare_libcs.html)ではなく[musl libc](https://musl.libc.org/)を使用しています。これによりいくつかの互換性問題が発生する可能性があります。特に、globフラグ`GLOB_BRACE`は [サポートされていません](https://www.php.net/manual/en/function.glob.php)。

問題が発生した場合は、静的バイナリのGNUバリアントおよびDebianベースのDockerイメージを使用することをお勧めします。

## Dockerで`https://127.0.0.1`を使用する

デフォルトでは、FrankenPHPは`localhost`用のTLS証明書を生成します。
これはローカル開発における最も簡単かつ推奨される方法です。

どうしても`127.0.0.1`をホストとして使用したい場合は、サーバー名を`127.0.0.1`に設定してその証明書を生成させることが可能です。

ただし、[Dockerのネットワークシステム](https://docs.docker.com/network/)の仕組みにより、Dockerを使用する場合はこれだけでは不十分です。
この場合、`curl: (35) LibreSSL/3.3.6: error:1404B438:SSL routines:ST_CONNECT:tlsv1 alert internal error`のようなTLSエラーが発生します。

Linuxを使用している場合、[ホストネットワークドライバー](https://docs.docker.com/network/network-tutorial-host/)を使用することで、この問題を解決できます：

```console
docker run \
    -e SERVER_NAME="127.0.0.1" \
    -v $PWD:/app/public \
    --network host \
    dunglas/frankenphp
```

ホストネットワークドライバーはMacとWindowsではサポートされていません。これらのプラットフォームでは、コンテナのIPアドレスを推測してサーバー名に含める必要があります。

`docker network inspect bridge`を実行し、`Containers`キーを確認して`IPv4Address`にある現在割り当てられている最後のIPアドレスを特定し、それに1を加えます。コンテナがまだ実行されていない場合、最初に割り当てられるIPアドレスは通常`172.17.0.2`です。

そして、これを`SERVER_NAME`環境変数に含めます：

```console
docker run \
    -e SERVER_NAME="127.0.0.1, 172.17.0.3" \
    -v $PWD:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

> [!CAUTION]
>
> `172.17.0.3`の部分は、実際にコンテナに割り当てられるIPに置き換えてください。

これでホストマシンから`https://127.0.0.1`へアクセスできるはずです。

うまくいかない場合は、FrankenPHPをデバッグモードで起動して問題を特定してみてください：

```console
docker run \
    -e CADDY_GLOBAL_OPTIONS="debug" \
    -e SERVER_NAME="127.0.0.1" \
    -v $PWD:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

## `@php` を参照するComposerスクリプト

[Composerスクリプト](https://getcomposer.org/doc/articles/scripts.md)では、いくつかのタスクでPHPバイナリを実行したい場合があります。例えば、[Laravelプロジェクト](laravel.md)で`@php artisan package:discover --ansi`を実行する場合です。しかし現在これは以下の2つの理由で[失敗します](https://github.com/php/frankenphp/issues/483#issuecomment-1899890915)：

- ComposerはFrankenPHPバイナリを呼び出す方法を知りません
- Composerはコマンドで`-d`フラグを使用してPHP設定を追加する場合があり、FrankenPHPはまだサポートしていません

回避策として、未サポートのパラメータを削除してFrankenPHPを呼び出すシェルスクリプトを`/usr/local/bin/php`に作成できます：

```bash
#!/usr/bin/env bash
args=("$@")
index=0
for i in "$@"
do
    if [ "$i" == "-d" ]; then
        unset 'args[$index]'
        unset 'args[$index+1]'
    fi
    index=$((index+1))
done

/usr/local/bin/frankenphp php-cli ${args[@]}
```

次に、環境変数`PHP_BINARY`にこの`php`スクリプトのパスを設定してComposerを実行します：

```console
export PHP_BINARY=/usr/local/bin/php
composer install
```

## 静的バイナリでのTLS/SSL問題のトラブルシューティング

静的バイナリを使用する場合、例えばSTARTTLSを使用してメールを送信する際に以下のTLS関連エラーが発生する可能性があります：

```text
Unable to connect with STARTTLS: stream_socket_enable_crypto(): SSL operation failed with code 5. OpenSSL Error messages:
error:80000002:system library::No such file or directory
error:80000002:system library::No such file or directory
error:80000002:system library::No such file or directory
error:0A000086:SSL routines::certificate verify failed
```

静的バイナリにはTLS証明書がバンドルされていないため、OpenSSLにローカルのCA証明書の位置を明示する必要があります。

[`openssl_get_cert_locations()`](https://www.php.net/manual/en/function.openssl-get-cert-locations.php)の出力を調べて、
CA証明書をどこにインストールすべきか確認し、その場所に保存してください。

> [!WARNING]
>
> WebとCLIコンテキストでは設定が異なる場合があります。
> 適切なコンテキストで`openssl_get_cert_locations()`を実行してください。

[Mozillaから抽出されたCA証明書はcURLのサイトでダウンロードできます](https://curl.se/docs/caextract.html)。

または、Debian、Ubuntu、Alpineなどのディストリビューションでも、これらの証明書を含む`ca-certificates`というパッケージを提供しています。

`SSL_CERT_FILE`および`SSL_CERT_DIR`を使用してOpenSSLにCA証明書を探す場所をヒントとして与えることも可能です：

```console
# TLS 証明書の環境変数を設定
export SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
export SSL_CERT_DIR=/etc/ssl/certs
