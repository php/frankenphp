---
title: Известные проблемы FrankenPHP, несовместимые расширения и обходные пути
description: Справочник по ограничениям FrankenPHP, включая неподдерживаемые расширения PHP, особенности musl libc, настройку Docker TLS с 127.0.0.1 и скрипты Composer @php.
---

# Известные проблемы

## Неподдерживаемые расширения PHP

Следующие расширения не совместимы с FrankenPHP:

| Название                                                                                                    | Причина                             | Альтернативы                                                                                                                                                                    |
| :---------------------------------------------------------------------------------------------------------- | :---------------------------------- | :-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [imap](https://www.php.net/manual/imap.installation.php)                                                    | Не является потокобезопасным        | [javanile/php-imap2](https://github.com/javanile/php-imap2), [webklex/php-imap](https://github.com/Webklex/php-imap), [ImapEngine](https://github.com/DirectoryTree/ImapEngine) |
| [newrelic](https://docs.newrelic.com/docs/apm/agents/php-agent/getting-started/introduction-new-relic-php/) | Не является потокобезопасным        | -                                                                                                                                                                               |
| [pcov](https://github.com/krakjoe/pcov)                                                                     | Не является потокобезопасным ([SIGSEGV в классическом режиме](https://github.com/krakjoe/pcov/issues/132)) | Собирайте покрытие с помощью SAPI CLI (запускайте PHPUnit/Pest вне FrankenPHP)                                                                                                    |

## Проблемные расширения PHP

Следующие расширения имеют известные ошибки или могут вести себя непредсказуемо при использовании с FrankenPHP:

| Название                                  | Проблема                                                                                                                                         |
| :---------------------------------------- | :----------------------------------------------------------------------------------------------------------------------------------------------- |
| [datadog](https://github.com/DataDog)     | Испытывает нестабильность при профилировании FrankenPHP. Эта ошибка [отслеживается DataDog](https://github.com/DataDog/dd-trace-php/issues/3729). |
| [blackfire](https://blackfire.io/)        | [Поддержка FrankenPHP находится в бета-версии и ещё не полностью функциональна](https://docs.blackfire.io/php/integrations/frankenphp).            |
| [imagick](https://github.com/Imagick/imagick) | Потоки OpenMP ImageMagick конфликтуют с потоками FrankenPHP, вызывая нестабильность и сбои. Смягчить проблему можно, отключив потоки ImageMagick с помощью `\Imagick::setResourceLimit(\Imagick::RESOURCETYPE_THREAD, 1)` или скомпилировав ImageMagick с `--disable-openmp`. OpenMP уже отключен в статических бинарных файлах и официальных пакетах `apt`/`apk`/`rpm`, поэтому затронуты только образы Docker и установки Homebrew. |

## Превышение жёсткого лимита памяти PHP останавливает весь сервер

Начиная с PHP 8.2.35, 8.3.36, 8.4.27 и 8.5.12, превышение `memory_limit` больше не останавливает скрипт немедленно: фатальная ошибка "Allowed memory size exhausted" возникает при следующем вызове функции или итерации цикла.
Если скрипт выделяет память за пределы жёсткого лимита памяти до этого, PHP завершает процесс.
Жёсткий лимит по умолчанию составляет двукратный `memory_limit` (или `memory_limit` плюс 1 ГиБ сверх 1 ГиБ) и может быть изменён с помощью настройки INI `memory_hard_limit_percent`.

Код, продолжающий выполняться после `memory_limit`, может достичь жёсткого лимита, например, при одном выделении памяти, превышающем `memory_limit` (таком как `array_merge()` для больших массивов), или при последовательных конкатенациях строк.

PHP-FPM теряет только один дочерний процесс.
FrankenPHP обслуживает все запросы из потоков одного процесса, поэтому каждый обрабатываемый запрос будет потерян, а сервер остановится до перезапуска.
Запускайте FrankenPHP под управлением супервизора, который его перезапускает, например, с помощью политики перезапуска Docker или настройки `Restart=` в systemd (уже установлено в пакетах `deb` и `rpm`).

## get_browser

Функция [get_browser()](https://www.php.net/manual/function.get-browser.php) со временем начинает работать медленно. Решение — кэшировать результаты для каждого User-Agent (например, с помощью [APCu](https://www.php.net/manual/book.apcu.php)), так как они статичны.

## Автономные бинарные файлы и образы на базе Alpine

Полностью статический бинарный файл и образы Docker на базе Alpine (`dunglas/frankenphp:*-alpine`) используют [musl libc](https://musl.libc.org/) вместо [glibc и его друзей](https://www.etalabs.net/compare_libcs.html) для уменьшения размера бинарного файла.
Это может привести к некоторым проблемам совместимости.
В частности, флаг `GLOB_BRACE` [недоступен](https://www.php.net/manual/function.glob.php).

Если вы столкнетесь с проблемами, предпочитайте использовать GNU-вариант статического бинарного файла и образы Docker на базе Debian.

## Использование `https://127.0.0.1` с Docker

По умолчанию FrankenPHP генерирует TLS-сертификат для `localhost`.
Это самый простой и рекомендуемый вариант для локальной разработки.

Если вы действительно хотите использовать `127.0.0.1` в качестве хоста, его можно настроить для генерации сертификата, установив имя сервера в `127.0.0.1`.

К сожалению, этого недостаточно при использовании Docker из-за [его сетевой системы](https://docs.docker.com/network/).
Вы получите ошибку TLS, похожую на `curl: (35) LibreSSL/3.3.6: error:1404B438:SSL routines:ST_CONNECT:tlsv1 alert internal error`.

Если вы используете Linux, решением является использование [драйвера хостовой сети](https://docs.docker.com/network/network-tutorial-host/):

```console
docker run \
    -e SERVER_NAME="127.0.0.1" \
    -v $PWD:/app/public \
    --network host \
    dunglas/frankenphp
```

Драйвер хостовой сети не поддерживается на Mac и Windows. На этих платформах вам придется угадать IP-адрес контейнера и включить его в имена серверов.

Выполните `docker network inspect bridge` и найдите ключ `Containers`, чтобы определить последний текущий назначенный IP-адрес в ключе `IPv4Address`, и увеличьте его на единицу. Если контейнеры не запущены, первый назначенный IP-адрес обычно `172.17.0.2`.

Затем включите это в переменную окружения `SERVER_NAME`:

```console
docker run \
    -e SERVER_NAME="127.0.0.1, 172.17.0.3" \
    -v $PWD:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

> [!CAUTION]
>
> Обязательно замените `172.17.0.3` на IP, который будет присвоен вашему контейнеру.

Теперь вы сможете получить доступ к `https://127.0.0.1` с хост-машины.

Если это не так, запустите FrankenPHP в режиме отладки, чтобы попытаться выяснить проблему:

```console
docker run \
    -e CADDY_GLOBAL_OPTIONS="debug" \
    -e SERVER_NAME="127.0.0.1" \
    -v $PWD:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

## Скрипты Composer с использованием `@php`

[Скрипты Composer](https://getcomposer.org/doc/articles/scripts.md) могут вызывать бинарный файл PHP для выполнения некоторых задач, например, в [проекте Laravel](laravel.md) для запуска `@php artisan package:discover --ansi`. Это [в настоящее время не удается](https://github.com/php/frankenphp/issues/483#issuecomment-1899890915) по двум причинам:

- Composer не знает, как вызывать бинарный файл FrankenPHP;
- Composer может добавлять настройки PHP с помощью флага `-d` в команде, который FrankenPHP пока не поддерживает.

В качестве обходного пути мы можем создать shell-скрипт в `/usr/local/bin/php`, который удаляет неподдерживаемые параметры, а затем вызывает FrankenPHP:

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

Затем установите переменную окружения `PHP_BINARY` на путь к нашему скрипту `php` и запустите Composer:

```console
export PHP_BINARY=/usr/local/bin/php
composer install
```

## Устранение неполадок TLS/SSL со статическими бинарными файлами

При использовании статических бинарных файлов вы можете столкнуться со следующими ошибками, связанными с TLS, например, при отправке электронных писем с использованием STARTTLS:

```text
Unable to connect with STARTTLS: stream_socket_enable_crypto(): SSL operation failed with code 5. OpenSSL Error messages:
error:80000002:system library::No such file or directory
error:80000002:system library::No such file or directory
error:80000002:system library::No such file or directory
error:0A000086:SSL routines::certificate verify failed
```

Поскольку статический бинарный файл не включает TLS-сертификаты, вам необходимо указать OpenSSL путь к локальной установке CA-сертификатов.

Проверьте вывод [`openssl_get_cert_locations()`](https://www.php.net/manual/function.openssl-get-cert-locations.php),
чтобы определить, куда должны быть установлены CA-сертификаты, и поместите их в это место.

> [!WARNING]
>
> Веб- и CLI-контексты могут иметь разные настройки.
> Убедитесь, что вы запускаете `openssl_get_cert_locations()` в соответствующем контексте.

[CA-сертификаты, извлечённые из Mozilla, можно скачать с сайта cURL](https://curl.se/docs/caextract.html).

Кроме того, многие дистрибутивы, включая Debian, Ubuntu и Alpine, предоставляют пакеты с именем `ca-certificates`, которые содержат эти сертификаты.

Также можно использовать `SSL_CERT_FILE` и `SSL_CERT_DIR`, чтобы подсказать OpenSSL, где искать CA-сертификаты:

```console
# Установите переменные окружения для TLS-сертификатов
export SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
export SSL_CERT_DIR=/etc/ssl/certs
