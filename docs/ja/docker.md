# カスタムDockerイメージのビルド

[FrankenPHPのDockerイメージ](https://hub.docker.com/r/dunglas/frankenphp)は、[公式PHPイメージ](https://hub.docker.com/_/php/)をベースにしています。主要なアーキテクチャに対してDebianとAlpine Linuxのバリアントを提供しており、Debianバリアントの使用を推奨しています。

PHP 8.2、8.3、8.4、8.5向けのバリアントが提供されています。

タグは次のパターンに従います：`dunglas/frankenphp:<frankenphp-version>-php<php-version>-<os>`

- `<frankenphp-version>`および`<php-version>`は、それぞれFrankenPHPおよびPHPのバージョン番号で、メジャー（例：`1`）、マイナー（例：`1.2`）からパッチバージョン（例：`1.2.3`）まであります。
- `<os>`は`trixie`（Debian Trixie用）、`bookworm`（Debian Bookworm用）、または`alpine`（Alpine最新安定版用）のいずれかです。

[タグを閲覧](https://hub.docker.com/r/dunglas/frankenphp/tags)。

## イメージの使用方法

プロジェクトに`Dockerfile`を作成します：

```dockerfile
FROM dunglas/frankenphp

COPY . /app/public
```

次に、以下のコマンドを実行してDockerイメージをビルドし、実行します：

```console
docker build -t my-php-app .
docker run -it --rm --name my-running-app my-php-app
```

## 設定を調整する方法

利便性のため、役立つ環境変数を含む[デフォルトの`Caddyfile`](https://github.com/php/frankenphp/blob/main/caddy/frankenphp/Caddyfile)がイメージに含まれています。

## PHP拡張モジュールの追加インストール方法

ベースイメージには[`docker-php-extension-installer`](https://github.com/mlocati/docker-php-extension-installer)スクリプトが含まれており、
追加のPHP拡張モジュールを簡単にインストールできます：

```dockerfile
FROM dunglas/frankenphp

# ここに追加の拡張モジュールを追加：
RUN install-php-extensions \
	pdo_mysql \
	gd \
	intl \
	zip \
	opcache
```

## Caddyモジュールの追加インストール方法

FrankenPHPはCaddyをベースに構築されているため、すべての[Caddyモジュール](https://caddyserver.com/docs/modules/)をFrankenPHPでも使用できます。

カスタムCaddyモジュールをインストールする最も簡単な方法は、[xcaddy](https://github.com/caddyserver/xcaddy)を使用することです：

```dockerfile
FROM dunglas/frankenphp:builder AS builder

# builderイメージにxcaddyをコピー
COPY --from=caddy:builder /usr/bin/xcaddy /usr/bin/xcaddy

# FrankenPHPをビルドするにはCGOを有効にする必要があります
RUN CGO_ENABLED=1 \
    XCADDY_SETCAP=1 \
    XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
    CGO_CFLAGS=$(php-config --includes) \
    CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
    xcaddy build \
        --output /usr/local/bin/frankenphp \
        --with github.com/dunglas/frankenphp=./ \
        --with github.com/dunglas/frankenphp/caddy=./caddy/ \
        --with github.com/dunglas/caddy-cbrotli \
        # MercureとVulcainは公式ビルドに含まれていますが、お気軽に削除してください
        --with github.com/dunglas/mercure/caddy \
        --with github.com/dunglas/vulcain/caddy
        # ここに追加のCaddyモジュールを指定してください

FROM dunglas/frankenphp AS runner

# 公式バイナリをカスタムモジュールを含むものに置き換え
COPY --from=builder /usr/local/bin/frankenphp /usr/local/bin/frankenphp
```

FrankenPHPが提供する`builder`イメージには、コンパイル済みの`libphp`が含まれています。
[ビルダーイメージ](https://hub.docker.com/r/dunglas/frankenphp/tags?name=builder)は、FrankenPHPおよびPHPのすべてのバージョンに対して、DebianとAlpineの両方が提供されています。

> [!TIP]
>
> Alpine LinuxとSymfonyを使用している場合は、
> [デフォルトのスタックサイズを増やす](compile.md#using-xcaddy) 必要がある場合があります。

## デフォルトでワーカーモードを有効にする

FrankenPHPをワーカースクリプトで起動するには、`FRANKENPHP_CONFIG`環境変数を設定します：

```dockerfile
FROM dunglas/frankenphp

# ...

ENV FRANKENPHP_CONFIG="worker ./public/index.php"
```

## 開発時にボリュームを使う

FrankenPHPでの開発を簡単に行うには、ホスト側のアプリケーションのソースコードを含むディレクトリを、Dockerコンテナ内にボリュームとしてマウントします：

```console
docker run -v $PWD:/app/public -p 80:80 -p 443:443 -p 443:443/udp --tty my-php-app
```

> [!TIP]
>
> `--tty`オプションを使うと、JSONではなく人間が読みやすいログが表示されます。

Docker Composeを使用する場合：

```yaml
# compose.yaml

services:
  php:
    image: dunglas/frankenphp
    # カスタムDockerfileを使用したい場合は以下の行のコメントを外してください
    #build: .
    # 本番環境で使用する場合は以下の行のコメントを外してください
    # restart: always
    ports:
      - "80:80" # HTTP
      - "443:443" # HTTPS
      - "443:443/udp" # HTTP/3
    volumes:
      - ./:/app/public
      - caddy_data:/data
      - caddy_config:/config
    # 開発環境で人間が読みやすいログを出力するため、本番ではこの行をコメントアウトしてください
    tty: true

# Caddyの証明書や設定に必要なボリューム
volumes:
  caddy_data:
  caddy_config:
```

## 非rootユーザーとして実行する

FrankenPHPはDockerで非rootユーザーとして実行できます。

これを行うサンプル`Dockerfile`は以下の通りです：

```dockerfile
FROM dunglas/frankenphp

ARG USER=appuser

RUN \
	# Alpine系ディストリビューションでは "adduser -D ${USER}" を使用
	useradd ${USER}; \
	# ポート 80 や 443 にバインドするための追加ケーパビリティを追加
	setcap CAP_NET_BIND_SERVICE=+eip /usr/local/bin/frankenphp; \
	# /config/caddy および /data/caddy への書き込み権限を付与
	chown -R ${USER}:${USER} /config/caddy /data/caddy

USER ${USER}
```

### ケーパビリティなしでの実行

FrankenPHPをroot以外のユーザーで実行する場合でも、特権ポート（80と443）でWebサーバーを
バインドするために`CAP_NET_BIND_SERVICE`ケーパビリティが必要です。

FrankenPHPを非特権ポート（1024以上）で公開する場合は、
ウェブサーバーを非rootユーザーとして実行し、ケーパビリティを必要とせずに実行することが可能です：

```dockerfile
FROM dunglas/frankenphp

ARG USER=appuser

RUN \
	# Alpine 系ディストリビューションでは "adduser -D ${USER}" を使用
	useradd ${USER}; \
	# デフォルトのケーパビリティを削除
	setcap -r /usr/local/bin/frankenphp; \
	# /config/caddy と /data/caddy への書き込み権限を付与
	chown -R ${USER}:${USER} /config/caddy /data/caddy

USER ${USER}
```

その後、`SERVER_NAME`環境変数を設定して非特権ポートを使用します。
例： `:8000`

## アップデート

Dockerイメージは以下のタイミングでビルドされます：

- 新しいリリースがタグ付けされたとき
- 公式PHPイメージに新しいバージョンがある場合、毎日UTC午前4時に自動ビルド

## イメージの強化

FrankenPHP Dockerイメージの攻撃対象領域とサイズをさらに削減するために、[Google distroless](https://github.com/GoogleContainerTools/distroless)または[Docker hardened](https://www.docker.com/products/hardened-images)イメージをベースにビルドすることも可能です。

> [!WARNING]
> これらの最小限のベースイメージにはシェルやパッケージマネージャーが含まれていないため、デバッグがより困難になります。そのため、セキュリティが最優先される本番環境でのみ推奨されます。

追加のPHP拡張機能を追加する場合は、中間ビルドステージが必要になります：

```dockerfile
FROM dunglas/frankenphp AS builder

# ここに追加のPHP拡張機能を追加
RUN install-php-extensions pdo_mysql pdo_pgsql #...

# frankenphpとインストールされているすべての拡張機能の共有ライブラリを一時的な場所にコピー
# この手順は、frankenphpバイナリおよび各拡張機能の.soファイルのldd出力を分析することで手動で行うこともできます
RUN apt-get update && apt-get install -y libtree && \
    EXT_DIR="$(php -r 'echo ini_get("extension_dir");')" && \
    FRANKENPHP_BIN="$(which frankenphp)"; \
    LIBS_TMP_DIR="/tmp/libs"; \
    mkdir -p "$LIBS_TMP_DIR"; \
    for target in "$FRANKENPHP_BIN" $(find "$EXT_DIR" -maxdepth 2 -type f -name "*.so"); do \
        libtree -pv "$target" | sed 's/.*── \(.*\) \[.*/\1/' | grep -v "^$target" | while IFS= read -r lib; do \
            [ -z "$lib" ] && continue; \
            base=$(basename "$lib"); \
            destfile="$LIBS_TMP_DIR/$base"; \
            if [ ! -f "$destfile" ]; then \
                cp "$lib" "$destfile"; \
            fi; \
        done; \
    done


# Distroless Debianベースイメージ。ベースイメージと同じDebianバージョンであることを確認してください
FROM gcr.io/distroless/base-debian13
# Docker hardened イメージの代替
# FROM dhi.io/debian:13

# コンテナにコピーするアプリケーションとCaddyfileの場所
ARG PATH_TO_APP="."
ARG PATH_TO_CADDYFILE="./Caddyfile"

# アプリケーションを/appにコピー
# さらに強化するために、書き込み可能なパスのみが非rootユーザーに所有されていることを確認してください
COPY --chown=nonroot:nonroot "$PATH_TO_APP" /app
COPY "$PATH_TO_CADDYFILE" /etc/caddy/Caddyfile

# frankenphpと必要なライブラリをコピー
COPY --from=builder /usr/local/bin/frankenphp /usr/local/bin/frankenphp
COPY --from=builder /usr/local/lib/php/extensions /usr/local/lib/php/extensions
COPY --from=builder /tmp/libs /usr/lib

# php.ini設定ファイルをコピー
COPY --from=builder /usr/local/etc/php/conf.d /usr/local/etc/php/conf.d
COPY --from=builder /usr/local/etc/php/php.ini-production /usr/local/etc/php/php.ini

# Caddyデータディレクトリ — 読み取り専用のルートファイルシステム上でも、非rootユーザーが書き込み可能である必要があります
ENV XDG_CONFIG_HOME=/config \
    XDG_DATA_HOME=/data
COPY --from=builder --chown=nonroot:nonroot /data/caddy /data/caddy
COPY --from=builder --chown=nonroot:nonroot /config/caddy /config/caddy

USER nonroot

WORKDIR /app

# 指定されたCaddyfileでfrankenphpを実行するエントリポイント
ENTRYPOINT ["/usr/local/bin/frankenphp", "run", "-c", "/etc/caddy/Caddyfile"]
```

## 開発版

開発版は[`dunglas/frankenphp-dev`](https://hub.docker.com/repository/docker/dunglas/frankenphp-dev)Dockerリポジトリで利用できます。
GitHubリポジトリの`main`ブランチにコミットがpushされるたびに新しいビルドが実行されます。

`latest*`タグは`main`ブランチのヘッドを指しており、`sha-<git-commit-hash>` 形式のタグも利用可能です。
