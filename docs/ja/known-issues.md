---
title: FrankenPHPの既知の問題、非互換の拡張モジュール、および回避策
description: サポートされていないPHP拡張モジュール、musl libcの注意点、127.0.0.1を使用したDocker TLSセットアップ、Composer @phpスクリプトなど、FrankenPHPの制限事項の参照。
---

# 既知の問題

## 未対応のPHP拡張モジュール

以下の拡張モジュールはFrankenPHPと互換性がないことが確認されています：

| 名前                                                                                                        | 理由                 | 代替手段                                                                                                                                                                    |
| :---------------------------------------------------------------------------------------------------------- | :------------------- | :------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| [imap](https://www.php.net/manual/imap.installation.php)                                                    | スレッドセーフでない | [javanile/php-imap2](https://github.com/javanile/php-imap2), [webklex/php-imap](https://github.com/Webklex/php-imap), [ImapEngine](https://github.com/DirectoryTree/ImapEngine) |
| [newrelic](https://docs.newrelic.com/docs/apm/agents/php-agent/getting-started/introduction-new-relic-php/) | スレッドセーフでない | -                                                                                                                                                                               |
| [pcov](https://github.com/krakjoe/pcov)                                                                     | スレッドセーフでない ([クラシックモードでのSIGSEGV](https://github.com/krakjoe/pcov/issues/132)) | CLI SAPIでカバレッジを収集する（FrankenPHPの外部でPHPUnit/Pestを実行する）                                                                                         |

## バグのあるPHP拡張モジュール

以下の拡張モジュールはFrankenPHPとの組み合わせで既知のバグや予期しない動作が確認されています：

| 名前                                  | 問題                                                                                                                                         |
| :------------------------------------ | :----------------------------------------------------------------------------------------------------------------------------------------------- |
| [datadog](https://github.com/DataDog) | FrankenPHPのプロファイリング時に不安定な動作が確認されています。このバグは[DataDogによって追跡中](https://github.com/DataDog/dd-trace-php/issues/3729)です。 |
| [blackfire](https://blackfire.io/)    | [FrankenPHPのサポートはベータ版であり、まだ機能が完全ではありません](https://docs.blackfire.io/php/integrations/frankenphp)。                            |
| [imagick](https://github.com/Imagick/imagick) | ImageMagickのOpenMPスレッドがFrankenPHPのスレッドと競合し、不安定性やクラッシュを引き起こします。\Imagick::setResourceLimit(\Imagick::RESOURCETYPE_THREAD, 1)でImageMagickのスレッドを無効にするか、`--disable-openmp`でImageMagickをコンパイルすることで軽減できます。静的バイナリおよび公式の`apt`/`apk`/`rpm`パッケージではOpenMPはすでに無効になっているため、DockerイメージとHomebrewインストールのみが影響を受けます。 |

## PHPのハードメモリ制限を超過するとサーバー全体が停止する

PHP 8.2.35、8.3.36、8.4.27、および8.5.12以降、`memory_limit`を超過してもスクリプトがすぐに停止しなくなりました。「Allowed memory size exhausted」の致命的なエラーは、次の関数呼び出しまたはループイテレーションで発生します。
それ以前にスクリプトがハードメモリ制限を超えて割り当てを行うと、PHPはプロセスを終了します。
ハード制限はデフォルトで`memory_limit`の2倍（または1 GiBを超える場合は`memory_limit`プラス1 GiB）であり、`memory_hard_limit_percent` INI設定で変更できます。

`memory_limit`を超えて実行され続けるコードは、例えば`memory_limit`より大きい単一の割り当て（大規模な配列に対する`array_merge()`など）や、連続する文字列連結によって、この制限に達する可能性があります。

PHP-FPMは1つの子プロセスしか失いません。
FrankenPHPはすべてのリクエストを単一プロセスのスレッドから処理するため、処理中のすべてのリクエストが失われ、サーバーは再起動されるまで停止します。
Dockerの再起動ポリシーやsystemdの`Restart=`（`deb`および`rpm`パッケージではすでに設定済み）など、FrankenPHPを再起動する監視プログラムの下で実行してください。

## get_browser

[get_browser()](https://www.php.net/manual/function.get-browser.php)関数は継続使用するとパフォーマンスが悪化することが確認されています。回避策として、User Agentごとの結果をキャッシュ（例：[APCu](https://www.php.net/manual/book.apcu.php)を利用）してください。User Agentごとの結果は静的なためです。

## スタンドアロンバイナリおよびAlpineベースのDockerイメージ

スタンドアロンバイナリおよびAlpineベースのDockerイメージ（`dunglas/frankenphp:*-alpine`）は、バイナリサイズを小さく保つために[glibc and friends](https://www.etalabs.net/compare_libcs.html)ではなく[musl libc](https://musl.libc.org/)を使用しています。これによりいくつかの互換性問題が発生する可能性があります。特に、globフラグ`GLOB_BRACE`は [サポートされていません](https://www.php.net/manual/function.glob.php) 。

問題が発生した場合は、静的バイナリのGNU版およびDebianベースのDockerイメージを使用することをお勧めします。

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
# /usr/local/bin/php
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

[`openssl_get_cert_locations()`](https://www.php.net/manual/function.openssl-get-cert-locations.php)の出力を調べて、
CA証明書をどこにインストールすべきか確認し、その場所に保存してください。

> [!WARNING]
>
> WebとCLIコンテキストでは設定が異なる場合があります。
> 適切なコンテキストで`openssl_get_cert_locations()`を実行してください。

[Mozillaから抽出されたCA証明書はcurlのサイトでダウンロードできます](https://curl.se/docs/caextract.html)。

または、Debian、Ubuntu、Alpineなどのディストリビューションでも、これらの証明書を含む`ca-certificates`というパッケージを提供しています。

`SSL_CERT_FILE`および`SSL_CERT_DIR`を使用してOpenSSLにCA証明書を探す場所をヒントとして与えることも可能です：

```console
# TLS 証明書の環境変数を設定
export SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
export SSL_CERT_DIR=/etc/ssl/certs
```
