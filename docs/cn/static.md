# 创建静态构建

与其使用本地安装的PHP库，
由于伟大的 [static-php-cli 项目](https://github.com/crazywhalecc/static-php-cli)，创建一个静态或基本静态的 FrankenPHP 构建是可能的（尽管它的名字，这个项目支持所有的 SAPI，而不仅仅是 CLI）。

使用这种方法，我们可构建一个包含 PHP 解释器、Caddy Web 服务器和 FrankenPHP 的可移植二进制文件！

完全静态的本地可执行文件不需要任何依赖，并且可以在 [`scratch` Docker 镜像](https://docs.docker.com/build/building/base-images/#create-a-minimal-base-image-using-scratch) 上运行。
然而，它们无法加载动态 PHP 扩展（例如 Xdebug），并且由于使用了 musl libc，有一些限制。

大多数静态二进制文件只需要 `glibc` 并且可以加载动态扩展。

在可能的情况下，我们建议使用基于glibc的、主要是静态构建的版本。

FrankenPHP 还支持 [将 PHP 应用程序嵌入到静态二进制文件中](embed.md)。

## Linux

我们提供了一个 Docker 镜像来构建 Linux 静态二进制文件：

### 基于musl的完全静态构建

对于一个在任何Linux发行版上运行且不需要依赖项的完全静态二进制文件，但不支持动态加载扩展：

```console
docker buildx bake --load static-builder-musl
docker cp $(docker create --name static-builder-musl dunglas/frankenphp:static-builder-musl):/go/src/app/dist/frankenphp-linux-$(uname -m) frankenphp ; docker rm static-builder-musl
```

为了在高度并发的场景中获得更好的性能，请考虑使用 [mimalloc](https://github.com/microsoft/mimalloc) 分配器。

```console
docker buildx bake --load --set static-builder-musl.args.MIMALLOC=1 static-builder-musl
```

### 基于glibc的，主要静态构建（支持动态扩展）

对于一个支持动态加载 PHP 扩展的二进制文件，同时又将所选扩展静态编译：

```console
docker buildx bake --load static-builder-gnu
docker cp $(docker create --name static-builder-gnu dunglas/frankenphp:static-builder-gnu):/go/src/app/dist/frankenphp-linux-$(uname -m) frankenphp ; docker rm static-builder-gnu
```

该二进制文件支持所有glibc版本2.17及以上，但不支持基于musl的系统（如Alpine Linux）。

生成的主要是静态的（除了 `glibc`）二进制文件名为 `frankenphp`，并且可以在当前目录中找到。

如果你想在没有 Docker 的情况下构建静态二进制文件，请查看 macOS 说明，它也适用于 Linux。

### 自定义扩展

默认情况下，大多数流行的 PHP 扩展都会被编译。

为了减少二进制文件的大小和减少攻击面，您可以选择使用 `PHP_EXTENSIONS` Docker ARG 构建的扩展列表。

例如，运行以下命令仅构建 `opcache` 扩展：

```console
docker buildx bake --load --set static-builder-musl.args.PHP_EXTENSIONS=opcache,pdo_sqlite static-builder-musl
# ...
```

若要将启用其他功能的库添加到已启用的扩展中，可以使用 `PHP_EXTENSION_LIBS` Docker 参数：

```console
docker buildx bake \
  --load \
  --set static-builder-musl.args.PHP_EXTENSIONS=gd \
  --set static-builder-musl.args.PHP_EXTENSION_LIBS=libjpeg,libwebp \
  static-builder-musl
```

### 额外的 Caddy 模块

要向 [xcaddy](https://github.com/caddyserver/xcaddy) 添加额外的 Caddy 模块或传递其他参数，请使用 `XCADDY_ARGS` Docker 参数：

```console
docker buildx bake \
  --load \
  --set static-builder-musl.args.XCADDY_ARGS="--with github.com/darkweak/souin/plugins/caddy --with github.com/dunglas/caddy-cbrotli --with github.com/dunglas/mercure/caddy --with github.com/dunglas/vulcain/caddy" \
  static-builder-musl
```

在本例中，我们为 Caddy 添加了 [Souin](https://souin.io) HTTP 缓存模块，以及 [cbrotli](https://github.com/dunglas/caddy-cbrotli)、[Mercure](https://mercure.rocks) 和 [Vulcain](https://vulcain.rocks) 模块。

> [!TIP]
>
> 如果 `XCADDY_ARGS` 为空或未设置，则默认包含 cbrotli、Mercure 和 Vulcain 模块。
> 如果自定义了 `XCADDY_ARGS` 的值，则必须显式地包含它们。

参见：[自定义构建](#自定义构建)

### GitHub Token

如果遇到了 GitHub API 速率限制，请在 `GITHUB_TOKEN` 的环境变量中设置 GitHub Personal Access Token：

```console
GITHUB_TOKEN="xxx" docker --load buildx bake static-builder-musl
# ...
```

## macOS

运行以下脚本以创建适用于 macOS 的静态二进制文件（需要先安装 [Homebrew](https://brew.sh/)）：

```console
git clone https://github.com/php/frankenphp
cd frankenphp
./build-static.sh
```

注意：此脚本也适用于 Linux（可能也适用于其他 Unix 系统），我们提供的用于构建静态二进制的 Docker 镜像也在内部使用这个脚本。

## 自定义构建

以下环境变量可以传递给 `docker build` 和 `build-static.sh`
脚本来自定义静态构建：

- `FRANKENPHP_VERSION`: 要使用的 FrankenPHP 版本
- `PHP_VERSION`: 要使用的 PHP 版本
- `PHP_EXTENSIONS`: 要构建的 PHP 扩展（[支持的扩展列表](https://static-php.dev/zh/guide/extensions.html)）
- `PHP_EXTENSION_LIBS`: 要构建的额外库，为扩展添加额外的功能
- `XCADDY_ARGS`：传递给 [xcaddy](https://github.com/caddyserver/xcaddy) 的参数，例如用于添加额外的 Caddy 模块
- `EMBED`: 要嵌入二进制文件的 PHP 应用程序的路径
- `CLEAN`: 设置后，libphp 及其所有依赖项都是重新构建的（不使用缓存）
- `NO_COMPRESS`: 不要使用UPX压缩生成的二进制文件
- `DEBUG_SYMBOLS`: 设置后，调试符号将被保留在二进制文件内
- `MIMALLOC`: (实验性，仅限Linux) 用[mimalloc](https://github.com/microsoft/mimalloc)替换musl的mallocng，以提高性能。我们仅建议在musl目标构建中使用此选项，对于glibc，建议禁用此选项，并在运行二进制文件时使用[`LD_PRELOAD`](https://microsoft.github.io/mimalloc/overrides.html)。
- `RELEASE`: （仅限维护者）设置后，生成的二进制文件将上传到 GitHub 上

## 扩展

使用glibc或基于macOS的二进制文件，您可以动态加载PHP扩展。然而，这些扩展必须使用ZTS支持进行编译。
由于大多数软件包管理器目前不提供其扩展的 ZTS 版本，因此您必须自己编译它们。

为此，您可以构建并运行 `static-builder-gnu` Docker 容器，远程进入它，并使用 `./configure --with-php-config=/go/src/app/dist/static-php-cli/buildroot/bin/php-config` 编译扩展。

关于 [Xdebug 扩展](https://xdebug.org) 的示例步骤：

```console
docker build -t gnu-ext -f static-builder-gnu.Dockerfile --build-arg FRANKENPHP_VERSION=1.0 .
docker create --name static-builder-gnu -it gnu-ext /bin/sh
docker start static-builder-gnu
docker exec -it static-builder-gnu /bin/sh
cd /go/src/app/dist/static-php-cli/buildroot/bin
git clone https://github.com/xdebug/xdebug.git && cd xdebug
source scl_source enable devtoolset-10
../phpize
./configure --with-php-config=/go/src/app/dist/static-php-cli/buildroot/bin/php-config
make
exit
docker cp static-builder-gnu:/go/src/app/dist/static-php-cli/buildroot/bin/xdebug/modules/xdebug.so xdebug-zts.so
docker cp static-builder-gnu:/go/src/app/dist/frankenphp-linux-$(uname -m) ./frankenphp
docker stop static-builder-gnu
docker rm static-builder-gnu
docker rmi gnu-ext
```

这将在当前目录中创建 `frankenphp` 和 `xdebug-zts.so`。
如果你将 `xdebug-zts.so` 移动到你的扩展目录中，添加 `zend_extension=xdebug-zts.so` 到你的 php.ini 并运行 FrankenPHP，它将加载 Xdebug。
