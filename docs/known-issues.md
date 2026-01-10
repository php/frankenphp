# Known Issues

## Unsupported PHP Extensions

The following extensions are known not to be compatible with FrankenPHP:

| Name                                                                                                        | Reason          | Alternatives                                                                                                         |
| ----------------------------------------------------------------------------------------------------------- | --------------- | -------------------------------------------------------------------------------------------------------------------- |
| [imap](https://www.php.net/manual/en/imap.installation.php)                                                 | Not thread-safe | [javanile/php-imap2](https://github.com/javanile/php-imap2), [webklex/php-imap](https://github.com/Webklex/php-imap) |
| [newrelic](https://docs.newrelic.com/docs/apm/agents/php-agent/getting-started/introduction-new-relic-php/) | Not thread-safe | - *                                                                                                                 |

## Buggy PHP Extensions

The following extensions have known bugs and unexpected behaviors when used with FrankenPHP:

| Name                                                          | Problem                                                                                                                                                                                                                   |
| ------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [ext-openssl](https://www.php.net/manual/en/book.openssl.php) | When using musl libc, the OpenSSL extension may crash under heavy loads. The problem doesn't occur when using the more popular GNU libc. This bug is [being tracked by PHP](https://github.com/php/php-src/issues/13648). |

## get_browser

The [get_browser()](https://www.php.net/manual/en/function.get-browser.php) function seems to perform badly after a while. A workaround is to cache (e.g. with [APCu](https://www.php.net/manual/en/book.apcu.php)) the results per User Agent, as they are static.

## Standalone Binary and Alpine-based Docker Images

The fully binary and Alpine-based Docker images (`dunglas/frankenphp:*-alpine`) use [musl libc](https://musl.libc.org/) instead of [glibc and friends](https://www.etalabs.net/compare_libcs.html), to keep a smaller binary size.
This may lead to some compatibility issues.
In particular, the glob flag `GLOB_BRACE` is [not available](https://www.php.net/manual/en/function.glob.php)

Prefer using the GNU variant of the static binary and Debian-based Docker images if you encounter issues.

## Using `https://127.0.0.1` with Docker

By default, FrankenPHP generates a TLS certificate for `localhost`.
It's the easiest and recommended option for local development.

If you really want to use `127.0.0.1` as a host instead, it's possible to configure it to generate a certificate for it by setting the server name to `127.0.0.1`.

Unfortunately, this is not enough when using Docker because of [its networking system](https://docs.docker.com/network/).
You will get a TLS error similar to `curl: (35) LibreSSL/3.3.6: error:1404B438:SSL routines:ST_CONNECT:tlsv1 alert internal error`.

If you're using Linux, a solution is to use [the host networking driver](https://docs.docker.com/network/network-tutorial-host/):

```console
docker run \
    -e SERVER_NAME="127.0.0.1" \
    -v $PWD:/app/public \
    --network host \
    dunglas/frankenphp
```

The host networking driver isn't supported on Mac and Windows. On these platforms, you will have to guess the IP address of the container and include it in the server names.

Run the `docker network inspect bridge` and look at the `Containers` key to identify the last currently assigned IP address under the `IPv4Address` key, and increment it by one. If no container is running, the first assigned IP address is usually `172.17.0.2`.

Then, include this in the `SERVER_NAME` environment variable:

```console
docker run \
    -e SERVER_NAME="127.0.0.1, 172.17.0.3" \
    -v $PWD:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

> [!CAUTION]
>
> Be sure to replace `172.17.0.3` with the IP that will be assigned to your container.

You should now be able to access `https://127.0.0.1` from the host machine.

If that's not the case, start FrankenPHP in debug mode to try to figure out the problem:

```console
docker run \
    -e CADDY_GLOBAL_OPTIONS="debug" \
    -e SERVER_NAME="127.0.0.1" \
    -v $PWD:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

## Composer Scripts Referencing `@php`

[Composer scripts](https://getcomposer.org/doc/articles/scripts.md) may want to execute a PHP binary for some tasks, e.g. in [a Laravel project](laravel.md) to run `@php artisan package:discover --ansi`. This [currently fails](https://github.com/php/frankenphp/issues/483#issuecomment-1899890915) for two reasons:

- Composer does not know how to call the FrankenPHP binary;
- Composer may add PHP settings using the `-d` flag in the command, which FrankenPHP does not yet support.

As a workaround, we can create a shell script in `/usr/local/bin/php` which strips the unsupported parameters and then calls FrankenPHP:

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

Then set the environment variable `PHP_BINARY` to the path of our `php` script and run Composer:

```console
export PHP_BINARY=/usr/local/bin/php
composer install
```

## Troubleshooting TLS/SSL Issues with Static Binaries

When using the static binaries, you may encounter the following TLS-related errors, for instance when sending emails using STARTTLS:

```text
Unable to connect with STARTTLS: stream_socket_enable_crypto(): SSL operation failed with code 5. OpenSSL Error messages:
error:80000002:system library::No such file or directory
error:80000002:system library::No such file or directory
error:80000002:system library::No such file or directory
error:0A000086:SSL routines::certificate verify failed
```

As the static binary doesn't bundle TLS certificates, you need to point OpenSSL to your local CA certificates installation.

Inspect the output of [`openssl_get_cert_locations()`](https://www.php.net/manual/en/function.openssl-get-cert-locations.php),
to find where CA certificates must be installed and store them at this location.

> [!WARNING]
>
> Web and CLI contexts may have different settings.
> Be sure to run `openssl_get_cert_locations()` in the proper context.

[CA certificates extracted from Mozilla can be downloaded on the cURL site](https://curl.se/docs/caextract.html).

Alternatively, many distributions, including Debian, Ubuntu, and Alpine provide packages named `ca-certificates` that contain these certificates.

It's also possible to use the `SSL_CERT_FILE` and `SSL_CERT_DIR` to hint OpenSSL where to look for CA certificates:

```console
# Set TLS certificates environment variables
export SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
export SSL_CERT_DIR=/etc/ssl/certs
```
