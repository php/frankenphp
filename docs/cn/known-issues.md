# 已知问题

## 不支持的 PHP 扩展

已知以下扩展与 FrankenPHP 不兼容：

| 名称                                                                                                        | 原因          | 替代方案                                                                                                         |
| ----------------------------------------------------------------------------------------------------------- | --------------- | -------------------------------------------------------------------------------------------------------------------- |
| [imap](https://www.php.net/manual/en/imap.installation.php)                                                 | 不安全的线程 | [javanile/php-imap2](https://github.com/javanile/php-imap2), [webklex/php-imap](https://github.com/Webklex/php-imap) |
| [newrelic](https://docs.newrelic.com/docs/apm/agents/php-agent/getting-started/introduction-new-relic-php/) | 不安全的线程 | -                                                                                                                    |

## 有缺陷的 PHP 扩展

以下扩展在与 FrankenPHP 一起使用时已知存在错误和意外行为：

| 名称                                                          | 问题                                                                                                                                                                                                                                                                                         |
| ------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [ext-openssl](https://www.php.net/manual/en/book.openssl.php) | 在使用静态构建的 FrankenPHP（使用 musl libc 构建）时，在重负载下 OpenSSL 扩展可能会崩溃。一个解决方法是使用动态链接的构建（如 Docker 镜像中使用的版本）。此错误正在由 PHP 跟踪。[查看问题](https://github.com/php/php-src/issues/13648)。 |

## get_browser

[get_browser()](https://www.php.net/manual/en/function.get-browser.php) 函数在一段时间后似乎表现不佳。解决方法是缓存（例如使用 [APCu](https://www.php.net/manual/zh/book.apcu.php)）每个 User-Agent，因为它们是不变的。

## 独立的二进制和基于 Alpine 的 Docker 镜像

独立的二进制文件和基于 Alpine 的 Docker 镜像 (`dunglas/frankenphp:*-alpine`) 使用的是 [musl libc](https://musl.libc.org/) 而不是 [glibc and friends](https://www.etalabs.net/compare_libcs.html)，为的是保持较小的二进制大小。这可能会导致一些兼容性问题。特别是，glob 标志 `GLOB_BRACE` [不可用](https://www.php.net/manual/en/function.glob.php)。

## 在 Docker 中使用 `https://127.0.0.1`

默认情况下，FrankenPHP 会为 `localhost` 生成一个 TLS 证书。
这是本地开发最简单且推荐的选项。

如果确实想使用 `127.0.0.1` 作为主机，可以通过将服务器名称设置为 `127.0.0.1` 来配置它以为其生成证书。

如果你使用 Docker，因为 [Docker 网络](https://docs.docker.com/network/) 问题，只做这些是不够的。
你将收到类似于以下内容的 TLS 错误 `curl: (35) LibreSSL/3.3.6: error:1404B438:SSL routines:ST_CONNECT:tlsv1 alert internal error`。

如果你使用的是 Linux，解决方案是使用 [使用宿主机网络](https://docs.docker.com/network/network-tutorial-host/)：

```console
docker run \
    -e SERVER_NAME="127.0.0.1" \
    -v $PWD:/app/public \
    --network host \
    dunglas/frankenphp
```

Mac 和 Windows 不支持 Docker 使用宿主机网络。在这些平台上，你必须猜测容器的 IP 地址并将其包含在服务器名称中。

运行 `docker network inspect bridge` 并查看 `Containers`，找到 `IPv4Address` 当前分配的最后一个 IP 地址，并增加 1。如果没有容器正在运行，则第一个分配的 IP 地址通常为 `172.17.0.2`。

然后将其包含在 `SERVER_NAME` 环境变量中：

```console
docker run \
    -e SERVER_NAME="127.0.0.1, 172.17.0.3" \
    -v $PWD:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

> [!CAUTION]
>
> 请务必将 `172.17.0.3` 替换为将分配给容器的 IP。

你现在应该能够从主机访问 `https://127.0.0.1`。

如果不是这种情况，请在调试模式下启动 FrankenPHP 以尝试找出问题：

```console
docker run \
    -e CADDY_GLOBAL_OPTIONS="debug" \
    -e SERVER_NAME="127.0.0.1" \
    -v $PWD:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

## Composer 脚本引用 `@php`

[Composer 脚本](https://getcomposer.org/doc/articles/scripts.md) 可能想要执行一个 PHP 二进制文件来完成一些任务，例如在 [Laravel 项目](laravel.md) 中运行 `@php artisan package:discover --ansi`。这 [目前失败](https://github.com/php/frankenphp/issues/483#issuecomment-1899890915) 的原因有两个：

- Composer 不知道如何调用 FrankenPHP 二进制文件；
- Composer 可以在命令中使用 `-d` 标志添加 PHP 设置，而 FrankenPHP 目前尚不支持。

作为一种变通方法，我们可以在 `/usr/local/bin/php` 中创建一个 Shell 脚本，该脚本会去掉不支持的参数，然后调用 FrankenPHP:

```bash
#!/usr/bin/env bash
args=("$@")
index=0
for i in "$@"
do
    if [ "$i" == "-d" ]; then
        unset 'args[$index]'
        unset 'args[$index+1]'
    fi
    index=$((index+1))
done

/usr/local/bin/frankenphp php-cli ${args[@]}
```

然后将环境变量 `PHP_BINARY` 设置为我们 `php` 脚本的路径，并运行 Composer：

```console
export PHP_BINARY=/usr/local/bin/php
composer install
```

## 使用静态二进制文件排查 TLS/SSL 问题

在使用静态二进制文件时，您可能会遇到以下与TLS相关的错误，例如在使用STARTTLS发送电子邮件时：

```text
Unable to connect with STARTTLS: stream_socket_enable_crypto(): SSL operation failed with code 5. OpenSSL Error messages:
error:80000002:system library::No such file or directory
error:80000002:system library::No such file or directory
error:80000002:system library::No such file or directory
error:0A000086:SSL routines::certificate verify failed
```

由于静态二进制不捆绑 TLS 证书，因此您需要将 OpenSSL 指向本地 CA 证书安装。

检查 [`openssl_get_cert_locations()`](https://www.php.net/manual/en/function.openssl-get-cert-locations.php) 的输出，
以找到 CA 证书必须安装的位置，并将它们存储在该位置。

> [!WARNING]
>
> Web 和命令行界面可能有不同的设置。
> 确保在适当的上下文中运行 `openssl_get_cert_locations()`。

[从Mozilla提取的CA证书可以在curl网站上下载](https://curl.se/docs/caextract.html)。

或者，许多发行版，包括 Debian、Ubuntu 和 Alpine，提供名为 `ca-certificates` 的软件包，其中包含这些证书。

还可以使用 `SSL_CERT_FILE` 和 `SSL_CERT_DIR` 来提示 OpenSSL 在哪里查找 CA 证书：

```console
# Set TLS certificates environment variables
export SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
export SSL_CERT_DIR=/etc/ssl/certs
```
