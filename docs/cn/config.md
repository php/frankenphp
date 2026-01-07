# 配置

FrankenPHP、Caddy 以及 [Mercure](mercure.md) 和 [Vulcain](https://vulcain.rocks) 模块可以使用 [Caddy 支持的格式](https://caddyserver.com/docs/getting-started#your-first-config) 进行配置。

最常见的格式是 `Caddyfile`，它是一种简单、人类可读的文本格式。默认情况下，FrankenPHP 会在当前目录中查找 `Caddyfile`。你可以使用 `-c` 或 `--config` 选项指定自定义路径。

以下显示了一个用于服务 PHP 应用程序的最小 `Caddyfile`：

```caddyfile
# 响应的主机名
localhost

# （可选）提供文件服务的目录，否则默认为当前目录
#root public/
php_server
```

FrankenPHP 仓库中提供了 [一个更高级的 `Caddyfile`](https://github.com/php/frankenphp/blob/main/caddy/frankenphp/Caddyfile)，该文件支持更多功能并提供方便的环境变量，Docker 镜像也附带了该文件。

PHP 本身可以[使用 `php.ini` 文件](https://www.php.net/manual/zh/configuration.file.php)进行配置。

根据你的安装方法，FrankenPHP 和 PHP 解释器将在下面描述的位置查找配置文件。

## Docker

FrankenPHP:

- `/etc/frankenphp/Caddyfile`：主配置文件
- `/etc/frankenphp/Caddyfile.d/*.caddyfile`：自动加载的附加配置文件

PHP:

- `php.ini`: `/usr/local/etc/php/php.ini`（默认情况下不提供 `php.ini`）
- 附加配置文件: `/usr/local/etc/php/conf.d/*.ini`
- PHP 扩展: `/usr/local/lib/php/extensions/no-debug-zts-<YYYYMMDD>/`
- 你应该复制 PHP 项目提供的官方模板：

```dockerfile
FROM dunglas/frankenphp

# 生产环境:
RUN cp $PHP_INI_DIR/php.ini-production $PHP_INI_DIR/php.ini

# 或开发环境:
RUN cp $PHP_INI_DIR/php.ini-development $PHP_INI_DIR/php.ini
```

## RPM 和 Debian 包

FrankenPHP:

- `/etc/frankenphp/Caddyfile`：主配置文件
- `/etc/frankenphp/Caddyfile.d/*.caddyfile`：自动加载的附加配置文件

PHP:

- `php.ini`: `/etc/php-zts/php.ini`（默认情况下提供带有生产预设的 `php.ini` 文件）
- 附加配置文件: `/etc/php-zts/conf.d/*.ini`

## 静态二进制文件

FrankenPHP:

- 在当前工作目录中：`Caddyfile`

PHP:

- `php.ini`: 执行 `frankenphp run` 或 `frankenphp php-server` 的目录，然后是 `/etc/frankenphp/php.ini`
- 附加配置文件: `/etc/frankenphp/php.d/*.ini`
- PHP 扩展: 无法加载，将它们打包在二进制文件本身中
- 复制 [PHP 源代码](https://github.com/php/php-src/) 中提供的 `php.ini-production` 或 `php.ini-development` 中的一个。

## Caddyfile 配置

可以在站点块中使用 `php_server` 或 `php` [HTTP 指令](https://caddyserver.com/docs/caddyfile/concepts#directives) 来为你的 PHP 应用程序提供服务。

最小示例：

```caddyfile
localhost {
	# 启用压缩（可选）
	encode zstd br gzip
	# 在当前目录中执行 PHP 文件并提供资源服务
	php_server
}
```

你还可以使用 `frankenphp` [全局选项](https://caddyserver.com/docs/caddyfile/concepts#global-options) 显式配置 FrankenPHP：

```caddyfile
{
	frankenphp {
		num_threads <num_threads> # 设置要启动的 PHP 线程数量。默认：可用 CPU 数量的 2 倍。
		max_threads <num_threads> # 限制可以在运行时启动的额外 PHP 线程的数量。默认值：num_threads。可以设置为 'auto'。
		max_wait_time <duration> # 设置请求在超时之前可以等待的最大时间，直到找到一个空闲的 PHP 线程。 默认：禁用。
		php_ini <key> <value> # 设置一个 php.ini 指令。可以多次使用以设置多个指令。
		worker {
			file <path> # 设置工作脚本的路径。
			num <num> # 设置要启动的 PHP 线程数量，默认为可用 CPU 数量的 2 倍。
			env <key> <value> # 设置一个额外的环境变量为给定的值。可以多次指定以设置多个环境变量。
			watch <path> # 设置要监视文件更改的路径。可以为多个路径多次指定。
			name <name> # 设置worker的名称，用于日志和指标。默认值：worker文件的绝对路径。
			max_consecutive_failures <num> # 设置在工人被视为不健康之前的最大连续失败次数，-1意味着工人将始终重新启动。默认值：6。
		}
	}
}

# ...
```

或者，您可以使用 `worker` 选项的一行简短形式：

```caddyfile
{
	frankenphp {
		worker <file> <num>
	}
}

# ...
```

如果您在同一服务器上服务多个应用程序，您还可以定义多个工作线程：

```caddyfile
app.example.com {
    root /path/to/app/public
	php_server {
		root /path/to/app/public # 允许更好的缓存
		worker index.php <num>
	}
}

other.example.com {
    root /path/to/other/public
	php_server {
		root /path/to/other/public
		worker index.php <num>
	}
}

# ...
```

使用 `php_server` 指令通常是您需要的。
但是如果你需要完全控制，你可以使用更低级的 `php` 指令。
`php` 指令将所有输入传递给 PHP，而不是先检查是否
是一个PHP文件。在[性能页面](performance.md#try_files)中了解更多关于它的信息。

使用 `php_server` 指令等同于以下配置：

```caddyfile
route {
	# 为目录请求添加尾斜杠
	@canonicalPath {
		file {path}/index.php
		not path */
	}
	redir @canonicalPath {path}/ 308
	# 如果请求的文件不存在，则尝试 index 文件
	@indexFiles file {
		try_files {path} {path}/index.php index.php
		split_path .php
	}
	rewrite @indexFiles {http.matchers.file.relative}
	# FrankenPHP!
	@phpFiles path *.php
	php @phpFiles
	file_server
}
```

`php_server` 和 `php` 指令有以下选项：

```caddyfile
php_server [<matcher>] {
	root <directory> # 将根文件夹设置为站点。默认值：`root` 指令。
	split_path <delim...> # 设置用于将 URI 分割成两部分的子字符串。第一个匹配的子字符串将用来将 "路径信息" 与路径分开。第一部分后缀为匹配的子字符串，并将被视为实际资源（CGI 脚本）名称。第二部分将被设置为脚本使用的 PATH_INFO。默认值：`.php`。
	resolve_root_symlink false # 禁用通过评估符号链接（如果存在）将 `root` 目录解析为其实际值（默认启用）。
	env <key> <value> # 设置一个额外的环境变量为给定的值。可以多次指定以设置多个环境变量。
	file_server off # 禁用内置的 file_server 指令。
	worker { # 为此服务器创建特定的worker。可以多次指定以创建多个workers。
		file <path> # 设置工作脚本的路径，可以相对于 php_server 根目录
		num <num> # 设置要启动的 PHP 线程数，默认为可用 CPU 数量的 2 倍
		name <name> # 为worker设置名称，用于日志和指标。默认值：worker文件的绝对路径。定义在 php_server 块中时，始终以 m# 开头。
		watch <path> # 设置要监视文件更改的路径。可以为多个路径多次指定。
		env <key> <value> # 设置一个额外的环境变量为给定值。可以多次指定以设置多个环境变量。此工作进程的环境变量也从 php_server 父进程继承，但可以在此处覆盖。
		match <path> # 将worker匹配到路径模式。覆盖 try_files，并且只能在 php_server 指令中使用。
	}
	worker <other_file> <num> # 也可以像在全局 frankenphp 块中那样使用简短形式。
}
```

### 监控文件变化

由于 workers 只会启动您的应用程序一次并将其保留在内存中，
因此对您的 PHP 文件的任何更改不会立即反映出来。

工作线程可以通过 `watch` 指令在文件更改时重新启动。
这对开发环境很有用。

```caddyfile
{
	frankenphp {
		worker {
			file  /path/to/app/public/worker.php
			watch
		}
	}
}
```

此功能通常与 [热重载](hot-reload.md) 结合使用。

如果没有指定 `watch` 目录，它将回退到 `./**/*.{env,php,twig,yaml,yml}`，
这将监视启动 FrankenPHP 进程的目录及其子目录中的所有 `.env`、`.php`、`.twig`、`.yaml` 和 `.yml` 文件。
你也可以通过 [shell 文件名模式](https://pkg.go.dev/path/filepath#Match) 指定一个或多个目录：

```caddyfile
{
	frankenphp {
		worker {
			file  /path/to/app/public/worker.php
			watch /path/to/app # 监视 /path/to/app 所有子目录中的所有文件
			watch /path/to/app/*.php # 监视位于/path/to/app中的以.php结尾的文件
			watch /path/to/app/**/*.php # 监视 /path/to/app 及子目录中的 PHP 文件
			watch /path/to/app/**/*.{php,twig} # 在/path/to/app及其子目录中监视PHP和Twig文件
		}
	}
}
```

- `**` 模式表示递归监视
- 目录也可以是相对的（相对于FrankenPHP进程启动的位置）
- 如果您定义了多个workers，当文件发生更改时，将重新启动所有workers。
- 小心查看在运行时创建的文件（如日志），因为它们可能导致不必要的工作进程重启。

文件监视器基于[e-dant/watcher](https://github.com/e-dant/watcher)。

## 将 worker 匹配到一条路径

在传统的PHP应用程序中，脚本总是放在公共目录中。
这对于工作脚本也是如此，这些脚本被视为任何其他PHP脚本。
如果您想将工作脚本放在公共目录外，可以通过 `match` 指令来实现。

`match` 指令是 `try_files` 的一种优化替代方案，仅在 `php_server` 和 `php` 内部可用。
以下示例将在公共目录中提供文件（如果存在）
并将请求转发给与路径模式匹配的 worker。

```caddyfile
{
	frankenphp {
		php_server {
			worker {
				file /path/to/worker.php # 文件可以在公共路径之外
				match /api/* # 所有以 /api/ 开头的请求将由此 worker 处理
			}
		}
	}
}
```

## 环境变量

可以使用以下环境变量在不修改 `Caddyfile` 的情况下注入 Caddy 指令：

- `SERVER_NAME`: 更改[监听的地址](https://caddyserver.com/docs/caddyfile/concepts#addresses)，提供的宿主名也将用于生成的TLS证书。
- `SERVER_ROOT`: 更改网站的根目录，默认为 `public/`
- `CADDY_GLOBAL_OPTIONS`: 注入[全局选项](https://caddyserver.com/docs/caddyfile/options)
- `FRANKENPHP_CONFIG`: 在 `frankenphp` 指令下注入配置

至于 FPM 和 CLI SAPIs，环境变量默认在 `$_SERVER` 超全局中暴露。

[the `variables_order` PHP 指令](https://www.php.net/manual/zh/ini.core.php#ini.variables-order) 的 `S` 值始终等于 `ES`，无论 `E` 在该指令中的其他位置如何。

## PHP 配置

要加载[附加的 PHP 配置文件](https://www.php.net/manual/zh/configuration.file.php#configuration.file.scan)，
可以使用 `PHP_INI_SCAN_DIR` 环境变量。
设置后，PHP 将加载给定目录中所有带有 `.ini` 扩展名的文件。

您还可以通过在 `Caddyfile` 中使用 `php_ini` 指令来更改 PHP 配置:

```caddyfile
{
    frankenphp {
        php_ini memory_limit 256M

        # 或者

        php_ini {
            memory_limit 256M
            max_execution_time 15
        }
    }
}
```

### 禁用 HTTPS

默认情况下，FrankenPHP 会为所有主机名（包括 `localhost`）自动启用 HTTPS。如果你想禁用 HTTPS（例如在开发环境中），你可以将 `SERVER_NAME` 环境变量设置为 `http://` 或 `:80`：

另外，你可以使用 [Caddy 文档](https://caddyserver.com/docs/automatic-https#activation) 中描述的所有其他方法。

如果你想使用 `127.0.0.1` IP 地址而不是 `localhost` 主机名来使用 HTTPS，请阅读 [已知问题](known-issues.md#using-https127001-with-docker) 部分。

### 全双工 (HTTP/1)

在使用 HTTP/1.x 时，可能希望启用全双工模式，以允许在读取整个请求体之前写入响应。（例如：[Mercure](mercure.md)、WebSocket、Server-Sent Events 等）

这是一个可选配置，需要添加到 `Caddyfile` 中的全局选项中:

```caddyfile
{
  servers {
    enable_full_duplex
  }
}
```

> [!CAUTION]
>
> 启用此选项可能导致不支持全双工的旧 HTTP/1.x 客户端死锁。
> 这也可以通过 `CADDY_GLOBAL_OPTIONS` 环境变量进行配置：

```sh
CADDY_GLOBAL_OPTIONS="servers {
  enable_full_duplex
}"
```

您可以在[Caddy文档](https://caddyserver.com/docs/caddyfile/options#enable-full-duplex)中找到有关此设置的更多信息。

## 启用调试模式

使用Docker镜像时，将`CADDY_GLOBAL_OPTIONS`环境变量设置为`debug`以启用调试模式:

```console
docker run -v $PWD:/app/public \
    -e CADDY_GLOBAL_OPTIONS=debug \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```
