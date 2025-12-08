# PHP-приложения как автономные бинарные файлы

FrankenPHP позволяет встраивать исходный код и ресурсы PHP-приложений в статический автономный бинарный файл.

Благодаря этой функции PHP-приложения могут распространяться как автономные бинарные файлы, которые содержат само приложение, интерпретатор PHP и Caddy — веб-сервер уровня продакшн.

Подробнее об этой функции [в презентации Кевина на SymfonyCon 2023](https://dunglas.dev/2023/12/php-and-symfony-apps-as-standalone-binaries/).

Для встраивания Laravel-приложений ознакомьтесь с [документацией](laravel.md#laravel-приложения-как-автономные-бинарные-файлы).

## Подготовка приложения

Перед созданием автономного бинарного файла убедитесь, что ваше приложение готово для встраивания.

Например, вам может понадобиться:

- Установить продакшн-зависимости приложения.
- Сгенерировать автозагрузчик.
- Включить продакшн-режим приложения (если он есть).
- Удалить ненужные файлы, такие как `.git` или тесты, чтобы уменьшить размер итогового бинарного файла.

Для приложения на Symfony это может выглядеть так:

```console
# Экспорт проекта, чтобы избавиться от .git/ и других ненужных файлов
mkdir $TMPDIR/my-prepared-app
git archive HEAD | tar -x -C $TMPDIR/my-prepared-app
cd $TMPDIR/my-prepared-app

# Установить соответствующие переменные окружения
echo APP_ENV=prod > .env.local
echo APP_DEBUG=0 >> .env.local

# Удалить тесты и другие ненужные файлы
rm -Rf tests/

# Установить зависимости
composer install --ignore-platform-reqs --no-dev -a

# Оптимизировать .env
composer dump-env prod
```

### Настройка конфигурации

Чтобы настроить [конфигурацию](config.md), вы можете разместить файлы `Caddyfile` и `php.ini` в основной директории приложения (`$TMPDIR/my-prepared-app` в примере выше).

## Создание бинарного файла для Linux

Самый простой способ создать бинарный файл для Linux — использовать предоставленный Docker-билдер.

1. Создайте файл `static-build.Dockerfile` в репозитории вашего приложения:

   ```dockerfile
   FROM --platform=linux/amd64 dunglas/frankenphp:static-builder-gnu
   # Если вы планируете запускать бинарный файл на системах с musl-libc, используйте static-builder-musl

   # Скопировать приложение
   WORKDIR /go/src/app/dist/app
   COPY . .

   # Сборка статического бинарного файла
   WORKDIR /go/src/app/
   RUN EMBED=/go/src/app/dist/app/ ./build-static.sh
   ```

   > [!CAUTION]
   >
   > Некоторые `.dockerignore` файлы (например, [Symfony Docker `.dockerignore`](https://github.com/dunglas/symfony-docker/blob/main/.dockerignore))  
   > игнорируют директорию `vendor/` и файлы `.env`. Перед сборкой убедитесь, что `.dockerignore` файл настроен корректно или удалён.

2. Соберите образ:

   ```console
   docker build -t static-app -f static-build.Dockerfile .
   ```

3. Извлеките бинарный файл:

   ```console
   docker cp $(docker create --name static-app-tmp static-app):/go/src/app/dist/frankenphp-linux-x86_64 my-app ; docker rm static-app-tmp
   ```

Созданный бинарный файл сохранится в текущей директории под именем `my-app`.

## Создание бинарного файла для других ОС

Если вы не хотите использовать Docker или хотите собрать бинарный файл для macOS, используйте предоставленный скрипт:

```console
git clone https://github.com/php/frankenphp
cd frankenphp
EMBED=/path/to/your/app ./build-static.sh
```

Итоговый бинарный файл будет находиться в директории `dist/` под именем `frankenphp-<os>-<arch>`.

## Использование бинарного файла

Готово! Файл `my-app` (или `dist/frankenphp-<os>-<arch>` для других ОС) содержит ваше автономное приложение.

Для запуска веб-приложения выполните:

```console
./my-app php-server
```

Если ваше приложение содержит [worker-скрипт](worker.md), запустите его следующим образом:

```console
./my-app php-server --worker public/index.php
```

Чтобы включить HTTPS (Let's Encrypt автоматически создаст сертификат), HTTP/2 и HTTP/3, укажите доменное имя:

```console
./my-app php-server --domain localhost
```

Вы также можете запускать PHP-скрипты CLI, встроенные в бинарный файл:

```console
./my-app php-cli bin/console
```

## PHP-расширения

По умолчанию скрипт собирает расширения, указанные в `composer.json` вашего проекта.  
Если файла `composer.json` нет, собираются стандартные расширения, как указано в [документации по статической сборке](static.md).

Чтобы настроить список расширений, используйте переменную окружения `PHP_EXTENSIONS`.

## Настройка сборки

[Ознакомьтесь с документацией по статической сборке](static.md), чтобы узнать, как настроить бинарный файл (расширения, версию PHP и т.д.).

## Распространение бинарного файла

На Linux созданный бинарный файл сжимается с помощью [UPX](https://upx.github.io).

На Mac для уменьшения размера файла перед отправкой его можно сжать. Рекомендуется использовать `xz`.
