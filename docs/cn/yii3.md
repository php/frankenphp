---
title: 使用 FrankenPHP 运行 Yii3（Docker，worker 模式）
description: 如何使用 Docker 镜像、本地安装或 yii-runner-frankenphp 包的 worker 模式来使用 FrankenPHP 运行 Yii3 应用程序。
---

# Yii3

## 使用 FrankenPHP Docker 镜像运行 Yii3

使用 FrankenPHP 提供 [Yii](https://www.yiiframework.com/) Web 应用程序就像将项目挂载到官方 Docker 镜像的 `/app` 目录一样简单。

在您的 Yii 应用程序的主目录中运行此命令：

```console
docker run -p 80:80 -p 443:443 -p 443:443/udp -v $PWD:/app dunglas/frankenphp
```

尽情享受吧！

## 本地安装 FrankenPHP 运行 Yii3

或者，您可以在本地机器上使用 FrankenPHP 运行 Yii 项目：

1.  [下载与您的系统对应的二进制文件](../#standalone-binary)
2.  将以下配置添加到您的 Yii 项目根目录中名为 `Caddyfile` 的文件中：

    ```caddyfile
    {
    	frankenphp
    }

    # 您服务器的域名
    localhost {
    	# 启用压缩（可选）
    	encode zstd br gzip
    	# 从 public/ 目录执行 PHP 文件并提供静态资源
    	php_server {
    		root public/
    		try_files {path} index.php
    	}
    }
    ```

3.  从您的 Yii 项目的根目录启动 FrankenPHP：`frankenphp run`

## Yii3 worker 模式

要在 [worker 模式](worker.md)下运行您的 Yii 应用程序，请安装 [Yii FrankenPHP runner](https://github.com/yiisoft/yii-runner-frankenphp)：

```console
composer require yiisoft/yii-runner-frankenphp
```

然后，在您的应用程序的根目录中创建一个名为 `worker.php` 的文件：

```php
<?php

declare(strict_types=1);

use Yiisoft\Yii\Runner\FrankenPHP\FrankenPHPApplicationRunner;

require_once __DIR__ . '/vendor/autoload.php';

(new FrankenPHPApplicationRunner(rootPath: __DIR__))->run();
```

更新您的 `Caddyfile` 以在 worker 模式下启动应用程序：

```caddyfile
{
	frankenphp
}

localhost {
	encode zstd br gzip
	php_server {
		root public/
		worker ./worker.php {
			# 将所有请求发送到 worker
			match *
			# 当 PHP 文件发生变化时重新加载 worker（仅限开发环境）
			watch ./**/*.php
		}
	}
}
```

如果您的应用程序基于 [官方 Yii 应用程序模板](https://github.com/yiisoft/app)，请参阅[包的 README](https://github.com/yiisoft/yii-runner-frankenphp) 以获取包含调试、环境和错误处理程序配置的完整 `worker.php` 示例。

要限制 worker 在重启前处理的请求数量（有助于缓解内存泄漏），请设置 `MAX_REQUESTS` 环境变量。默认情况下，worker 无限期地处理请求。

使用 worker 模式时，请确保有状态服务在每次请求后都会重置。有关详细信息，请参阅 [worker 模式文档](worker.md) 和 [Yii DI `StateResetter` 文档](https://github.com/yiisoft/di#resetting-services-state)。
