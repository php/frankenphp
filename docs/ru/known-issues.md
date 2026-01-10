# Известные проблемы

## Неподдерживаемые расширения PHP

Следующие расширения не совместимы с FrankenPHP:

| Название                                                                                                    | Причина                            | Альтернативы                                                                                                         |
| ----------------------------------------------------------------------------------------------------------- | ---------------------------------- | -------------------------------------------------------------------------------------------------------------------- |
| [imap](https://www.php.net/manual/en/imap.installation.php)                                                 | Не поддерживает потокобезопасность | [javanile/php-imap2](https://github.com/javanile/php-imap2), [webklex/php-imap](https://github.com/Webklex/php-imap) |
| [newrelic](https://docs.newrelic.com/docs/apm/agents/php-agent/getting-started/introduction-new-relic-php/) | Не поддерживает потокобезопасность | -                                                                                                                    |

## Проблемные расширения PHP

Следующие расширения имеют известные ошибки или могут вести себя непредсказуемо при использовании с FrankenPHP:

| Название                                                      | Проблема                                                                                                                                                                                                                                                                                                                            |
| ------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [ext-openssl](https://www.php.net/manual/en/book.openssl.php) | При использовании статической сборки FrankenPHP (на базе musl libc) расширение OpenSSL может аварийно завершаться при высокой нагрузке. Решение — использовать динамически связанную сборку (например, ту, что используется в Docker-образах). Ошибка [отслеживается сообществом PHP](https://github.com/php/php-src/issues/13648). |

## `get_browser`

Функция [get_browser()](https://www.php.net/manual/en/function.get-browser.php) начинает работать медленно через некоторое время. Решение — кэшировать результаты для каждого User-Agent, например, с помощью [APCu](https://www.php.net/manual/en/book.apcu.php), так как они статичны.

## Автономные бинарные файлы и образы на базе Alpine

Автономные бинарные файлы и образы на базе Alpine (`dunglas/frankenphp:*-alpine`) используют [musl libc](https://musl.libc.org/) вместо [glibc](https://www.etalabs.net/compare_libcs.html) для уменьшения размера бинарных файлов. Это может вызвать проблемы совместимости. В частности, флаг `GLOB_BRACE` в функции glob [не поддерживается](https://www.php.net/manual/en/function.glob.php).

## Использование `https://127.0.0.1` с Docker

По умолчанию FrankenPHP генерирует TLS-сертификат для `localhost`, что является самым простым и рекомендуемым вариантом для локальной разработки.

Если вы всё же хотите использовать `127.0.0.1`, настройте генерацию сертификата, указав в переменной окружения `SERVER_NAME` значение `127.0.0.1`.

Однако этого может не хватить при использовании Docker из-за [особенностей его сетевой системы](https://docs.docker.com/network/). Возможна ошибка TLS вида:  
`curl: (35) LibreSSL/3.3.6: error:1404B438:SSL routines:ST_CONNECT:tlsv1 alert internal error`.

Если вы используете Linux, можно воспользоваться [host-драйвером](https://docs.docker.com/network/network-tutorial-host/):

```console
docker run \
    -e SERVER_NAME="127.0.0.1" \
    -v $PWD:/app/public \
    --network host \
    dunglas/frankenphp
```

Host-драйвер не поддерживается на Mac и Windows. На этих платформах нужно определить IP-адрес контейнера и включить его в `SERVER_NAME`.

Выполните команду `docker network inspect bridge`, найдите ключ `Containers` и определите последний присвоенный IP из `IPv4Address`. Увеличьте его на единицу. Если контейнеров нет, первый IP обычно `172.17.0.2`.

Включите этот IP в переменную окружения `SERVER_NAME`:

```console
docker run \
    -e SERVER_NAME="127.0.0.1, 172.17.0.3" \
    -v $PWD:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

> [!CAUTION]
> Обязательно замените `172.17.0.3` на IP, который будет присвоен вашему контейнеру.

Теперь вы должны иметь доступ к `https://127.0.0.1`.

Если это не так, запустите FrankenPHP в режиме отладки:

```console
docker run \
    -e CADDY_GLOBAL_OPTIONS="debug" \
    -e SERVER_NAME="127.0.0.1" \
    -v $PWD:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

## Скрипты Composer с использованием `@php`

[Скрипты Composer](https://getcomposer.org/doc/articles/scripts.md) могут вызывать PHP для выполнения задач, например, в [проекте Laravel](laravel.md) для команды `@php artisan package:discover --ansi`.  
Это [на данный момент не поддерживается](https://github.com/php/frankenphp/issues/483#issuecomment-1899890915) по двум причинам:

- Composer не знает, как вызывать бинарный файл FrankenPHP;
- Composer может добавлять настройки PHP через флаг `-d`, который FrankenPHP пока не поддерживает.

Решение — создать shell-скрипт в `/usr/local/bin/php`, который удаляет неподдерживаемые параметры и вызывает FrankenPHP:

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

Затем установите переменную окружения `PHP_BINARY` на путь к нашему скрипту `php` и запустите Composer:

```console
export PHP_BINARY=/usr/local/bin/php
composer install
```

## TLS/SSL: проблемы со статическими бинарными файлами

При использовании статических бинарных файлов могут возникать следующие ошибки TLS, например, при отправке писем через STARTTLS:

```text
Unable to connect with STARTTLS: stream_socket_enable_crypto(): SSL operation failed with code 5. OpenSSL Error messages:
error:80000002:system library::No such file or directory
error:80000002:system library::No such file or directory
error:80000002:system library::No such file or directory
error:0A000086:SSL routines::certificate verify failed
```

Статический бинарный файл не включает TLS-сертификаты, поэтому необходимо указать OpenSSL местоположение локальных сертификатов CA.

Выполните [`openssl_get_cert_locations()`](https://www.php.net/manual/en/function.openssl-get-cert-locations.php), чтобы определить, где должны находиться сертификаты CA, и поместите их туда.

> [!WARNING]
> Веб и CLI контексты могут иметь разные настройки.  
> Запустите `openssl_get_cert_locations()` в нужном контексте.

[Сертификаты CA, извлечённые из Mozilla, можно скачать с сайта cURL](https://curl.se/docs/caextract.html).

Кроме того, многие дистрибутивы, такие как Debian, Ubuntu и Alpine, предоставляют пакеты `ca-certificates`, содержащие эти сертификаты.

Также можно использовать переменные `SSL_CERT_FILE` и `SSL_CERT_DIR`, чтобы указать OpenSSL, где искать сертификаты CA:

```console
# Установите переменные окружения для TLS-сертификатов
export SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
export SSL_CERT_DIR=/etc/ssl/certs
```
