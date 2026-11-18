---
title: Статическая сборка FrankenPHP: PHP-сервер как единый бинарный файл
description: Создавайте статические или преимущественно статические бинарные файлы FrankenPHP со встроенным PHP и Caddy на Linux и macOS, настраивайте расширения и загружайте динамические модули с glibc.
---

# Создание статической сборки

Вместо использования локальной установки библиотеки PHP можно создать статическую или преимущественно статическую сборку FrankenPHP благодаря отличному проекту [static-php-cli](https://github.com/crazywhalecc/static-php-cli) (несмотря на название, этот проект поддерживает все SAPI, а не только CLI).

С помощью этого метода единый, переносимый бинарный файл будет содержать PHP-интерпретатор, веб-сервер Caddy и FrankenPHP!

Полностью статичные нативные исполняемые файлы не требуют никаких зависимостей и могут быть запущены даже на [`scratch` Docker-образе](https://docs.docker.com/build/building/base-images/#create-a-minimal-base-image-using-scratch). Однако они не могут загружать динамические PHP-расширения (такие как Xdebug) и имеют некоторые ограничения из-за использования musl libc.

Преимущественно статичные бинарные файлы требуют только `glibc` и могут загружать динамические расширения.

По возможности мы рекомендуем использовать преимущественно статичные сборки на основе glibc.

FrankenPHP также поддерживает [встраивание PHP-приложения в статический бинарный файл](embed.md).

## Linux

Мы предоставляем Docker-образы для сборки статических бинарных файлов для Linux:

### Полностью статичная сборка на основе musl

Для полностью статичного бинарного файла, который работает в любом дистрибутиве Linux без зависимостей, но не поддерживает динамическую загрузку расширений:

```console
docker buildx bake --load static-builder-musl
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log dunglas/frankenphp:static-builder-musl
cp "output/frankenphp-linux-$(uname -m)" frankenphp
```

Для лучшей производительности в сценариях с высокой конкуренцией рассмотрите возможность использования аллокатора [mimalloc](https://github.com/microsoft/mimalloc).

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log -e MIMALLOC=1 dunglas/frankenphp:static-builder-musl
```

### Преимущественно статичная сборка на основе glibc (с поддержкой динамических расширений)

Для бинарного файла, который поддерживает динамическую загрузку PHP-расширений, при этом имея выбранные расширения, скомпилированные статически:

```console
docker buildx bake --load static-builder-gnu
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log dunglas/frankenphp:static-builder-gnu
cp "output/frankenphp-linux-$(uname -m)" frankenphp
```

Этот бинарный файл поддерживает все версии glibc 2.28 и выше, но не работает на системах на основе musl (например, Alpine Linux).

Полученный преимущественно статичный (за исключением `glibc`) бинарный файл называется `frankenphp` и находится в текущей директории.

Если сборка не удалась, логи static-php-cli находятся в `output/log/`.

Если вы хотите собрать статический бинарный файл без Docker, обратитесь к инструкциям для macOS, которые также работают для Linux.

### Пользовательские PHP-расширения в статической сборке

По умолчанию компилируются самые популярные PHP-расширения. Настройки по умолчанию находятся в [`craft.yml`](https://github.com/php/frankenphp/blob/main/craft.yml).

Чтобы уменьшить размер бинарного файла и сократить поверхность атаки, вы можете выбрать список расширений для сборки, используя переменную `PHP_EXTENSIONS`.

Например, выполните следующую команду, чтобы собрать только расширение `opcache`:

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log -e PHP_EXTENSIONS=opcache,pdo_sqlite dunglas/frankenphp:static-builder-musl
# ...
```

Чтобы добавить библиотеки, обеспечивающие дополнительную функциональность для включённых вами расширений, вы можете передать переменную `PHP_EXTENSION_LIBS`:

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log \
  -e PHP_EXTENSIONS=gd \
  -e PHP_EXTENSION_LIBS=libjpeg,libwebp \
  dunglas/frankenphp:static-builder-musl
```

### Дополнительные модули Caddy

Чтобы добавить дополнительные модули Caddy или передать другие аргументы в [xcaddy](https://github.com/caddyserver/xcaddy), используйте переменную `XCADDY_ARGS`:

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log \
  -e XCADDY_ARGS="--with github.com/darkweak/souin/plugins/caddy --with github.com/dunglas/caddy-cbrotli --with github.com/dunglas/mercure/caddy --with github.com/dunglas/vulcain/caddy" \
  dunglas/frankenphp:static-builder-musl
```

В этом примере мы добавляем модуль HTTP-кэширования [Souin](https://souin.io) для Caddy, а также модули [cbrotli](https://github.com/dunglas/caddy-cbrotli), [Mercure](https://mercure.rocks) и [Vulcain](https://vulcain.rocks).

> [!TIP]
>
> Модули cbrotli, Mercure и Vulcain включены по умолчанию, если `XCADDY_ARGS` пуст или не установлен.
> Если вы настраиваете значение `XCADDY_ARGS`, вы должны явно включить их, если хотите, чтобы они были добавлены.

См. также, как [настроить статическую сборку FrankenPHP](#настройка-статической-сборки-frankenphp)

### Токен GitHub

Если вы достигли лимита запросов к API GitHub, установите персональный токен доступа GitHub в переменной окружения с именем `GITHUB_TOKEN`:

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log -e GITHUB_TOKEN="xxx" dunglas/frankenphp:static-builder-musl
# ...
```

## macOS

Запустите следующий скрипт, чтобы создать статический бинарный файл для macOS (у вас должен быть установлен [Homebrew](https://brew.sh/)):

```console
git clone https://github.com/php/frankenphp
cd frankenphp
./build-static.sh
```

Примечание: этот скрипт также работает на Linux (и, вероятно, на других Unix-системах) и используется внутри предоставляемых нами Docker-образов.

Мы публикуем бинарные файлы macOS только для Apple Silicon. На компьютерах Mac с процессором Intel соберите из исходного кода с помощью этого скрипта.

## Настройка статической сборки FrankenPHP

Следующие переменные окружения могут быть переданы в `docker run` и в скрипт `build-static.sh` для настройки статической сборки:

- `FRANKENPHP_VERSION`: используемая версия FrankenPHP
- `PHP_VERSION`: используемая версия PHP
- `PHP_EXTENSIONS`: PHP-расширения для сборки ([список поддерживаемых расширений](https://static-php.dev/en/guide/extensions.html))
- `PHP_EXTENSION_LIBS`: дополнительные библиотеки для сборки, которые добавляют функции расширениям
- `XCADDY_ARGS`: аргументы для передачи в [xcaddy](https://github.com/caddyserver/xcaddy), например, для добавления дополнительных модулей Caddy
- `EMBED`: путь к PHP-приложению для встраивания в бинарный файл
- `CLEAN`: если установлено, libphp и все его зависимости будут собраны с нуля (без кэша)
- `COMPRESS`: если установлено в `1`, упаковывает полученный бинарный файл с помощью UPX (только для Linux; игнорируется, если установлен `DEBUG_SYMBOLS`)
- `DEBUG_SYMBOLS`: если установлено, отладочные символы не будут удалены и будут добавлены в бинарный файл
- `MIMALLOC`: (экспериментально, только для Linux) заменяет musl's mallocng на [mimalloc](https://github.com/microsoft/mimalloc) для повышения производительности. Мы рекомендуем использовать это только для сборок, ориентированных на musl; для glibc предпочтительнее отключить эту опцию и использовать [`LD_PRELOAD`](https://microsoft.github.io/mimalloc/overrides.html) при запуске бинарного файла.
- `RELEASE`: (только для мейнтейнеров) если установлено, полученный бинарный файл будет загружен на GitHub
- `LIBC`: (только для Linux) `musl` для полностью статического бинарного файла (по умолчанию), или `gnu` для динамической компоновки с системной glibc. Это сокращение для `SPC_TARGET` и `SPC_TOOLCHAIN` из static-php-cli, которые по-прежнему могут быть установлены напрямую; они должны быть реальными переменными окружения, потому что static-php-cli разрешает свой инструментарий до чтения `craft.yml`. По умолчанию `musl` ориентируется на базовый процессор архитектуры (`x86_64-linux-musl` или `aarch64-linux-musl`), чтобы бинарный файл работал на любом процессоре; установите `SPC_TARGET=native-native-musl` для оптимизации под сборочную машину.

## Динамическая загрузка PHP-расширений в статический бинарный файл

С бинарными файлами на основе glibc или macOS вы можете динамически загружать PHP-расширения. Однако эти расширения должны быть скомпилированы с поддержкой ZTS.
Поскольку большинство менеджеров пакетов в настоящее время не предлагают ZTS-версии своих расширений, вам придется компилировать их самостоятельно.

Для этого вы можете запустить Docker-контейнер `static-builder-gnu`, подключиться к нему удаленно и скомпилировать расширения с помощью `./configure --with-php-config=/go/src/app/dist/buildroot/bin/php-config`.

Пример шагов для [расширения Xdebug](https://xdebug.org):

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

Это создаст `frankenphp` и `xdebug-zts.so` в текущей директории.
Если вы переместите `xdebug-zts.so` в директорию расширений, добавите `zend_extension=xdebug-zts.so` в ваш php.ini и запустите FrankenPHP, он загрузит Xdebug.
