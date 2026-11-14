---
title: FrankenPHP 已知问题、不兼容的扩展和解决方案
description: FrankenPHP 限制的参考，包括不支持的 PHP 扩展、musl libc 注意事项、使用 127.0.0.1 的 Docker TLS 设置以及 Composer @php 脚本。
---

# 已知问题

## 不支持的 PHP 扩展

已知以下扩展与 FrankenPHP 不兼容：

| 名称                                                                                                        | 原因          | 替代方案                                                                                                                                                                    |
| ----------------------------------------------------------------------------------------------------------- | ------------- |---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [imap](https://www.php.net/manual/imap.installation.php)                                                 | 非线程安全    | [javanile/php-imap2](https://github.com/javanile/php-imap2), [webklex/php-imap](https://github.com/Webklex/php-imap), [ImapEngine](https://github.com/DirectoryTree/ImapEngine) |
| [newrelic](https://docs.newrelic.com/docs/apm/agents/php-agent/getting-started/introduction-new-relic-php/) | 非线程安全    | -                                                                                                                                                                               |
| [pcov](https://github.com/krakjoe/pcov)                                                                     | 非线程安全 ([经典模式下的 SIGSEGV](https://github.com/krakjoe/pcov/issues/132)) | 使用 CLI SAPI 收集覆盖率（在 FrankenPHP 外部运行 PHPUnit/Pest） |

## 有缺陷的 PHP 扩展

以下扩展在与 FrankenPHP 一起使用时已知存在错误和意外行为：

| 名称                                  | 问题                                                                                                                                         |
|---------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------|
| [datadog](https://github.com/DataDog) | 在分析 FrankenPHP 时存在不稳定性。此 bug [正由 DataDog 跟踪](https://github.com/DataDog/dd-trace-php/issues/3729)。 |
| [blackfire](https://blackfire.io/) | [FrankenPHP 支持处于 Beta 阶段，尚未功能完善](https://docs.blackfire.io/php/integrations/frankenphp)。                            |
| [imagick](https://github.com/Imagick/imagick) | ImageMagick 的 OpenMP 线程与 FrankenPHP 的线程冲突，导致不稳定和崩溃。可以通过 `\Imagick::setResourceLimit(\Imagick::RESOURCETYPE_THREAD, 1)` 禁用 ImageMagick 线程，或通过使用 `--disable-openmp` 编译 ImageMagick 来缓解。OpenMP 在静态二进制文件和官方 `apt`/`apk`/`rpm` 包中已禁用，因此仅 Docker 镜像和 Homebrew 安装受影响。 |

## 超出 PHP 硬内存限制会停止整个服务器

自 PHP 8.2.35、8.3.36、8.4.27 和 8.5.12 起，超出 `memory_limit` 不再立即停止脚本：在下一次函数调用或循环迭代时会引发“允许的内存大小耗尽”的致命错误。如果脚本在此之前分配超过硬内存限制，PHP 将退出进程。硬限制默认为 `memory_limit` 的两倍（或超过 1 GiB 时为 `memory_limit` 加 1 GiB），并且可以通过 `memory_hard_limit_percent` INI 设置进行更改。

超出 `memory_limit` 后仍在运行的代码可能会达到此限制，例如单个分配大于 `memory_limit`（如在大型数组上使用 `array_merge()`）或连续的字符串连接。

PHP-FPM 只会丢失一个子进程。FrankenPHP 从单个进程的线程处理所有请求，因此每个正在进行的请求都会丢失，并且服务器会停止，直到重新启动。请在能重启 FrankenPHP 的监督器下运行它，例如 Docker 重启策略或 systemd 的 `Restart=`（已在 `deb` 和 `rpm` 包中设置）。

## get_browser

[get_browser()](https://www.php.net/manual/function.get-browser.php) 函数在一段时间后似乎表现不佳。解决方法是缓存（例如使用 [APCu](https://www.php.net/manual/book.apcu.php)）每个 User-Agent 的结果，因为它们是静态的。

## 独立的二进制和基于 Alpine 的 Docker 镜像

完全静态的二进制文件和基于 Alpine 的 Docker 镜像 (`dunglas/frankenphp:*-alpine`) 使用的是 [musl libc](https://musl.libc.org/) 而不是 [glibc and friends](https://www.etalabs.net/compare_libcs.html)，为的是保持较小的二进制大小。这可能会导致一些兼容性问题。特别是，glob 标志 `GLOB_BRACE` [不可用](https://www.php.net/manual/function.glob.php)。

如果遇到问题，请优先使用静态二进制文件的 GNU 变体和基于 Debian 的 Docker 镜像。

## 在 Docker 中使用 `https://127.0.0.1`

默认情况下，FrankenPHP 会为 `localhost` 生成一个 TLS 证书。
这是本地开发最简单且推荐的选项。

如果确实想使用 `127.0.0.1` 作为主机，可以通过将服务器名称设置为 `127.0.0.1` 来配置它以为其生成证书。

不幸的是，如果你使用 Docker，因为 [Docker 网络系统](https://docs.docker.com/network/) 问题，只做这些是不够的。
你将收到类似于以下内容的 TLS 错误 `curl: (35) LibreSSL/3.3.6: error:1404B438:SSL routines:ST_CONNECT:tlsv1 alert internal error`。

如果你使用的是 Linux，解决方案是使用 [宿主机网络驱动](https://docs.docker.com/network/network-tutorial-host/)：

```console
docker run \
    -e SERVER_NAME="127.0.0.1" \
    -v $PWD:/app/public \
    --network host \
    dunglas/frankenphp
```

Mac 和 Windows 不支持宿主机网络驱动。在这些平台上，你必须猜测容器的 IP 地址并将其包含在服务器名称中。

运行 `docker network inspect bridge` 并查看 `Containers` 键，以识别 `IPv4Address` 键下当前分配的最后一个 IP 地址，并将其增加 1。如果没有容器正在运行，则第一个分配的 IP 地址通常为 `172.17.0.2`。

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
# /usr/local/bin/php
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

检查 [`openssl_get_cert_locations()`](https://www.php.net/manual/function.openssl-get-cert-locations.php) 的输出，
以找到 CA 证书必须安装的位置，并将它们存储在该位置。

> [!WARNING]
>
> Web 和 CLI 上下文可能有不同的设置。
> 确保在适当的上下文中运行 `openssl_get_cert_locations()`。

[从 Mozilla 提取的 CA 证书可以在 cURL 网站上下载](https://curl.se/docs/caextract.html)。

或者，许多发行版，包括 Debian、Ubuntu 和 Alpine，提供名为 `ca-certificates` 的软件包，其中包含这些证书。

还可以使用 `SSL_CERT_FILE` 和 `SSL_CERT_DIR` 来提示 OpenSSL 在哪里查找 CA 证书：

```console
# Set TLS certificates environment variables
export SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
export SSL_CERT_DIR=/etc/ssl/certs
```
