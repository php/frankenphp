---
title: FrankenPHP 静态构建：单二进制 PHP 应用程序服务器
description: 在 Linux 和 macOS 上创建带有嵌入式 PHP 和 Caddy 的静态或大部分静态 FrankenPHP 二进制文件，自定义扩展，并使用 glibc 加载动态模块。
---

# 创建静态构建

与其使用本地安装的 PHP 库，
通过出色的 [static-php-cli 项目](https://github.com/crazywhalecc/static-php-cli)，可以创建 FrankenPHP 的静态或大部分静态构建（尽管其名称，该项目支持所有 SAPI，而不仅仅是 CLI）。

使用此方法，单个便携式二进制文件将包含 PHP 解释器、Caddy Web 服务器和 FrankenPHP！

完全静态的本地可执行文件无需任何依赖，甚至可以在 [`scratch` Docker 镜像](https://docs.docker.com/build/building/base-images/#create-a-minimal-base-image-using-scratch) 上运行。
然而，它们无法加载动态 PHP 扩展（例如 Xdebug），并且由于使用了 musl libc，存在一些限制。

大部分静态二进制文件只需要 `glibc` 即可加载动态扩展。

在可能的情况下，我们建议使用基于 glibc 的大部分静态构建。

FrankenPHP 还支持 [将 PHP 应用程序嵌入到静态二进制文件中](embed.md)。

## Linux

我们提供 Docker 镜像来构建 Linux 静态二进制文件：

### 基于 musl 的完全静态构建

对于一个在任何 Linux 发行版上运行且无需依赖，但不支持动态加载扩展的完全静态二进制文件：

```console
docker buildx bake --load static-builder-musl
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log dunglas/frankenphp:static-builder-musl
cp "output/frankenphp-linux-$(uname -m)" frankenphp
```

为了在高度并发场景中获得更好的性能，请考虑使用 [mimalloc](https://github.com/microsoft/mimalloc) 分配器。

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log -e MIMALLOC=1 dunglas/frankenphp:static-builder-musl
```

### 基于 glibc 的，大部分静态构建（支持动态扩展）

对于一个支持动态加载 PHP 扩展，同时仍然将所选扩展静态编译的二进制文件：

```console
docker buildx bake --load static-builder-gnu
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log dunglas/frankenphp:static-builder-gnu
cp "output/frankenphp-linux-$(uname -m)" frankenphp
```

此二进制文件支持所有 glibc 2.28 及更高版本，但不能在基于 musl 的系统（如 Alpine Linux）上运行。

生成的（除了 `glibc` 之外）大部分静态二进制文件名为 `frankenphp`，并位于当前目录中。

如果构建失败，static-php-cli 日志位于 `output/log/`。

如果您想在没有 Docker 的情况下构建静态二进制文件，请查看 macOS 说明，它也适用于 Linux。

### 静态构建中的自定义 PHP 扩展

默认情况下，最流行的 PHP 扩展会被编译。
默认配置位于 [`craft.yml`](https://github.com/php/frankenphp/blob/main/craft.yml) 中。

为了减小二进制文件的大小并减少攻击面，您可以使用 `PHP_EXTENSIONS` 变量选择要构建的扩展列表。

例如，运行以下命令仅构建 `opcache` 扩展：

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log -e PHP_EXTENSIONS=opcache,pdo_sqlite dunglas/frankenphp:static-builder-musl
# ...
```

要将启用额外功能的库添加到已启用的扩展中，您可以传递 `PHP_EXTENSION_LIBS` 变量：

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log \
  -e PHP_EXTENSIONS=gd \
  -e PHP_EXTENSION_LIBS=libjpeg,libwebp \
  dunglas/frankenphp:static-builder-musl
```

### 额外的 Caddy 模块

要添加额外的 Caddy 模块或将其他参数传递给 [xcaddy](https://github.com/caddyserver/xcaddy)，请使用 `XCADDY_ARGS` 变量：

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log \
  -e XCADDY_ARGS="--with github.com/darkweak/souin/plugins/caddy --with github.com/dunglas/caddy-cbrotli --with github.com/dunglas/mercure/caddy --with github.com/dunglas/vulcain/caddy" \
  dunglas/frankenphp:static-builder-musl
```

在此示例中，我们添加了 Caddy 的 [Souin](https://souin.io) HTTP 缓存模块，以及 [cbrotli](https://github.com/dunglas/caddy-cbrotli)、[Mercure](https://mercure.rocks) 和 [Vulcain](https://vulcain.rocks) 模块。

> [!TIP]
>
> 如果 `XCADDY_ARGS` 为空或未设置，则默认包含 cbrotli、Mercure 和 Vulcain 模块。
> 如果您自定义 `XCADDY_ARGS` 的值，则必须显式包含它们才能使其生效。

另请参阅如何 [自定义 FrankenPHP 静态构建](#自定义-frankenphp-静态构建)

### GitHub token

如果您达到了 GitHub API 速率限制，请在名为 `GITHUB_TOKEN` 的环境变量中设置 GitHub 个人访问令牌：

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log -e GITHUB_TOKEN="xxx" dunglas/frankenphp:static-builder-musl
# ...
```

## macOS

运行以下脚本为 macOS 创建静态二进制文件（您必须已安装 [Homebrew](https://brew.sh/)）：

```console
git clone https://github.com/php/frankenphp
cd frankenphp
./build-static.sh
```

注意：此脚本也适用于 Linux（可能也适用于其他 Unix 系统），并且在我们提供的 Docker 镜像内部使用。

我们只发布 Apple Silicon 架构的 macOS 二进制文件。在 Intel Mac 上，请使用此脚本从源代码构建。

## 自定义 FrankenPHP 静态构建

以下环境变量可以传递给 `docker run` 和 `build-static.sh` 脚本，以自定义静态构建：

- `FRANKENPHP_VERSION`: 要使用的 FrankenPHP 版本
- `PHP_VERSION`: 要使用的 PHP 版本
- `PHP_EXTENSIONS`: 要构建的 PHP 扩展（[支持的扩展列表](https://static-php.dev/guide/extensions.html)）
- `PHP_EXTENSION_LIBS`: 要构建的额外库，为扩展添加额外的功能
- `XCADDY_ARGS`: 传递给 [xcaddy](https://github.com/caddyserver/xcaddy) 的参数，例如用于添加额外的 Caddy 模块
- `EMBED`: 要嵌入二进制文件的 PHP 应用程序的路径
- `CLEAN`: 设置后，libphp 及其所有依赖项都将从头开始构建（不使用缓存）
- `COMPRESS`: 设置为 `1` 时，使用 UPX 压缩生成的二进制文件（仅限 Linux；设置了 `DEBUG_SYMBOLS` 时忽略此选项）
- `DEBUG_SYMBOLS`: 设置后，调试符号将不会被剥离，并会添加到二进制文件中
- `MIMALLOC`: (实验性，仅限 Linux) 将 musl 的 mallocng 替换为 [mimalloc](https://github.com/microsoft/mimalloc)，以提高性能。我们仅建议在面向 musl 的构建中使用此选项；对于 glibc，建议禁用此选项，并在运行二进制文件时使用 [`LD_PRELOAD`](https://microsoft.github.io/mimalloc/overrides.html)。
- `RELEASE`: (仅限维护者) 设置后，生成的二进制文件将上传到 GitHub 上
- `LIBC`: (仅限 Linux) `musl` 用于完全静态的二进制文件（默认），或 `gnu` 用于动态链接到系统 glibc。它是 static-php-cli 的 `SPC_TARGET` 和 `SPC_TOOLCHAIN` 的简写，这两个变量仍然可以直接设置；它们必须是真实的环境变量，因为 static-php-cli 在读取 `craft.yml` 之前会解析其工具链。`musl` 默认目标是架构的基准 CPU (`x86_64-linux-musl` 或 `aarch64-linux-musl`)，因此二进制文件可以在任何 CPU 上运行；设置 `SPC_TARGET=native-native-musl` 则会优化为构建机器。

## 在静态二进制文件中动态加载 PHP 扩展

使用基于 glibc 或 macOS 的二进制文件，您可以动态加载 PHP 扩展。然而，这些扩展必须使用 ZTS 支持进行编译。
由于大多数软件包管理器目前不提供其扩展的 ZTS 版本，因此您将不得不自己编译它们。

为此，您可以运行 `static-builder-gnu` Docker 容器，远程进入其中，并使用 `./configure --with-php-config=/go/src/app/dist/buildroot/bin/php-config` 编译扩展。

以下是 [Xdebug 扩展](https://xdebug.org) 的示例步骤：

```console
docker buildx bake --load static-builder-gnu
docker run --rm -v "$PWD/dist:/go/src/app/dist" -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log dunglas/frankenphp:static-builder-gnu
docker run --rm -it -v "$PWD/dist:/go/src/app/dist" dunglas/frankenphp:static-builder-gnu /bin/bash
cd /go/src/app/dist/buildroot/bin
git clone https://github.com/xdebug/xdebug.git && cd xdebug
../phpize
./configure --with-php-config=/go/src/app/dist/buildroot/bin/php-config
make
exit
cp dist/buildroot/bin/xdebug/modules/xdebug.so xdebug-zts.so
cp "output/frankenphp-linux-$(uname -m)" frankenphp
```

这将在当前目录中创建 `frankenphp` 和 `xdebug-zts.so`。
如果您将 `xdebug-zts.so` 移动到您的扩展目录中，将 `zend_extension=xdebug-zts.so` 添加到您的 php.ini 并运行 FrankenPHP，它将加载 Xdebug。
