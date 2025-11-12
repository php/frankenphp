# 使用 FrankenPHP Workers

启动一次应用程序并将其保存在内存中。
FrankenPHP 将在几毫秒内处理传入请求。

## 启动 Worker 脚本

### Docker

将 `FRANKENPHP_CONFIG` 环境变量的值设置为 `worker /path/to/your/worker/script.php`：

```console
docker run \
    -e FRANKENPHP_CONFIG="worker /app/path/to/your/worker/script.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### 独立二进制文件

使用 `php-server` 命令的 `--worker` 选项通过 worker 为当前目录的内容提供服务：

```console
frankenphp php-server --worker /path/to/your/worker/script.php
```

如果你的 PHP 应用程序已[嵌入到二进制文件中](embed.md)，你可以在应用程序的根目录中添加自定义的 `Caddyfile`。
它将被自动使用。

还可以使用 `--watch` 选项在[文件更改时重启 worker](config.md#watching-for-file-changes)。
如果 `/path/to/your/app/` 目录或子目录中任何以 `.php` 结尾的文件被修改，以下命令将触发重启：

```console
frankenphp php-server --worker /path/to/your/worker/script.php --watch="/path/to/your/app/**/*.php"
```

## Symfony Runtime

FrankenPHP 的 worker 模式由 [Symfony Runtime Component](https://symfony.com/doc/current/components/runtime.html) 支持。
要在 worker 中启动任何 Symfony 应用程序，请安装 [PHP Runtime](https://github.com/php-runtime/runtime) 的 FrankenPHP 包：

```console
composer require runtime/frankenphp-symfony
```

通过定义 `APP_RUNTIME` 环境变量来使用 FrankenPHP Symfony Runtime 启动你的应用服务器：

```console
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php" \
    -e APP_RUNTIME=Runtime\\FrankenPhpSymfony\\Runtime \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

## Laravel Octane

请参阅[专门的文档](laravel.md#laravel-octane)。

## 自定义应用程序

以下示例展示了如何创建自己的 worker 脚本而不依赖第三方库：

```php
<?php
// public/index.php

// 防止客户端连接中断时 worker 脚本终止
ignore_user_abort(true);

// 启动你的应用程序
require __DIR__.'/vendor/autoload.php';

$myApp = new \App\Kernel();
$myApp->boot();

// 在循环外的处理器以获得更好的性能（减少工作量）
$handler = static function () use ($myApp) {
    // 当收到请求时调用，
    // 超全局变量、php://input 等都会被重置
    echo $myApp->handle($_GET, $_POST, $_COOKIE, $_FILES, $_SERVER);
};

$maxRequests = (int)($_SERVER['MAX_REQUESTS'] ?? 0);
for ($nbRequests = 0; !$maxRequests || $nbRequests < $maxRequests; ++$nbRequests) {
    $keepRunning = \frankenphp_handle_request($handler);

    // 在发送 HTTP 响应后做一些事情
    $myApp->terminate();

    // 调用垃圾收集器以减少在页面生成过程中触发垃圾收集的可能性
    gc_collect_cycles();

    if (!$keepRunning) break;
}

// 清理
$myApp->shutdown();
```

然后，启动你的应用程序并使用 `FRANKENPHP_CONFIG` 环境变量配置你的 worker：

```console
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

默认情况下，每个 CPU 启动 2 个 worker。
你也可以配置要启动的 worker 数量：

```console
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php 42" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### 在处理一定数量的请求后重启 Worker

由于 PHP 最初不是为长时间运行的进程而设计的，仍有许多库和传统代码会泄漏内存。
在 worker 模式下使用此类代码的一个解决方法是在处理一定数量的请求后重启 worker 脚本：

前面的 worker 代码片段允许通过设置名为 `MAX_REQUESTS` 的环境变量来配置要处理的最大请求数。

### 手动重启 Workers

虽然可以在[文件更改时重启 workers](config.md#watching-for-file-changes)，但也可以通过 [Caddy admin API](https://caddyserver.com/docs/api) 优雅地重启所有 workers。如果在你的 [Caddyfile](config.md#caddyfile-config) 中启用了 admin，你可以通过简单的 POST 请求 ping 重启端点，如下所示：

```console
curl -X POST http://localhost:2019/frankenphp/workers/restart
```

## 超全局变量行为

[PHP 超全局变量](https://www.php.net/manual/zh/language.variables.superglobals.php)（`$_SERVER`、`$_ENV`、`$_GET`...）
行为如下：

- 在第一次调用 `frankenphp_handle_request()` 之前，超全局变量包含绑定到 worker 脚本本身的值
- 在调用 `frankenphp_handle_request()` 期间和之后，超全局变量包含从处理的 HTTP 请求生成的值，每次调用 `frankenphp_handle_request()` 都会更改超全局变量的值

要在回调内访问 worker 脚本的超全局变量，必须复制它们并将副本导入到回调的作用域中：

```php
<?php
// 在第一次调用 frankenphp_handle_request() 之前复制 worker 的 $_SERVER 超全局变量
$workerServer = $_SERVER;

$handler = static function () use ($workerServer) {
    var_dump($_SERVER); // 与请求绑定的 $_SERVER
    var_dump($workerServer); // worker 脚本的 $_SERVER
};

// ...
```
