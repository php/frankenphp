# Laravel

## Docker

使用 FrankenPHP 为 [Laravel](https://laravel.com) Web 应用程序提供服务就像将项目挂载到官方 Docker 镜像的 `/app` 目录中一样简单。

从 Laravel 应用程序的主目录运行以下命令：

```console
docker run -p 80:80 -p 443:443 -p 443:443/udp -v $PWD:/app dunglas/frankenphp
```

尽情享受吧！

## 本地安装

或者，你可以从本地机器上使用 FrankenPHP 运行 Laravel 项目：

1. [下载与您的系统相对应的二进制文件](../#standalone-binary)
2. 将以下配置添加到 Laravel 项目根目录中名为 `Caddyfile` 的文件中：

   ```caddyfile
   {
   	frankenphp
   }

   # 服务器的域名
   localhost {
   	# 将 webroot 设置为 public/ 目录
   	root public/
   	# 启用压缩(可选)
   	encode zstd br gzip
   	# 从 public/ 目录执行 PHP 文件并提供静态文件
   	php_server {
   	    try_files {path} index.php
       }
   }
   ```

3. 从 Laravel 项目的根目录启动 FrankenPHP：`frankenphp run`

## Laravel Octane

Octane 可以通过 Composer 包管理器安装：

```console
composer require laravel/octane
```

安装 Octane 后，你可以执行 `octane:install` Artisan 命令，该命令会将 Octane 的配置文件安装到你的应用程序中：

```console
php artisan octane:install --server=frankenphp
```

Octane 服务可以通过 `octane:frankenphp` Artisan 命令启动。

```console
php artisan octane:frankenphp
```

`octane:frankenphp` 命令可以采用以下选项：

- `--host`: 服务器应绑定到的 IP 地址（默认值: `127.0.0.1`）
- `--port`: 服务器应可用的端口（默认值: `8000`）
- `--admin-port`: 管理服务器应可用的端口（默认值: `2019`）
- `--workers`: 应可用于处理请求的 worker 数（默认值: `auto`）
- `--max-requests`: 在服务器重新加载前处理的请求数量（默认值: `500`）
- `--caddyfile`：FrankenPHP `Caddyfile` 文件的路径（默认： [Laravel Octane 中的存根 `Caddyfile`](https://github.com/laravel/octane/blob/2.x/src/Commands/stubs/Caddyfile)）
- `--https`: 开启 HTTPS、HTTP/2 和 HTTP/3，自动生成和续订证书
- `--http-redirect`: 启用 HTTP 到 HTTPS 重定向（仅在使用 `--https` 时启用）
- `--watch`: 修改应用程序时自动重新加载服务器
- `--poll`: 在监视时使用文件系统轮询，以便通过网络监视文件
- `--log-level`: 使用原生 Caddy 日志记录器，记录指定日志级别或更高级别的消息

> [!TIP]
> 要获取结构化的 JSON 日志（在使用日志分析解决方案时非常有用），请明确传递 `--log-level` 选项。

另请参阅 [如何在 Octane 中使用 Mercure](#mercure-support)。

在 [Laravel Octane 官方文档](https://laravel.com/docs/octane) 中了解更多信息。

## Laravel 应用程序作为独立二进制文件

使用[FrankenPHP 的应用嵌入功能](embed.md)，可以将 Laravel 应用程序作为
独立的二进制文件分发。

按照以下步骤将您的Laravel应用程序打包为Linux的独立二进制文件：

1. 在您的应用程序的存储库中创建一个名为 `static-build.Dockerfile` 的文件:

   ```dockerfile
   FROM --platform=linux/amd64 dunglas/frankenphp:static-builder-gnu
   # 如果你打算在 musl-libc 系统上运行该二进制文件，请使用 static-builder-musl

   # 复制你的应用
   WORKDIR /go/src/app/dist/app
   COPY . .

   # 删除测试和其他不必要的文件以节省空间
   # 或者，将这些文件添加到 .dockerignore 文件中
   RUN rm -Rf tests/

   # 复制 .env 文件
   RUN cp .env.example .env
   # 将 APP_ENV 和 APP_DEBUG 更改为适合生产环境
   RUN sed -i'' -e 's/^APP_ENV=.*/APP_ENV=production/' -e 's/^APP_DEBUG=.*/APP_DEBUG=false/' .env

   # 根据需要对您的 .env 文件进行其他更改

   # 安装依赖项
   RUN composer install --ignore-platform-reqs --no-dev -a

   # 构建静态二进制文件
   WORKDIR /go/src/app/
   RUN EMBED=dist/app/ ./build-static.sh
   ```

   > [!CAUTION]
   >
   > 一些 `.dockerignore` 文件
   > 将忽略 `vendor/` 目录和 `.env` 文件。在构建之前，请确保调整或删除 `.dockerignore` 文件。

2. 构建:

   ```console
   docker build -t static-laravel-app -f static-build.Dockerfile .
   ```

3. 提取二进制:

   ```console
   docker cp $(docker create --name static-laravel-app-tmp static-laravel-app):/go/src/app/dist/frankenphp-linux-x86_64 frankenphp ; docker rm static-laravel-app-tmp
   ```

4. 填充缓存：

   ```console
   frankenphp php-cli artisan optimize
   ```

5. 运行数据库迁移（如果有的话）：

   ```console
   frankenphp php-cli artisan migrate
   ```

6. 生成应用程序的密钥：

   ```console
   frankenphp php-cli artisan key:generate
   ```

7. 启动服务器：

   ```console
   frankenphp php-server
   ```

您的应用程序现在准备好了！

了解有关可用选项的更多信息，以及如何为其他操作系统构建二进制文件，请参见 [应用程序嵌入](embed.md)
文档。

### 更改存储路径

默认情况下，Laravel 将上传的文件、缓存、日志等存储在应用程序的 `storage/` 目录中。
这不适合嵌入式应用，因为每个新版本将被提取到不同的临时目录中。

设置 `LARAVEL_STORAGE_PATH` 环境变量（例如，在 `.env` 文件中）或调用 `Illuminate\Foundation\Application::useStoragePath()` 方法以使用临时目录之外的目录。

### Mercure 支持

[Mercure](https://mercure.rocks) 是为您的 Laravel 应用程序添加实时功能的绝佳方式。
FrankenPHP [开箱即用地支持 Mercure](mercure.md)。

如果您未使用 [Octane](#laravel-octane)，请参阅 [Mercure 文档条目](mercure.md)。

如果您正在使用 Octane，可以通过在您的 `config/octane.php` 文件中添加以下行来启用 Mercure 支持：

```php
// ...

return [
    // ...

    'mercure' => [
        'anonymous' => true,
        'publisher_jwt' => '!ChangeThisMercureHubJWTSecretKey!',
        'subscriber_jwt' => '!ChangeThisMercureHubJWTSecretKey!',
    ],
];
```

您可以在此数组中使用 [Mercure 支持的所有指令](https://mercure.rocks/docs/hub/config#directives)。

要发布和订阅更新，我们推荐使用 [Laravel Mercure Broadcaster](https://github.com/mvanduijker/laravel-mercure-broadcaster) 库。
或者，请参阅 [Mercure 文档](mercure.md) 以纯 PHP 和 JavaScript 实现。

### 使用独立二进制文件运行 Octane

甚至可以将 Laravel Octane 应用打包为独立的二进制文件！

为此，[正确安装 Octane](#laravel-octane) 并遵循 [独立二进制文件形式的 Laravel 应用程序](#laravel-apps-as-standalone-binaries) 中描述的步骤。

然后，通过 Octane 在工作模式下启动 FrankenPHP，运行：

```console
PATH="$PWD:$PATH" frankenphp php-cli artisan octane:frankenphp
```

> [!CAUTION]
>
> 为了使命令有效，独立二进制文件**必须**命名为 `frankenphp`
> 因为 Octane 需要一个名为 `frankenphp` 的程序在路径中可用。
