# Создание статических бинарных файлов

Вместо использования локальной установки библиотеки PHP, можно создать статическую сборку FrankenPHP благодаря проекту [static-php-cli](https://github.com/crazywhalecc/static-php-cli) (несмотря на название, проект поддерживает все SAPI, а не только CLI).

С помощью этого метода создаётся единый переносимый бинарник, который включает PHP-интерпретатор, веб-сервер Caddy и FrankenPHP!

FrankenPHP также поддерживает [встраивание PHP-приложений в статический бинарный файл](embed.md).

## Linux

Мы предоставляем Docker-образ для сборки статического бинарника для Linux:

```console
docker buildx bake --load static-builder
docker cp $(docker create --name static-builder-musl dunglas/frankenphp:static-builder-musl):/go/src/app/dist/frankenphp-linux-$(uname -m) frankenphp ; docker rm static-builder
```

Созданный статический бинарный файл называется `frankenphp` и будет доступен в текущей директории.

Чтобы собрать статический бинарный файл без Docker, используйте инструкции для macOS — они подходят и для Linux.

### Дополнительные расширения

По умолчанию компилируются самые популярные PHP-расширения.

Чтобы уменьшить размер бинарного файла и сократить возможные векторы атак, можно указать список расширений, которые следует включить в сборку, используя Docker-аргумент `PHP_EXTENSIONS`.

Например, выполните следующую команду, чтобы собрать только расширение `opcache`:

```console
docker buildx bake --load --set static-builder.args.PHP_EXTENSIONS=opcache,pdo_sqlite static-builder
# ...
```

Чтобы добавить библиотеки, расширяющие функциональность включённых расширений, используйте Docker-аргумент `PHP_EXTENSION_LIBS`:

```console
docker buildx bake \
  --load \
  --set static-builder.args.PHP_EXTENSIONS=gd \
  --set static-builder.args.PHP_EXTENSION_LIBS=libjpeg,libwebp \
  static-builder
```

### Дополнительные модули Caddy

Чтобы добавить дополнительные модули Caddy или передать аргументы в [xcaddy](https://github.com/caddyserver/xcaddy), используйте Docker-аргумент `XCADDY_ARGS`:

```console
docker buildx bake \
  --load \
  --set static-builder.args.XCADDY_ARGS="--with github.com/darkweak/souin/plugins/caddy --with github.com/dunglas/caddy-cbrotli --with github.com/dunglas/mercure/caddy --with github.com/dunglas/vulcain/caddy" \
  static-builder
```

В этом примере добавляются модуль HTTP-кэширования [Souin](https://souin.io) для Caddy, а также модули [cbrotli](https://github.com/dunglas/caddy-cbrotli), [Mercure](https://mercure.rocks) и [Vulcain](https://vulcain.rocks).

> [!TIP]
>
> Модули cbrotli, Mercure и Vulcain включены по умолчанию, если `XCADDY_ARGS` пуст или не установлен.  
> Если вы изменяете значение `XCADDY_ARGS`, добавьте их явно, если хотите включить их в сборку.

См. также, как [настроить сборку](#настройка-сборки).

### Токен GitHub

Если вы достигли лимита запросов к API GitHub, задайте личный токен доступа GitHub в переменной окружения `GITHUB_TOKEN`:

```console
GITHUB_TOKEN="xxx" docker --load buildx bake static-builder
# ...
```

## macOS

Запустите следующий скрипт, чтобы создать статический бинарный файл для macOS (должен быть установлен [Homebrew](https://brew.sh/)):

```console
git clone https://github.com/php/frankenphp
cd frankenphp
./build-static.sh
```

Примечание: этот скрипт также работает на Linux (и, вероятно, на других Unix-системах) и используется внутри предоставленного Docker-образа для статической сборки.

## Настройка сборки

Следующие переменные окружения можно передать в `docker build` и скрипт `build-static.sh`, чтобы настроить статическую сборку:

- `FRANKENPHP_VERSION`: версия FrankenPHP
- `PHP_VERSION`: версия PHP
- `PHP_EXTENSIONS`: PHP-расширения для сборки ([список поддерживаемых расширений](https://static-php.dev/en/guide/extensions.html))
- `PHP_EXTENSION_LIBS`: дополнительные библиотеки, добавляющие функциональность расширениям
- `XCADDY_ARGS`: аргументы для [xcaddy](https://github.com/caddyserver/xcaddy), например, для добавления модулей Caddy
- `EMBED`: путь к PHP-приложению для встраивания в бинарник
- `CLEAN`: если задано, libphp и все его зависимости будут пересобраны с нуля (без кэша)
- `NO_COMPRESS`: отключает сжатие результирующего бинарника с помощью UPX
- `DEBUG_SYMBOLS`: если задано, отладочные символы не будут удалены и будут добавлены в бинарник
- `MIMALLOC`: (экспериментально, только для Linux) заменяет musl's mallocng на [mimalloc](https://github.com/microsoft/mimalloc) для повышения производительности
- `RELEASE`: (только для мейнтейнеров) если задано, бинарник будет загружен на GitHub
