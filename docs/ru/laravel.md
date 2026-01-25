# Laravel

## Docker

Запустить [Laravel](https://laravel.com) веб-приложение с FrankenPHP очень просто: достаточно смонтировать проект в директорию `/app` официального Docker-образа.

Выполните эту команду из корневой директории вашего Laravel-приложения:

```console
docker run -p 80:80 -p 443:443 -p 443:443/udp -v $PWD:/app dunglas/frankenphp
```

И наслаждайтесь!

## Локальная установка

Вы также можете запустить ваши Laravel-проекты с FrankenPHP на локальной машине:

1. [Скачайте бинарный файл для вашей системы](README.md#автономный-бинарный-файл)
2. Добавьте следующую конфигурацию в файл с именем `Caddyfile` в корневой директории вашего Laravel-проекта:

   ```caddyfile
   {
   	frankenphp
   }

   # Доменное имя вашего сервера
   localhost {
   	# Укажите веб-корень как директорию public/
   	root public/
   	# Включите сжатие (опционально)
   	encode zstd br gzip
   	# Выполняйте PHP-файлы из директории public/ и обслуживайте статические файлы
   	php_server
   }
   ```

3. Запустите FrankenPHP из корневой директории вашего Laravel-проекта: `frankenphp run`

## Laravel Octane

Octane можно установить с помощью менеджера пакетов Composer:

```console
composer require laravel/octane
```

После установки Octane выполните Artisan-команду `octane:install`, которая создаст конфигурационный файл Octane в вашем приложении:

```console
php artisan octane:install --server=frankenphp
```

Сервер Octane можно запустить с помощью Artisan-команды `octane:frankenphp`:

```console
php artisan octane:frankenphp
```

Команда `octane:frankenphp` поддерживает следующие опции:

- `--host`: IP-адрес, к которому должен привязаться сервер (по умолчанию: `127.0.0.1`)
- `--port`: Порт, на котором сервер будет доступен (по умолчанию: `8000`)
- `--admin-port`: Порт, на котором будет доступен административный сервер (по умолчанию: `2019`)
- `--workers`: Количество worker-скриптов для обработки запросов (по умолчанию: `auto`)
- `--max-requests`: Количество запросов, обрабатываемых перед перезагрузкой сервера (по умолчанию: `500`)
- `--caddyfile`: Путь к файлу `Caddyfile` FrankenPHP (по умолчанию: [stubbed `Caddyfile` в Laravel Octane](https://github.com/laravel/octane/blob/2.x/src/Commands/stubs/Caddyfile))
- `--https`: Включить HTTPS, HTTP/2 и HTTP/3, а также автоматически генерировать и обновлять сертификаты
- `--http-redirect`: Включить редирект с HTTP на HTTPS (включается только при передаче --https)
- `--watch`: Автоматически перезагружать сервер при изменении приложения
- `--poll`: Использовать опрос файловой системы для отслеживания изменений в файлах через сеть
- `--log-level`: Установить уровень логирования, используя встроенный логгер Caddy

> [!TIP]
> Чтобы получить структурированные JSON-логи (полезно при использовании решений для анализа логов), явно укажите опцию `--log-level`.

Подробнее о [Laravel Octane читайте в официальной документации](https://laravel.com/docs/octane).

## Laravel-приложения как автономные бинарные файлы

Используя [возможность встраивания приложений в FrankenPHP](embed.md), можно распространять Laravel-приложения как автономные бинарные файлы.

Следуйте этим шагам, чтобы упаковать ваше Laravel-приложение в автономный бинарный файл для Linux:

1. Создайте файл с именем `static-build.Dockerfile` в репозитории вашего приложения:

   ```dockerfile
   FROM --platform=linux/amd64 dunglas/frankenphp:static-builder-gnu
   # Если вы планируете запускать бинарный файл на системах с musl-libc, используйте static-builder-musl

   # Скопируйте ваше приложение
   WORKDIR /go/src/app/dist/app
   COPY . .

   # Удалите тесты и другие ненужные файлы, чтобы сэкономить место
   # В качестве альтернативы добавьте эти файлы в .dockerignore
   RUN rm -Rf tests/

   # Скопируйте файл .env
   RUN cp .env.example .env
   # Измените APP_ENV и APP_DEBUG для продакшна
   RUN sed -i'' -e 's/^APP_ENV=.*/APP_ENV=production/' -e 's/^APP_DEBUG=.*/APP_DEBUG=false/' .env

   # Внесите другие изменения в файл .env, если необходимо

   # Установите зависимости
   RUN composer install --ignore-platform-reqs --no-dev -a

   # Соберите статический бинарный файл
   WORKDIR /go/src/app/
   RUN EMBED=dist/app/ ./build-static.sh
   ```

   > [!CAUTION]
   >
   > Некоторые `.dockerignore` файлы могут игнорировать директорию `vendor/` и файлы `.env`. Убедитесь, что вы скорректировали или удалили `.dockerignore` перед сборкой.

2. Соберите:

   ```console
   docker build -t static-laravel-app -f static-build.Dockerfile .
   ```

3. Извлеките бинарный файл:

   ```console
   docker cp $(docker create --name static-laravel-app-tmp static-laravel-app):/go/src/app/dist/frankenphp-linux-x86_64 frankenphp ; docker rm static-laravel-app-tmp
   ```

4. Заполните кеши:

   ```console
   frankenphp php-cli artisan optimize
   ```

5. Запустите миграции базы данных (если есть):

   ```console
   frankenphp php-cli artisan migrate
   ```

6. Сгенерируйте секретный ключ приложения:

   ```console
   frankenphp php-cli artisan key:generate
   ```

7. Запустите сервер:

   ```console
   frankenphp php-server
   ```

Ваше приложение готово!

Узнайте больше о доступных опциях и о том, как собирать бинарные файлы для других ОС в [документации по встраиванию приложений](embed.md).

### Изменение пути хранения

По умолчанию Laravel сохраняет загруженные файлы, кеши, логи и другие данные в директории `storage/` приложения. Это неудобно для встроенных приложений, так как каждая новая версия будет извлекаться в другую временную директорию.

Установите переменную окружения `LARAVEL_STORAGE_PATH` (например, в вашем `.env` файле) или вызовите метод `Illuminate\Foundation\Application::useStoragePath()`, чтобы использовать директорию за пределами временной директории.

### Запуск Octane как автономный бинарный файл

Можно даже упаковать приложения Laravel Octane как автономный бинарный файл!

Для этого [установите Octane правильно](#laravel-octane) и следуйте шагам, описанным в [предыдущем разделе](#laravel-приложения-как-автономные-бинарные-файлы).

Затем, чтобы запустить FrankenPHP в worker-режиме через Octane, выполните:

```console
PATH="$PWD:$PATH" frankenphp php-cli artisan octane:frankenphp
```

> [!CAUTION]
> Для работы команды автономный бинарник **обязательно** должен быть назван `frankenphp`, так как Octane требует наличия программы с именем `frankenphp` в PATH.
