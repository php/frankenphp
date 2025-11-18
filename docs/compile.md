# Compile From Sources

This document explains how to create a FrankenPHP binary that will load PHP as a dynamic library.
This is the recommended method.

Alternatively, [fully and mostly static builds](static.md) can also be created.

## Install PHP

FrankenPHP is compatible with PHP 8.2 and superior.

### With Homebrew (Linux and Mac)

The easiest way to install a version of libphp compatible with FrankenPHP is to use the ZTS packages provided by [Homebrew PHP](https://github.com/shivammathur/homebrew-php).

First, if not already done, install [Homebrew](https://brew.sh).

Then, install the ZTS variant of PHP, Brotli (optional, for compression support) and watcher (optional, for file change detection):

```console
brew install shivammathur/php/php-zts brotli watcher
brew link --overwrite --force shivammathur/php/php-zts
```

### By Compiling PHP

Alternatively, you can compile PHP from sources with the options needed by FrankenPHP by following these steps.

First, [get the PHP sources](https://www.php.net/downloads.php) and extract them:

```console
tar xf php-*
cd php-*/
```

Then, run the `configure` script with the options needed for your platform.
The following `./configure` flags are mandatory, but you can add others, for example, to compile extensions or additional features.

#### Linux

```console
./configure \
    --enable-embed \
    --enable-zts \
    --disable-zend-signals \
    --enable-zend-max-execution-timers
```

#### Mac

Use the [Homebrew](https://brew.sh/) package manager to install the required and optional dependencies:

```console
brew install libiconv bison brotli re2c pkg-config watcher
echo 'export PATH="/opt/homebrew/opt/bison/bin:$PATH"' >> ~/.zshrc
```

Then run the configure script:

```console
./configure \
    --enable-embed \
    --enable-zts \
    --disable-zend-signals \
    --with-iconv=/opt/homebrew/opt/libiconv/
```

#### Compile PHP

Finally, compile and install PHP:

```console
make -j"$(getconf _NPROCESSORS_ONLN)"
sudo make install
```

## Install Optional Dependencies

Some FrankenPHP features depend on optional system dependencies that must be installed.
Alternatively, these features can be disabled by passing build tags to the Go compiler.

| Feature                        | Dependency                                                                                                   | Build tag to disable it |
| ------------------------------ | ------------------------------------------------------------------------------------------------------------ | ----------------------- |
| Brotli compression             | [Brotli](https://github.com/google/brotli)                                                                   | nobrotli                |
| Restart workers on file change | [Watcher C](https://github.com/e-dant/watcher/tree/release/watcher-c)                                        | nowatcher               |
| [Mercure](mercure.md)          | [Mercure Go library](https://pkg.go.dev/github.com/dunglas/mercure) (automatically installed, AGPL licensed) | nomercure               |

## Compile the Go App

You can now build the final binary.

### Using xcaddy

The recommended way is to use [xcaddy](https://github.com/caddyserver/xcaddy) to compile FrankenPHP.
`xcaddy` also allows to easily add [custom Caddy modules](https://caddyserver.com/docs/modules/) and FrankenPHP extensions:

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/dunglas/frankenphp/caddy \
    --with github.com/dunglas/mercure/caddy \
    --with github.com/dunglas/vulcain/caddy
    # Add extra Caddy modules and FrankenPHP extensions here
```

> [!TIP]
>
> If you're using musl libc (the default on Alpine Linux) and Symfony,
> you may need to increase the default stack size.
> Otherwise, you may get errors like `PHP Fatal error: Maximum call stack size of 83360 bytes reached during compilation. Try splitting expression`
>
> To do so, change the `XCADDY_GO_BUILD_FLAGS` environment variable to something like
> `XCADDY_GO_BUILD_FLAGS=$'-ldflags "-w -s -extldflags \'-Wl,-z,stack-size=0x80000\'"'`
> (change the stack size value according to your app needs).

### Without xcaddy

Alternatively, it's possible to compile FrankenPHP without `xcaddy` by using the `go` command directly:

```console
curl -L https://github.com/php/frankenphp/archive/refs/heads/main.tar.gz | tar xz
cd frankenphp-main/caddy/frankenphp
CGO_CFLAGS=$(php-config --includes) CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" go build -tags=nobadger,nomysql,nopgx
```
