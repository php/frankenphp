# Создание статической сборки

Вместо использования локальной установки библиотеки PHP, можно создать статическую или в основном статическую сборку FrankenPHP благодаря проекту [static-php-cli](https://github.com/crazywhalecc/static-php-cli) (несмотря на название, этот проект поддерживает все SAPI, а не только CLI).

С помощью этого метода единый, переносимый бинарный файл будет содержать PHP-интерпретатор, веб-сервер Caddy и FrankenPHP!

Полностью статические нативные исполняемые файлы не требуют никаких зависимостей и могут быть запущены даже на образе Docker [`scratch`](https://docs.docker.com/build/building/base-images/#create-a-minimal-base-image-using-scratch). Однако они не могут загружать динамические PHP-расширения (такие как Xdebug) и имеют некоторые ограничения, поскольку используют musl libc.

В основном статические бинарные файлы требуют только `glibc` и могут загружать динамические расширения.

По возможности мы рекомендуем использовать в основном статические сборки на базе glibc.

FrankenPHP также поддерживает [встраивание PHP-приложения в статический бинарный файл](embed.md).

## Linux

Мы предоставляем образы Docker для сборки статических бинарных файлов для Linux:

### Полностью статическая сборка на базе musl

Для полностью статического бинарного файла, который работает на любом дистрибутиве Linux без зависимостей, но не поддерживает динамическую загрузку расширений:

```console
docker buildx bake --load static-builder-musl
docker cp $(docker create --name static-builder-musl dunglas/frankenphp:static-builder-musl):/go/src/app/dist/frankenphp-linux-$(uname -m) frankenphp ; docker rm static-builder-musl
```

Для лучшей производительности в сценариях с высокой конкуренцией рассмотрите возможность использования аллокатора [mimalloc](https://github.com/microsoft/mimalloc).

```console
docker buildx bake --load --set static-builder-musl.args.MIMALLOC=1 static-builder-musl
```

### В основном статическая сборка на базе glibc (с поддержкой динамических расширений)

Для бинарного файла, который поддерживает динамическую загрузку PHP-расширений, при этом выбранные расширения скомпилированы статически:

```console
docker buildx bake --load static-builder-gnu
docker cp $(docker create --name static-builder-gnu dunglas/frankenphp:static-builder-gnu):/go/src/app/dist/frankenphp-linux-$(uname -m) frankenphp ; docker rm static-builder-gnu
```

Этот бинарный файл поддерживает все версии glibc 2.17 и выше, но не работает в системах на базе musl (таких как Alpine Linux).

Результирующий в основном статический (за исключением `glibc`) бинарный файл называется `frankenphp` и доступен в текущей директории.

Если вы хотите собрать статический бинарный файл без Docker, ознакомьтесь с инструкциями для macOS, которые также работают для Linux.

### Пользовательские расширения

По умолчанию компилируются самые популярные PHP-расширения.

Чтобы уменьшить размер бинарного файла и сократить поверхность атаки, вы можете выбрать список расширений для сборки, используя Docker-аргумент `PHP_EXTENSIONS`.

Например, выполните следующую команду, чтобы собрать только расширение `opcache`:

```console
docker buildx bake --load --set static-builder-musl.args.PHP_EXTENSIONS=opcache,pdo_sqlite static-builder-musl
# ...
```

Чтобы добавить библиотеки, обеспечивающие дополнительную функциональность для включенных вами расширений, вы можете передать Docker-аргумент `PHP_EXTENSION_LIBS`:

```console
docker buildx bake \
  --load \
  --set static-builder-musl.args.PHP_EXTENSIONS=gd \
  --set static-builder-musl.args.PHP_EXTENSION_LIBS=libjpeg,libwebp \
  static-builder-musl
```

### Дополнительные модули Caddy

Чтобы добавить дополнительные модули Caddy или передать другие аргументы в [xcaddy](https://github.com/caddyserver/xcaddy), используйте Docker-аргумент `XCADDY_ARGS`:

```console
docker buildx bake \
  --load \
  --set static-builder-musl.args.XCADDY_ARGS="--with github.com/darkweak/souin/plugins/caddy --with github.com/dunglas/caddy-cbrotli --with github.com/dunglas/mercure/caddy --with github.com/dunglas/vulcain/caddy" \
  static-builder-musl
```

В этом примере мы добавляем модуль HTTP-кэширования [Souin](https://souin.io) для Caddy, а также модули [cbrotli](https://github.com/dunglas/caddy-cbrotli), [Mercure](https://mercure.rocks) и [Vulcain](https://vulcain.rocks).

> [!TIP]
>
> Модули cbrotli, Mercure и Vulcain включены по умолчанию, если `XCADDY_ARGS` пуст или не установлен.
> Если вы настраиваете значение `XCADDY_ARGS`, вы должны явно включить их, если хотите, чтобы они были добавлены.

См. также, как [настроить сборку](#настройка-сборки)

### Токен GitHub

Если вы достигли лимита запросов к API GitHub, задайте личный токен доступа GitHub в переменной окружения `GITHUB_TOKEN`:

```console
GITHUB_TOKEN="xxx" docker --load buildx bake static-builder-musl
# ...
```

## macOS

Запустите следующий скрипт, чтобы создать статический бинарный файл для macOS (у вас должен быть установлен [Homebrew](https://brew.sh/)):

```console
git clone https://github.com/php/frankenphp
cd frankenphp
./build-static.sh
```

Примечание: этот скрипт также работает на Linux (и, вероятно, на других Unix-системах) и используется внутри предоставленных нами образов Docker.

## Настройка сборки

Следующие переменные окружения можно передать в `docker build` и скрипт `build-static.sh`, чтобы настроить статическую сборку:

- `FRANKENPHP_VERSION`: версия FrankenPHP
- `PHP_VERSION`: версия PHP
- `PHP_EXTENSIONS`: PHP-расширения для сборки ([список поддерживаемых расширений](https://static-php.dev/en/guide/extensions.html))
- `PHP_EXTENSION_LIBS`: дополнительные библиотеки, добавляющие функциональность расширениям
- `XCADDY_ARGS`: аргументы для [xcaddy](https://github.com/caddyserver/xcaddy), например, для добавления дополнительных модулей Caddy
- `EMBED`: путь к PHP-приложению для встраивания в бинарник
- `CLEAN`: если задано, libphp и все его зависимости будут пересобраны с нуля (без кэша)
- `NO_COMPRESS`: отключает сжатие результирующего бинарника с помощью UPX
- `DEBUG_SYMBOLS`: если задано, отладочные символы не будут удалены и будут добавлены в бинарник
- `MIMALLOC`: (экспериментально, только для Linux) заменяет musl's mallocng на [mimalloc](https://github.com/microsoft/mimalloc) для повышения производительности. Мы рекомендуем использовать это только для сборок, нацеленных на musl; для glibc предпочтительнее отключить эту опцию и использовать [`LD_PRELOAD`](https://microsoft.github.io/mimalloc/overrides.html) при запуске вашего бинарного файла.
- `RELEASE`: (только для мейнтейнеров) если задано, результирующий бинарник будет загружен на GitHub

## Расширения

С бинарными файлами на базе glibc или macOS вы можете динамически загружать PHP-расширения. Однако эти расширения должны быть скомпилированы с поддержкой ZTS. Поскольку большинство менеджеров пакетов в настоящее время не предлагают ZTS-версии своих расширений, вам придется компилировать их самостоятельно.

Для этого вы можете собрать и запустить контейнер Docker `static-builder-gnu`, подключиться к нему удаленно и скомпилировать расширения с помощью `./configure --with-php-config=/go/src/app/dist/static-php-cli/buildroot/bin/php-config`.

Пример шагов для [расширения Xdebug](https://xdebug.org):

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

Это создаст `frankenphp` и `xdebug-zts.so` в текущей директории. Если вы переместите `xdebug-zts.so` в каталог ваших расширений, добавите `zend_extension=xdebug-zts.so` в ваш php.ini и запустите FrankenPHP, он загрузит Xdebug.
