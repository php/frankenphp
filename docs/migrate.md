# Migrating from PHP-FPM

FrankenPHP replaces both your web server (Nginx, Apache) and PHP-FPM with a single binary.
This guide covers a basic migration for a typical PHP application.

## Key Differences

| PHP-FPM setup | FrankenPHP equivalent |
|---|---|
| Nginx/Apache + PHP-FPM | Single `frankenphp` binary |
| `php-fpm.conf` pool settings | [`frankenphp` global option](config.md#caddyfile-config) |
| Nginx `server {}` block | `Caddyfile` site block |
| `php_value` / `php_admin_value` | [`php_ini` Caddyfile directive](config.md#php-config) |
| `pm = static` / `pm.max_children` | `num_threads` |
| `pm = dynamic` | [`max_threads auto`](performance.md#max_threads) |

## Step 1: Replace Your Web Server Config

A typical Nginx + PHP-FPM configuration:

```nginx
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

Becomes a single `Caddyfile`:

```caddyfile
example.com {
    root /var/www/app/public
    php_server
}
```

That's it. The `php_server` directive handles PHP routing, `try_files`-like behavior, and static file serving.

## Step 2: Migrate PHP Configuration

Your existing `php.ini` works as-is. See [Configuration](config.md) for where to place it depending on your installation method.

You can also set directives directly in the `Caddyfile`:

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

## Step 3: Adjust Pool Size

In PHP-FPM, you tune `pm.max_children` to control the number of worker processes.
In FrankenPHP, the equivalent is `num_threads`:

```caddyfile
{
    frankenphp {
        num_threads 16
    }
}
```

By default, FrankenPHP starts 2 threads per CPU. For dynamic scaling similar to PHP-FPM's `pm = dynamic`:

```caddyfile
{
    frankenphp {
        num_threads 4
        max_threads auto
    }
}
```

## Step 4: Docker Migration

A typical PHP-FPM Docker setup using two containers (Nginx + PHP-FPM) can be replaced by a single container:

**Before:**

```yaml
services:
  nginx:
    image: nginx:1
    volumes:
      - ./nginx.conf:/etc/nginx/conf.d/default.conf
      - .:/var/www/app
    ports:
      - "80:80"

  php:
    image: php:8.5-fpm
    volumes:
      - .:/var/www/app
```

**After:**

```yaml
services:
  php:
    image: dunglas/frankenphp:1-php8.5
    volumes:
      - .:/app/public
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

If you need additional PHP extensions, see [Building Custom Docker Image](docker.md#how-to-install-more-php-extensions).

## Step 5: Consider Worker Mode (Optional)

In [classic mode](classic.md), FrankenPHP works like PHP-FPM: each request boots the application from scratch. This is a safe starting point for migration.

For better performance, you can switch to [worker mode](worker.md), which boots your application once and keeps it in memory:

```caddyfile
example.com {
    root /var/www/app/public
    php_server {
        root /var/www/app/public
        worker index.php 4
    }
}
```

> [!CAUTION]
>
> Worker mode keeps your application in memory between requests. Make sure your code does not rely on global state being reset between requests. Frameworks like [Symfony](worker.md#symfony-runtime), [Laravel](laravel.md#laravel-octane), and [API Platform](https://api-platform.com) have native support for this mode.

## What You Can Remove

After migrating, you no longer need:

- Nginx or Apache
- PHP-FPM (`php-fpm` service/process)
- FastCGI configuration
- Self-managed TLS certificates (Caddy handles them automatically)
