---
title: 从 Nginx 和 PHP-FPM 迁移到 FrankenPHP
description: 将 PHP 应用程序从 Nginx 加 PHP-FPM 堆栈迁移到 FrankenPHP 的分步指南，涵盖 Caddyfile、php.ini、线程和 Docker。
---

# 从 Nginx/PHP-FPM 迁移

FrankenPHP 将您的 Web 服务器（Nginx、Apache）和 PHP-FPM 都替换为一个单独的二进制文件。本指南涵盖了典型 PHP 应用程序的基本迁移。

## 主要区别

| PHP-FPM 设置                     | FrankenPHP 等效项                                    |
| --------------------------------- | -------------------------------------------------------- |
| Nginx/Apache + PHP-FPM            | 单个 `frankenphp` 二进制文件                               |
| `php-fpm.conf` 池设置      | [`frankenphp` 全局选项](config.md#caddyfile-配置) |
| Nginx `server {}` 块           | `Caddyfile` 站点块                                   |
| `php_value` / `php_admin_value`   | [`php_ini` Caddyfile 指令](config.md#php-配置)    |
| `pm = static` / `pm.max_children` | `num_threads`                                            |
| `pm = dynamic`                    | [`max_threads auto`](performance.md#max_threads)         |
| Web 服务器请求过滤      | Caddy 路由和匹配器                                |

## HTTP 请求过滤

从标准的 Nginx 或 Apache 软件包迁移时，请检查在 PHP-FPM 收到请求之前由 Web 服务器完成的任何请求过滤。
FrankenPHP 基于 Caddy 构建，因此有效的 HTTP 方法和头信息会通过 Caddy 的正常路由传递，除非您的 `Caddyfile` 或其他代理首先拒绝它们。

例如，某些 Web 服务器默认拒绝 `TRACE` 请求。如果您的应用程序应保留此行为，请在 `php_server` 之前添加一个显式匹配器：

```caddyfile
example.com {
    @trace method TRACE
    respond @trace 405

    root /var/www/app/public
    php_server
}
```

将相同的方法应用于您的旧 Web 服务器强制执行的任何更严格的方法或头信息策略。请求头名称也适用：某些 Web 服务器或代理在 PHP-FPM 看到它们之前拒绝或删除有效但罕见的 HTTP 字段名称，而 FrankenPHP 可能会将它们传递给 PHP 应用程序。

## 步骤 1：替换您的 Web 服务器配置

典型的 Nginx + PHP-FPM 配置：

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

变成一个单独的 `Caddyfile`：

```caddyfile
example.com {
    root /var/www/app/public
    php_server
}
```

就是这样。`php_server` 指令处理 PHP 路由、`try_files` 类似的行为以及静态文件服务。

## 步骤 2：迁移 PHP 配置

您现有的 `php.ini` 可以按原样工作。有关其放置位置（取决于您的安装方法），请参阅[配置](config.md)。

一个设置的含义有所改变：`max_execution_time` 在 FrankenPHP 中计算实际时间，而在 Linux 上非 ZTS 编译的 PHP-FPM 则计算 CPU 时间。等待数据库查询、HTTP 调用、`sleep()` 或会话文件锁定的时间现在计入限制，因此为 PHP-FPM 调整的值可能过低。

您也可以直接在 `Caddyfile` 中设置指令：

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

## 步骤 3：调整池大小

在 PHP-FPM 中，您通过调整 `pm.max_children` 来控制 worker 进程的数量。在 FrankenPHP 中，等效项是 `num_threads`：

```caddyfile
{
    frankenphp {
        num_threads 16
    }
}
```

默认情况下，FrankenPHP 为每个 CPU 启动 2 个线程。对于类似于 PHP-FPM 的 `pm = dynamic` 的动态扩展：

```caddyfile
{
    frankenphp {
        num_threads 4
        max_threads auto
    }
}
```

## 步骤 4：Docker 迁移

典型的使用两个容器（Nginx + PHP-FPM）的 PHP-FPM Docker 设置可以被单个容器替换：

**之前：**

```yaml
# compose.yaml
services:
  nginx:
    image: nginx:1
    volumes:
      # 将 Nginx 配置挂载到容器中
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

**之后：**

```yaml
# compose.yaml
services:
  php:
    image: dunglas/frankenphp:1-php8.5
    volumes:
      - .:/var/www/app
      # 将 Caddyfile 挂载到容器中
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

如果您需要额外的 PHP 扩展，请参阅[构建自定义 Docker 镜像](docker.md#如何安装更多php扩展)。

对于特定框架的 Docker 设置，请参阅 [Symfony Docker](https://github.com/dunglas/symfony-docker) 和[使用 FrankenPHP Docker 镜像运行 Laravel](laravel.md#使用frankenphp-docker镜像运行laravel)。

## 步骤 5：考虑 Worker 模式（可选）

在[经典模式](classic.md)下，FrankenPHP 的工作方式类似于 PHP-FPM：每个请求都从头开始启动应用程序。这是一个安全的迁移起点。

为了获得更好的性能，您可以切换到[Worker 模式](worker.md)，它会启动您的应用程序一次并将其保留在内存中：

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
> Worker 模式将您的应用程序保留在内存中，跨越多个请求。请确保您的代码不依赖于请求之间全局状态的重置。像 [Symfony](worker.md#symfony的worker-模式)、[Laravel](laravel.md#laravel-octane) 和 [API Platform](https://api-platform.com) 等框架原生支持此模式。

## 您可以移除什么

迁移后，您不再需要：

- Nginx 或 Apache
- PHP-FPM (`php-fpm` 服务/进程)
- FastCGI 配置
- 自行管理的 TLS 证书（Caddy 会自动处理它们）
