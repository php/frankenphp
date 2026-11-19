---
title: NginxとPHP-FPMからFrankenPHPへの移行
description: NginxとPHP-FPMスタックからFrankenPHPへのPHPアプリケーションの移行に関するステップバイステップガイド。Caddyfile、php.ini、スレッド、Dockerについて説明します。
---

# Nginx/PHP-FPMからの移行

FrankenPHPは、ウェブサーバー (Nginx、Apache) とPHP-FPMの両方を単一のバイナリで置き換えます。
このガイドでは、一般的なPHPアプリケーションの基本的な移行について説明します。

## 主な相違点

| PHP-FPM セットアップ                     | FrankenPHP の同等機能                                    |
| --------------------------------- | -------------------------------------------------------- |
| Nginx/Apache + PHP-FPM            | 単一の `frankenphp` バイナリ                               |
| `php-fpm.conf` プール設定      | [`frankenphp` グローバルオプション](config.md#caddyfile-config) |
| Nginx `server {}` ブロック           | `Caddyfile` サイトブロック                                   |
| `php_value` / `php_admin_value`   | [`php_ini` Caddyfile ディレクティブ](config.md#php-config)    |
| `pm = static` / `pm.max_children` | `num_threads`                                            |
| `pm = dynamic`                    | [`max_threads auto`](performance.md#max_threads)         |
| Webサーバーのリクエストフィルタリング      | Caddy ルートとマッチャー                                |

## HTTPリクエストのフィルタリング

標準のNginxまたはApacheパッケージから移行する場合、PHP-FPMがリクエストを受け取る前にウェブサーバーによって行われていたリクエストフィルタリングを確認してください。
FrankenPHPはCaddy上に構築されているため、`Caddyfile`または別のプロキシが最初に拒否しない限り、有効なHTTPメソッドとヘッダーはCaddyの通常のルーティングを通過します。

例えば、一部のウェブサーバーはデフォルトで`TRACE`リクエストを拒否します。
アプリケーションがその動作を維持する必要がある場合は、`php_server`の前に明示的なマッチャーを追加してください。

```caddyfile
example.com {
    @trace method TRACE
    respond @trace 405

    root /var/www/app/public
    php_server
}
```

以前のウェブサーバーが強制していたより厳密なメソッドまたはヘッダーポリシーに対しても、同じアプローチを適用してください。
リクエストヘッダー名にも同じことが言えます。一部のウェブサーバーやプロキシは、PHP-FPMがそれらを見る前に有効だが一般的ではないHTTPフィールド名を拒否または破棄しますが、FrankenPHPはそれらをPHPアプリケーションに渡す場合があります。

## ステップ1: ウェブサーバー設定の置き換え

典型的なNginx + PHP-FPMの設定:

```nginx
# /etc/nginx/sites-available/example.com
server {
    listen 80;
    server_name example.com;
    root /var/www/app/public;
    index index.php;

    location / {
        try_files $uri $uri/ /index.php$is_args$args;
    }

    location ~ \.php$ {
        fastcgi_pass unix:/run/php/php-fpm.sock;
        fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
        include fastcgi_params;
    }
}
```

単一の`Caddyfile`になります:

```caddyfile
example.com {
    root /var/www/app/public
    php_server
}
```

これだけです。`php_server`ディレクティブは、PHPルーティング、`try_files`のような動作、および静的ファイルの提供を処理します。

## ステップ2: PHP設定の移行

既存の`php.ini`はそのまま動作します。[設定](config.md)で、インストール方法に応じてどこに配置するかを確認してください。

1つの設定の意味が変わります: `max_execution_time`はFrankenPHPでは実時間をカウントしますが、Linux上のPHP-FPMの非ZTSビルドではCPU時間をカウントします。
データベースクエリ、HTTP呼び出し、`sleep()`、またはセッションファイルのロックを待つ時間も制限にカウントされるため、PHP-FPM用に調整された値は低すぎる可能性があります。

ディレクティブを`Caddyfile`に直接設定することもできます:

```caddyfile
{
    frankenphp {
        php_ini memory_limit 256M
        php_ini max_execution_time 30
    }
}

example.com {
    root /var/www/app/public
    php_server
}
```

## ステップ3: プールサイズ調整

PHP-FPMでは、ワーカープロセスの数を制御するために`pm.max_children`を調整します。
FrankenPHPでは、これに相当するのが`num_threads`です。

```caddyfile
{
    frankenphp {
        num_threads 16
    }
}
```

デフォルトでは、FrankenPHPはCPUあたり2つのスレッドを開始します。PHP-FPMの`pm = dynamic`に似た動的なスケーリングには:

```caddyfile
{
    frankenphp {
        num_threads 4
        max_threads auto
    }
}
```

## ステップ4: Docker移行

2つのコンテナ (Nginx + PHP-FPM) を使用する典型的なPHP-FPM Dockerセットアップは、単一のコンテナに置き換えることができます。

**変更前:**

```yaml
# compose.yaml
services:
  nginx:
    image: nginx:1
    volumes:
      # mount Nginx config into the container
      - ./config:/etc/nginx/conf.d
      - .:/var/www/app
    ports:
      - "80:80"
      - "443:443"

  php:
    image: php:8.5-fpm
    volumes:
      - .:/var/www/app
```

**変更後:**

```yaml
# compose.yaml
services:
  php:
    image: dunglas/frankenphp:1-php8.5
    volumes:
      - .:/var/www/app
      # mount the Caddyfile into the container
      - ./config:/etc/frankenphp
      - caddy_data:/data
      - caddy_config:/config
    ports:
      - "80:80"
      - "443:443"
      - "443:443/udp"

volumes:
  caddy_data:
  caddy_config:
```

追加のPHP拡張機能が必要な場合は、[カスタム Docker イメージの構築](docker.md#how-to-install-more-php-extensions) を参照してください。

フレームワーク固有のDockerセットアップについては、[Symfony Docker](https://github.com/dunglas/symfony-docker) と [FrankenPHP Docker イメージでLaravelを実行する](laravel.md#running-laravel-with-the-frankenphp-docker-image) を参照してください。

## ステップ5: ワーカーモードの検討 (オプション)

[クラシックモード](classic.md)では、FrankenPHPはPHP-FPMと同様に動作します。各リクエストはアプリケーションを最初から起動します。これは移行の安全な出発点です。

パフォーマンスを向上させるには、[ワーカーモード](worker.md) に切り替えることができます。これはアプリケーションを一度起動し、メモリに保持します。

```caddyfile
example.com {
    root /var/www/app/public
    php_server {
        root /var/www/app/public
        worker index.php 4
    }
}
```

> [!注意]
>
> ワーカーモードはアプリケーションをリクエスト間でメモリに保持します。コードがリクエスト間でグローバル状態がリセットされることに依存していないことを確認してください。[Symfony](worker.md#worker-mode-for-symfony)、[Laravel](laravel.md#laravel-octane)、および [API Platform](https://api-platform.com) のようなフレームワークは、このモードをネイティブでサポートしています。

## 削除できるもの

移行後、以下のものは不要になります。

- NginxまたはApache
- PHP-FPM (`php-fpm`サービス/プロセス)
- FastCGI設定
- 自己管理型TLS証明書 (Caddyが自動的に処理します)
