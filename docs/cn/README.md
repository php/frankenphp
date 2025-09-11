# FrankenPHP: 适用于 PHP 的现代应用服务器

<h1 align="center"><a href="https://frankenphp.dev"><img src="../../frankenphp.png" alt="FrankenPHP" width="600"></a></h1>

FrankenPHP 是建立在 [Caddy](https://caddyserver.com/) Web 服务器之上的现代 PHP 应用程序服务器。

FrankenPHP 凭借其令人惊叹的功能为你的 PHP 应用程序提供了超能力：[早期提示](early-hints.md)、[worker 模式](worker.md)、[实时功能](mercure.md)、自动 HTTPS、HTTP/2 和 HTTP/3 支持......

FrankenPHP 可与任何 PHP 应用程序一起使用，并且由于提供了与 worker 模式的集成，使你的 Symfony 和 Laravel 项目比以往任何时候都更快。

FrankenPHP 也可以用作独立的 Go 库，将 PHP 嵌入到任何使用 `net/http` 的应用程序中。

[**了解更多** _frankenphp.dev_](https://frankenphp.dev/cn/) 以及查看此演示文稿：

<a href="https://dunglas.dev/2022/10/frankenphp-the-modern-php-app-server-written-in-go/"><img src="https://dunglas.dev/wp-content/uploads/2022/10/frankenphp.png" alt="Slides" width="600"></a>

## 开始

### 独立二进制

我们为 Linux 和 macOS 提供包含 [PHP 8.4](https://www.php.net/releases/8.4/zh.php) 以及大多数常用 PHP 扩展的 FrankenPHP 静态二进制文件。

在 Windows 上，请使用 [WSL](https://learn.microsoft.com/windows/wsl/) 运行 FrankenPHP。

你可以 [下载 FrankenPHP](https://github.com/dunglas/frankenphp/releases)，或将以下命令复制到终端中，自动安装适用于你平台的版本：

```console
curl https://frankenphp.dev/install.sh | sh
mv frankenphp /usr/local/bin/
```

要提供当前目录的内容，请运行：

```console
frankenphp php-server
```

你还可以使用以下命令运行命令行脚本：

```console
frankenphp php-cli /path/to/your/script.php
```

### Docker

此外，还可以使用 [Docker 镜像](https://frankenphp.dev/docs/docker/)：

```console
docker run -v .:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

访问 `https://localhost`, 并享受吧!

> [!TIP]
>
> 不要尝试使用 `https://127.0.0.1`。使用 `https://localhost` 并接受自签名证书。
> 使用 [`SERVER_NAME` 环境变量](config.md#environment-variables) 更改要使用的域。

### Homebrew

FrankenPHP 也作为 [Homebrew](https://brew.sh) 软件包提供，适用于 macOS 和 Linux 系统。

安装方法：

```console
brew install dunglas/frankenphp/frankenphp
```

要提供当前目录的内容，请运行：

```console
frankenphp php-server
```

## 文档

- [Classic 模式](classic.md)
- [worker 模式](worker.md)
- [早期提示支持(103 HTTP status code)](early-hints.md)
- [实时功能](mercure.md)
- [高效地服务大型静态文件](x-sendfile.md)
- [配置](config.md)
- [用 Go 编写 PHP 扩展](extensions.md)
- [Docker 镜像](docker.md)
- [在生产环境中部署](production.md)
- [性能优化](performance.md)
- [创建独立、可自行执行的 PHP 应用程序](embed.md)
- [创建静态二进制文件](static.md)
- [从源代码编译](compile.md)
- [Laravel 集成](laravel.md)
- [已知问题](known-issues.md)
- [演示应用程序 (Symfony) 和性能测试](https://github.com/dunglas/frankenphp-demo)
- [Go 库文档](https://pkg.go.dev/github.com/dunglas/frankenphp)
- [贡献和调试](https://frankenphp.dev/docs/contributing/)

## 示例和框架

- [Symfony](https://github.com/dunglas/symfony-docker)
- [API Platform](https://api-platform.com/docs/distribution/)
- [Laravel](laravel.md)
- [Sulu](https://sulu.io/blog/running-sulu-with-frankenphp)
- [WordPress](https://github.com/StephenMiracle/frankenwp)
- [Drupal](https://github.com/dunglas/frankenphp-drupal)
- [Joomla](https://github.com/alexandreelise/frankenphp-joomla)
- [TYPO3](https://github.com/ochorocho/franken-typo3)
- [Magento2](https://github.com/ekino/frankenphp-magento2)
