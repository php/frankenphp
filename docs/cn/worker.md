---
title: FrankenPHP worker 模式：保持你的 PHP 应用程序在内存中
description: 以 worker 模式运行 FrankenPHP，使你的 PHP 应用程序在请求之间保持启动状态，减少启动开销，并在几毫秒内提供响应。
---

# 使用 FrankenPHP workers

启动你的应用程序一次并将其保存在内存中。
FrankenPHP 将在几毫秒内处理传入请求。

## 启动 FrankenPHP worker 脚本

### 使用 Docker 运行 FrankenPHP worker

将 `FRANKENPHP_CONFIG` 环境变量的值设置为 `worker /path/to/your/worker/script.php`：

```bash
docker run \
    -e FRANKENPHP_CONFIG="worker /app/path/to/your/worker/script.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### 使用独立二进制文件运行 FrankenPHP worker

使用 `php-server` 命令的 `--worker` 选项，通过 worker 为当前目录的内容提供服务：

```bash
frankenphp php-server --worker /path/to/your/worker/script.php
```

如果你的 PHP 应用程序已[嵌入到二进制文件中](embed.md)，你可以在应用程序的根目录中添加自定义的 `Caddyfile`。
它将被自动使用。

还可以使用 `--watch` 选项在[文件更改时重启 worker](config.md#watching-for-file-changes)。
如果 `/path/to/your/app/` 目录或子目录中任何以 `.php` 结尾的文件被修改，以下命令将触发重启：

```bash
frankenphp php-server --worker /path/to/your/worker/script.php --watch="/path/to/your/app/**/*.php"
```

此功能通常与[热重载](hot-reload.md)结合使用。

## 适用于 Symfony 的 Worker 模式

请参阅[FrankenPHP Symfony worker 模式文档](symfony.md#symfony-worker-mode-with-frankenphp)。

## 适用于 Laravel Octane 的 Worker 模式

请参阅[FrankenPHP Laravel Octane 文档](laravel.md#laravel-octane)。

## 编写自定义 FrankenPHP worker 脚本

以下示例展示了如何创建自己的 worker 脚本而不依赖第三方库：

```php
<?php
// public/index.php

// 启动你的应用程序
require __DIR__.'/vendor/autoload.php';

$myApp = new \App\Kernel();
$myApp->boot();

