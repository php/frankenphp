# 性能

默认情况下，FrankenPHP 尝试在性能和易用性之间提供良好的折衷。
但是，通过使用适当的配置，可以大幅提高性能。

## 线程和 Worker 数量

默认情况下，FrankenPHP 启动的线程和 worker（在 worker 模式下）数量是可用 CPU 数量的 2 倍。

适当的值很大程度上取决于你的应用程序是如何编写的、它做什么以及你的硬件。
我们强烈建议更改这些值。为了获得最佳的系统稳定性，建议 `num_threads` x `memory_limit` < `available_memory`。

要找到正确的值，最好运行模拟真实流量的负载测试。
[k6](https://k6.io) 和 [Gatling](https://gatling.io) 是很好的工具。

要配置线程数，请使用 `php_server` 和 `php` 指令的 `num_threads` 选项。
要更改 worker 数量，请使用 `frankenphp` 指令的 `worker` 部分的 `num` 选项。

### `max_threads`

虽然准确了解你的流量情况总是更好，但现实应用往往更加
不可预测。`max_threads` [配置](config.md#caddyfile-config) 允许 FrankenPHP 在运行时自动生成额外线程，直到指定的限制。
`max_threads` 可以帮助你确定需要多少线程来处理你的流量，并可以使服务器对延迟峰值更具弹性。
如果设置为 `auto`，限制将基于你的 `php.ini` 中的 `memory_limit` 进行估算。如果无法这样做，
`auto` 将默认为 2x `num_threads`。请记住，`auto` 可能会严重低估所需的线程数。
`max_threads` 类似于 PHP FPM 的 [pm.max_children](https://www.php.net/manual/en/install.fpm.configuration.php#pm.max-children)。主要区别是 FrankenPHP 使用线程而不是
进程，并根据需要自动在不同的 worker 脚本和"经典模式"之间委派它们。

## Worker 模式

启用 [worker 模式](worker.md) 大大提高了性能，
但你的应用必须适配以兼容此模式：
你需要创建一个 worker 脚本并确保应用不会泄漏内存。

## 不要使用 musl

官方 Docker 镜像的 Alpine Linux 变体和我们提供的默认二进制文件使用 [musl libc](https://musl.libc.org)。

众所周知，当使用这个替代 C 库而不是传统的 GNU 库时，PHP [更慢](https://gitlab.alpinelinux.org/alpine/aports/-/issues/14381)，
特别是在以 ZTS 模式（线程安全）编译时，这是 FrankenPHP 所必需的。在大量线程环境中，差异可能很显著。

另外，[一些错误只在使用 musl 时发生](https://github.com/php/php-src/issues?q=sort%3Aupdated-desc+is%3Aissue+is%3Aopen+label%3ABug+musl)。

在生产环境中，我们建议使用链接到 glibc，并以适当优化级别编译的 FrankenPHP。

这可以通过使用 Debian Docker 镜像、使用我们的维护者提供的 [.deb](https://debs.henderkes.com) 或 [.rpm](https://rpms.henderkes.com) 包，或通过[从源代码编译 FrankenPHP](compile.md) 来实现。

## Go 运行时配置

FrankenPHP 是用 Go 编写的。

一般来说，Go 运行时不需要任何特殊配置，但在某些情况下，
特定的配置可以提高性能。

你可能想要将 `GODEBUG` 环境变量设置为 `cgocheck=0`（FrankenPHP Docker 镜像中的默认值）。

如果你在容器（Docker、Kubernetes、LXC...）中运行 FrankenPHP 并限制容器的可用内存，
请将 `GOMEMLIMIT` 环境变量设置为可用内存量。

有关更多详细信息，[专门针对此主题的 Go 文档页面](https://pkg.go.dev/runtime#hdr-Environment_Variables) 是充分利用运行时的必读内容。

## `file_server`

默认情况下，`php_server` 指令自动设置文件服务器来
提供存储在根目录中的静态文件（资产）。

此功能很方便，但有成本。
要禁用它，请使用以下配置：

```caddyfile
php_server {
    file_server off
}
```

## `try_files`

除了静态文件和 PHP 文件外，`php_server` 还会尝试提供你应用程序的索引
和目录索引文件（`/path/` -> `/path/index.php`）。如果你不需要目录索引，
你可以通过明确定义 `try_files` 来禁用它们，如下所示：

```caddyfile
php_server {
    try_files {path} index.php
    root /root/to/your/app # 在这里明确添加根目录允许更好的缓存
}
```

这可以显著减少不必要的文件操作数量。

另一种具有 0 个不必要文件系统操作的方法是改用 `php` 指令并按路径将
文件与 PHP 分开。如果你的整个应用程序由一个入口文件提供服务，这种方法效果很好。
一个在 `/assets` 文件夹后面提供静态文件的示例[配置](config.md#caddyfile-config)可能如下所示：

```caddyfile
route {
    @assets {
        path /assets/*
    }

    # /assets 后面的所有内容都由文件服务器处理
    file_server @assets {
        root /root/to/your/app
    }

    # 不在 /assets 中的所有内容都由你的索引或 worker PHP 文件处理
    rewrite index.php
    php {
        root /root/to/your/app # 在这里明确添加根目录允许更好的缓存
    }
}
```

## 占位符

你可以在 `root` 和 `env` 指令中使用[占位符](https://caddyserver.com/docs/conventions#placeholders)。
但是，这会阻止缓存这些值，并带来显著的性能成本。

如果可能，请避免在这些指令中使用占位符。

## `resolve_root_symlink`

默认情况下，如果文档根目录是符号链接，FrankenPHP 会自动解析它（这对于 PHP 正常工作是必要的）。
如果文档根目录不是符号链接，你可以禁用此功能。

```caddyfile
php_server {
    resolve_root_symlink false
}
```

如果 `root` 指令包含[占位符](https://caddyserver.com/docs/conventions#placeholders)，这将提高性能。
在其他情况下，收益将可以忽略不计。

## 日志

日志显然非常有用，但根据定义，
它需要 I/O 操作和内存分配，这会大大降低性能。
确保你[正确设置日志级别](https://caddyserver.com/docs/caddyfile/options#log)，
并且只记录必要的内容。

## PHP 性能

FrankenPHP 使用官方 PHP 解释器。
所有常见的 PHP 相关性能优化都适用于 FrankenPHP。

特别是：

- 检查 [OPcache](https://www.php.net/manual/zh/book.opcache.php) 是否已安装、启用并正确配置
- 启用 [Composer 自动加载器优化](https://getcomposer.org/doc/articles/autoloader-optimization.md)
- 确保 `realpath` 缓存对于你的应用程序需求足够大
- 使用[预加载](https://www.php.net/manual/zh/opcache.preloading.php)

有关更多详细信息，请阅读[专门的 Symfony 文档条目](https://symfony.com/doc/current/performance.html)
（即使你不使用 Symfony，大多数提示也很有用）。

## 分割线程池

应用程序通常会与缓慢的外部服务交互，例如在高负载下容易变得不可靠或持续需要 10 秒以上才能响应的 API。
在这种情况下，将线程池拆分以拥有专用的“慢速”池可能会很有益。这可以防止慢速端点耗尽所有服务器资源/线程，并限制指向慢速端点的请求并发性，类似于连接池。

```caddyfile
{
    frankenphp {
        max_threads 100 # 所有 worker 共享最多 100 个线程
    }
}

example.com {
    php_server {
        root /app/public # 你的应用程序的根目录
        worker index.php {
            match /slow-endpoint/* # 所有路径为 /slow-endpoint/* 的请求都由这个线程池处理
            num 10 # 匹配 /slow-endpoint/* 的请求至少有 10 个线程
        }
        worker index.php {
            match * # 所有其他请求单独处理
            num 20 # 其他请求至少有 20 个线程，即使慢速端点开始挂起
        }
    }
}
```

通常，也建议通过使用消息队列等相关机制，异步处理非常慢的端点。
