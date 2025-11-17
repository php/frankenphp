# FrankenPHP: Modern App Server for PHP

<h1 align="center"><a href="https://frankenphp.dev"><img src="frankenphp.png" alt="FrankenPHP" width="600"></a></h1>

FrankenPHP is a modern application server for PHP built on top of the [Caddy](https://caddyserver.com/) web server.

FrankenPHP gives superpowers to your PHP apps thanks to its stunning features: [_Early Hints_](https://frankenphp.dev/docs/early-hints/), [worker mode](https://frankenphp.dev/docs/worker/), [real-time capabilities](https://frankenphp.dev/docs/mercure/), automatic HTTPS, HTTP/2, and HTTP/3 support...

FrankenPHP works with any PHP app and makes your Laravel and Symfony projects faster than ever thanks to their official integrations with the worker mode.

FrankenPHP can also be used as a standalone Go library to embed PHP in any app using `net/http`.

[**Learn more** on _frankenphp.dev_](https://frankenphp.dev) and in this slide deck:

<a href="https://dunglas.dev/2022/10/frankenphp-the-modern-php-app-server-written-in-go/"><img src="https://dunglas.dev/wp-content/uploads/2022/10/frankenphp.png" alt="Slides" width="600"></a>

## Getting Started

### Standalone Binary

We provide static FrankenPHP binaries for Linux and macOS
containing [PHP 8.4](https://www.php.net/releases/8.4/en.php) and most popular PHP extensions.
Only use this option for development as FrankenPHP is compiled for maximum compatibility, rather than performance.

On Windows, use [WSL](https://learn.microsoft.com/windows/wsl/) to run FrankenPHP.

[Download FrankenPHP](https://github.com/php/frankenphp/releases) or copy this line into your
terminal to automatically install the version appropriate for your platform:

```console
curl https://frankenphp.dev/install.sh | sh
mv frankenphp /usr/local/bin/
```

To serve the content of the current directory, run:

```console
frankenphp php-server
```

You can also run command-line scripts with:

```console
frankenphp php-cli /path/to/your/script.php
```

### RPM Packages

We offer RPM packages for all systems using `dnf`. To install, run:

```console
sudo dnf install https://rpm.henderkes.com/static-php-1-0.noarch.rpm
sudo dnf module enable php-zts:static-8.4 # 8.2-8.5 available
sudo dnf install frankenphp
# to install extensions:
sudo dnf install php-zts-xdebug
# if an extension is not available by default, install it with pie
sudo dnf install php-zts-devel
sudo pie install asgrim/example-pie-extension --with-php-config=php-config-zts
```

### DEB Packages

We offer DEB packages for all Debian-based systems. To install, run:

```console
. /etc/os-release && \
sudo curl -fsSL https://key.henderkes.com/static-php.gpg -o /usr/share/keyrings/static-php.gpg && \
echo "deb [signed-by=/usr/share/keyrings/static-php.gpg] https://deb.henderkes.com/ $VERSION_CODENAME main" | sudo tee /etc/apt/sources.list.d/static-php.list && \
sudo apt update
sudo apt install frankenphp
# to install extensions:
sudo apt install php-zts-xdebug
# if an extension is not available by default, install it with pie
sudo apt install php-zts-devel
sudo pie install asgrim/example-pie-extension --with-php-config=php-config-zts
```

### Docker

Alternatively, [Docker images](https://frankenphp.dev/docs/docker/) are available:

```console
docker run -v .:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

Go to `https://localhost`, and enjoy!

> [!TIP]
>
> Do not attempt to use `https://127.0.0.1`. Use `https://localhost` and accept the self-signed certificate.
> Use the [`SERVER_NAME` environment variable](docs/config.md#environment-variables) to change the domain to use.

### Homebrew

FrankenPHP is also available as a [Homebrew](https://brew.sh) package for macOS and Linux.

To install it:

```console
brew install dunglas/frankenphp/frankenphp
```

To serve the content of the current directory, run:

```console
frankenphp php-server
```

## Docs

- [Classic mode](https://frankenphp.dev/docs/classic/)
- [Worker mode](https://frankenphp.dev/docs/worker/)
- [Early Hints support (103 HTTP status code)](https://frankenphp.dev/docs/early-hints/)
- [Real-time](https://frankenphp.dev/docs/mercure/)
- [Efficiently Serving Large Static Files](https://frankenphp.dev/docs/x-sendfile/)
- [Configuration](https://frankenphp.dev/docs/config/)
- [Writing PHP Extensions in Go](https://frankenphp.dev/docs/extensions/)
- [Docker images](https://frankenphp.dev/docs/docker/)
- [Deploy in production](https://frankenphp.dev/docs/production/)
- [Performance optimization](https://frankenphp.dev/docs/performance/)
- [Create **standalone**, self-executable PHP apps](https://frankenphp.dev/docs/embed/)
- [Create static binaries](https://frankenphp.dev/docs/static/)
- [Compile from sources](https://frankenphp.dev/docs/compile/)
- [Monitoring FrankenPHP](https://frankenphp.dev/docs/metrics/)
- [Laravel integration](https://frankenphp.dev/docs/laravel/)
- [Known issues](https://frankenphp.dev/docs/known-issues/)
- [Demo app (Symfony) and benchmarks](https://github.com/dunglas/frankenphp-demo)
- [Go library documentation](https://pkg.go.dev/github.com/dunglas/frankenphp)
- [Contributing and debugging](https://frankenphp.dev/docs/contributing/)

## Examples and Skeletons

- [Symfony](https://github.com/dunglas/symfony-docker)
- [API Platform](https://api-platform.com/docs/symfony)
- [Laravel](https://frankenphp.dev/docs/laravel/)
- [Sulu](https://sulu.io/blog/running-sulu-with-frankenphp)
- [WordPress](https://github.com/StephenMiracle/frankenwp)
- [Drupal](https://github.com/dunglas/frankenphp-drupal)
- [Joomla](https://github.com/alexandreelise/frankenphp-joomla)
- [TYPO3](https://github.com/ochorocho/franken-typo3)
- [Magento2](https://github.com/ekino/frankenphp-magento2)