// 在循环外的处理器以获得更好的性能（减少工作量）
$handler = static function () use ($myApp) {
    try {
        // 当收到请求时调用，
        // 超全局变量、php://input 等都会被重置
        echo $myApp->handle($_GET, $_POST, $_COOKIE, $_FILES, $_SERVER);
    } catch (\Throwable $exception) {
        // `set_exception_handler` 仅在 worker 脚本结束时调用，
        // 这可能不是您所期望的，因此在此处捕获并处理异常
        (new \MyCustomExceptionHandler)->handleException($exception);
    }
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

```bash
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

默认情况下，每个 CPU 启动 2 个 worker。
你也可以配置要启动的 worker 数量：

```bash
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php 42" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### 使用 PSR-15

如果你的应用程序使用 [PSR-15](https://www.php-fig.org/psr/psr-15/) 而不是超全局变量，请使用 [`nyholm/psr7`](https://github.com/Nyholm/psr7) 和 [`nyholm/psr7-server`](https://github.com/Nyholm/psr7-server) 在处理器的边缘转换请求/响应：

```console
composer require nyholm/psr7 nyholm/psr7-server psr/http-server-handler
```

```php
<?php
// public/index.php

require __DIR__.'/vendor/autoload.php';

use Nyholm\Psr7\Factory\Psr17Factory;
use Nyholm\Psr7Server\ServerRequestCreator;

$myApp = new \App\Kernel(); // implements Psr\Http\Server\RequestHandlerInterface
$myApp->boot();

$psr17Factory = new Psr17Factory();
$creator = new ServerRequestCreator(
    $psr17Factory, // ServerRequestFactory
    $psr17Factory, // UriFactory
    $psr17Factory, // UploadedFileFactory
    $psr17Factory, // StreamFactory
);

$handler = static function () use ($myApp, $creator) {
    $response = $myApp->handle($creator->fromGlobals());

    http_response_code($response->getStatusCode());
    foreach ($response->getHeaders() as $name => $values) {
        foreach ($values as $value) {
            header("$name: $value", false);
        }
    }
    echo $response->getBody();
};

$maxRequests = (int)($_SERVER['MAX_REQUESTS'] ?? 0);
for ($nbRequests = 0; !$maxRequests || $nbRequests < $maxRequests; ++$nbRequests) {
    $keepRunning = \frankenphp_handle_request($handler);
    gc_collect_cycles();
    if (!$keepRunning) break;
}
```

### 在处理一定数量的请求后重启 worker

由于 PHP 最初不是为长时间运行的进程而设计的，许多库和传统代码仍可能导致内存泄漏。
在 worker 模式下使用此类代码的一个解决方法是在处理一定数量的请求后重启 worker 脚本：

前面的 worker 代码片段允许通过设置名为 `MAX_REQUESTS` 的环境变量来配置要处理的最大请求数。

### 手动重启 workers

虽然可以在[文件更改时重启 workers](config.md#watching-for-file-changes)，但也可以通过 [Caddy admin API](https://caddyserver.com/docs/api) 优雅地重启所有 workers。如果在你的 [Caddyfile](config.md#caddyfile-config) 中启用了 admin，你可以通过简单的 POST 请求 ping 重启端点，如下所示：

```bash
curl -X POST http://localhost:2019/frankenphp/workers/restart
```

### Worker 故障

如果 worker 脚本因非零退出代码而崩溃，FrankenPHP 将使用指数退避策略重启它。
如果 worker 脚本保持运行的时间超过上次退避 × 2，
它将不会惩罚 worker 脚本并再次重启它。
但是，如果 worker 脚本在短时间内继续以非零退出代码失败
（例如，脚本中有拼写错误），FrankenPHP 将崩溃并出现错误：`too many consecutive failures`。

可以在你的 [Caddyfile](config.md#caddyfile-config) 中使用 `max_consecutive_failures` 选项配置连续失败的次数：

```caddyfile
frankenphp {
    worker {
        # ...
        max_consecutive_failures 10
    }
}
```

### 线程数

当 worker 模式启用时，需要一个额外的线程来处理传入请求，
因此默认的 `num_threads` 值为 `1 + 2x(可用 CPU 数量)`。

因此，在该模式下，如果 `num_threads` 和 `num` 被显式设置，则 `num_threads` 必须严格大于 `num`。

## 超全局变量行为

[PHP 超全局变量](https://www.php.net/manual/language.variables.superglobals.php)（`$_SERVER`、`$_ENV`、`$_GET`...）
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

大多数超全局变量（`$_GET`、`$_POST`、`$_COOKIE`、`$_FILES`、`$_SERVER`、`$_REQUEST`）会在请求之间自动重置。
然而，**`$_ENV` 当前不会在请求之间重置**。
这意味着在请求期间对 `$_ENV` 所做的任何修改都将持续存在，并对同一 worker 线程处理的后续请求可见。
避免在 `$_ENV` 中存储与请求相关或敏感的数据。

## 状态持久性

由于 worker 模式使 PHP 进程在请求之间保持活动状态，以下状态会在请求之间持久化：

- **静态变量**：函数或方法中用 `static` 声明的变量在请求之间保持其值。
- **类静态属性**：类的静态属性在请求之间持久化。
- **全局变量**：worker 脚本全局范围内的变量在请求之间持久化。
- **内存缓存**：在请求处理器之外存储在内存中的任何数据（数组、对象）都将持久化。
- **运行时引擎设置**：使用 `ini_set()`、`stream_context_set_default()`、`date_default_timezone_set()`、`chdir()`、`stream_wrapper_register()`、`set_error_handler()` 或 `set_exception_handler()` 进行的更改对后续请求仍然有效。切勿在此处存储与请求或用户相关的特定值，例如默认流上下文中的凭据：请为每个调用传递一个显式上下文。
- **进程范围设置**：`umask()` 和 `setlocale()` 会更改整个 FrankenPHP 进程，而不仅仅是当前线程。Worker 模式不会在请求之间恢复它们，并且在任何模式下它们也会影响同时运行的请求。请在启动时设置一次，或者对创建的文件调用 `chmod()` 而不是更改 umask。

这是设计使然，也是 worker 模式快速的原因。然而，它需要注意避免意外的副作用：

```php
<?php
function getCounter(): int {
    static $count = 0;
    return ++$count; // 在请求之间递增！
}

$handler = static function () {
    echo getCounter(); // 此线程上的每个请求分别为 1、2、3...
};

while (\frankenphp_handle_request($handler)) {
    // ...
}
```

在编写 worker 脚本时，请确保在请求之间重置任何与请求相关的状态。
像 [Symfony](symfony.md) 和 [Laravel Octane](laravel.md) 这样的框架会为你处理大部分状态重置，但你可能仍然需要重置自己的服务。对于 Symfony，持有与请求相关状态的服务应实现 [`Symfony\Contracts\Service\ResetInterface`](https://github.com/symfony/contracts/blob/main/Service/ResetInterface.php)，以便它们在请求之间由内核重置。
