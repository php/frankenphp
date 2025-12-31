# 高效服务大型静态文件 (`X-Sendfile`/`X-Accel-Redirect`)

通常，静态文件可以直接由 Web 服务器提供服务，
但有时在发送它们之前需要执行一些 PHP 代码：
访问控制、统计、自定义 HTTP 头...

不幸的是，与直接使用 Web 服务器相比，使用 PHP 服务大型静态文件效率低下
（内存过载、性能降低...）。

FrankenPHP 让你在执行自定义 PHP 代码**之后**将静态文件的发送委托给 Web 服务器。

为此，你的 PHP 应用程序只需定义一个包含要服务的文件路径的自定义 HTTP 头。FrankenPHP 处理其余部分。

此功能在 Apache 中称为 **`X-Sendfile`**，在 NGINX 中称为 **`X-Accel-Redirect`**。

在以下示例中，我们假设项目的文档根目录是 `public/` 目录，
并且我们想要使用 PHP 来服务存储在 `public/` 目录外的文件，
来自名为 `private-files/` 的目录。

## 配置

首先，将以下配置添加到你的 `Caddyfile` 以启用此功能：

```patch
	root public/
	# ...

+	# Symfony、Laravel 和其他使用 Symfony HttpFoundation 组件的项目需要
+	request_header X-Sendfile-Type x-accel-redirect
+	request_header X-Accel-Mapping ../private-files=/private-files
+
+	intercept {
+		@accel header X-Accel-Redirect *
+		handle_response @accel {
+			root private-files/
+			rewrite * {resp.header.X-Accel-Redirect}
+			method * GET
+
+			# 删除 PHP 设置的 X-Accel-Redirect 头以提高安全性
+			header -X-Accel-Redirect
+
+			file_server
+		}
+	}

	php_server
```

## 纯 PHP

将相对文件路径（从 `private-files/`）设置为 `X-Accel-Redirect` 头的值：

```php
header('X-Accel-Redirect: file.txt');
```

## 使用 Symfony HttpFoundation 组件的项目（Symfony、Laravel、Drupal...）

Symfony HttpFoundation [原生支持此功能](https://symfony.com/doc/current/components/http_foundation.html#serving-files)。
它将自动确定 `X-Accel-Redirect` 头的正确值并将其添加到响应中。

```php
use Symfony\Component\HttpFoundation\BinaryFileResponse;

BinaryFileResponse::trustXSendfileTypeHeader();
$response = new BinaryFileResponse(__DIR__.'/../private-files/file.txt');

// ...
```
