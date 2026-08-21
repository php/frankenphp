---
title: FrankenPHP static build: single-binary PHP app server
description: Create static or mostly static FrankenPHP binaries with embedded PHP and Caddy on Linux and macOS, customize extensions, and load dynamic modules with glibc.
---

# Create a static build

Instead of using a local installation of the PHP library,
it's possible to create a static or mostly static build of FrankenPHP thanks to the great [static-php-cli project](https://github.com/crazywhalecc/static-php-cli) (despite its name, this project supports all SAPIs, not only CLI).

With this method, a single, portable, binary will contain the PHP interpreter, the Caddy web server, and FrankenPHP!

Fully static native executables require no dependencies at all and can even be run on a [`scratch` Docker image](https://docs.docker.com/build/building/base-images/#create-a-minimal-base-image-using-scratch).
However, they can't load dynamic PHP extensions (such as Xdebug) and have some limitations because they are using the musl libc.

Mostly static binaries only require `glibc` and can load dynamic extensions.

When possible, we recommend using glibc-based, mostly static builds.

FrankenPHP also supports [embedding the PHP app in the static binary](embed.md).

## Linux

We provide Docker images to build static Linux binaries:

### musl-based, fully static build

For a fully-static binary that runs on any Linux distribution without dependencies but doesn't support dynamic loading of extensions:

```console
docker buildx bake --load static-builder-musl
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log dunglas/frankenphp:static-builder-musl
cp "output/frankenphp-linux-$(uname -m)" frankenphp
```

For better performance in heavily concurrent scenarios, consider using the [mimalloc](https://github.com/microsoft/mimalloc) allocator.

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log -e MIMALLOC=1 dunglas/frankenphp:static-builder-musl
```

### glibc-based, mostly static build (with dynamic extension support)

For a binary that supports loading PHP extensions dynamically while still having the selected extensions compiled statically:

```console
docker buildx bake --load static-builder-gnu
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log dunglas/frankenphp:static-builder-gnu
cp "output/frankenphp-linux-$(uname -m)" frankenphp
```

This binary supports all glibc versions 2.28 and higher but does not run on musl-based systems (like Alpine Linux).

The resulting mostly static (except `glibc`) binary is named `frankenphp` and is available in the current directory.

If the build fails, the static-php-cli logs are in `output/log/`.

If you want to build the static binary without Docker, take a look at the macOS instructions, which also work for Linux.

### Custom PHP extensions in the static build

By default, the most popular PHP extensions are compiled.
The defaults live in [`craft.yml`](https://github.com/php/frankenphp/blob/main/craft.yml).

To reduce the size of the binary and to reduce the attack surface, you can choose the list of extensions to build using the `PHP_EXTENSIONS` variable.

For instance, run the following command to only build the `opcache` extension:

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log -e PHP_EXTENSIONS=opcache,pdo_sqlite dunglas/frankenphp:static-builder-musl
# ...
```

To add libraries enabling additional functionality to the extensions you've enabled, you can pass the `PHP_EXTENSION_LIBS` variable:

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log \
  -e PHP_EXTENSIONS=gd \
  -e PHP_EXTENSION_LIBS=libjpeg,libwebp \
  dunglas/frankenphp:static-builder-musl
```

### Extra Caddy modules

To add extra Caddy modules or pass other arguments to [xcaddy](https://github.com/caddyserver/xcaddy), use the `XCADDY_ARGS` variable:

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log \
  -e XCADDY_ARGS="--with github.com/darkweak/souin/plugins/caddy --with github.com/dunglas/caddy-cbrotli --with github.com/dunglas/mercure/caddy --with github.com/dunglas/vulcain/caddy" \
  dunglas/frankenphp:static-builder-musl
```

In this example, we add the [Souin](https://souin.io) HTTP cache module for Caddy as well as the [cbrotli](https://github.com/dunglas/caddy-cbrotli), [Mercure](https://mercure.rocks) and [Vulcain](https://vulcain.rocks) modules.

> [!TIP]
>
> The cbrotli, Mercure, and Vulcain modules are included by default if `XCADDY_ARGS` is empty or not set.
> If you customize the value of `XCADDY_ARGS`, you must include them explicitly if you want them to be included.

See also how to [customize the FrankenPHP static build](#customizing-the-frankenphp-static-build)

### GitHub token

If you hit the GitHub API rate limit, set a GitHub Personal Access Token in an environment variable named `GITHUB_TOKEN`:

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log -e GITHUB_TOKEN="xxx" dunglas/frankenphp:static-builder-musl
# ...
```

## macOS

Run the following script to create a static binary for macOS (you must have [Homebrew](https://brew.sh/) installed):

```console
git clone https://github.com/php/frankenphp
cd frankenphp
./build-static.sh
```

Note: this script also works on Linux (and probably on other Unixes), and is used internally by the Docker images we provide.

## Customizing the FrankenPHP static build

The following environment variables can be passed to `docker run` and to the `build-static.sh`
script to customize the static build:

- `FRANKENPHP_VERSION`: the version of FrankenPHP to use
- `PHP_VERSION`: the version of PHP to use
- `PHP_EXTENSIONS`: the PHP extensions to build ([list of supported extensions](https://static-php.dev/en/guide/extensions.html))
- `PHP_EXTENSION_LIBS`: extra libraries to build that add features to the extensions
- `XCADDY_ARGS`: arguments to pass to [xcaddy](https://github.com/caddyserver/xcaddy), for instance to add extra Caddy modules
- `EMBED`: path of the PHP application to embed in the binary
- `CLEAN`: when set, libphp and all its dependencies are built from scratch (no cache)
- `COMPRESS`: when set to `1`, pack the resulting binary with UPX (Linux only; ignored when `DEBUG_SYMBOLS` is set)
- `DEBUG_SYMBOLS`: when set, debug-symbols will not be stripped and will be added to the binary
- `MIMALLOC`: (experimental, Linux-only) replace musl's mallocng by [mimalloc](https://github.com/microsoft/mimalloc) for improved performance. We only recommend using this for musl targeting builds, for glibc prefer disabling this option and using [`LD_PRELOAD`](https://microsoft.github.io/mimalloc/overrides.html) when you run your binary instead.
- `RELEASE`: (maintainers only) when set, the resulting binary will be uploaded on GitHub

- `LIBC`: (Linux only) `musl` for a fully static binary (the default), or `gnu` to link
  dynamically against the system glibc. It is a shorthand for static-php-cli's `SPC_TARGET`
  and `SPC_TOOLCHAIN`, which can still be set directly; they have to be real environment
  variables because static-php-cli resolves its toolchain before reading `craft.yml`.

## Loading PHP extensions dynamically in the static binary

With the glibc or macOS-based binaries, you can load PHP extensions dynamically. However, these extensions will have to be compiled with ZTS support.
Since most package managers do not currently offer ZTS versions of their extensions, you will have to compile them yourself.

For this, you can run the `static-builder-gnu` Docker container, remote into it, and compile the extensions with `./configure --with-php-config=/go/src/app/dist/buildroot/bin/php-config`.

Example steps for [the Xdebug extension](https://xdebug.org):

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

This will have created `frankenphp` and `xdebug-zts.so` in the current directory.
If you move the `xdebug-zts.so` into your extension directory, add `zend_extension=xdebug-zts.so` to your php.ini and run FrankenPHP, it will load Xdebug.
