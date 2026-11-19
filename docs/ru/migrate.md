---
title: Переход с Nginx и PHP-FPM на FrankenPHP
description: Пошаговое руководство по миграции PHP-приложения со стека Nginx плюс PHP-FPM на FrankenPHP, охватывающее Caddyfile, php.ini, потоки и Docker.
---

# Переход с Nginx/PHP-FPM

FrankenPHP заменяет как ваш веб-сервер (Nginx, Apache), так и PHP-FPM одним бинарным файлом. Это руководство охватывает базовую миграцию для типичного PHP-приложения.

## Ключевые отличия

| Настройка PHP-FPM                 | Эквивалент FrankenPHP                                    |
| --------------------------------- | -------------------------------------------------------- |
| Nginx/Apache + PHP-FPM            | Один бинарный файл `frankenphp`                          |
| Настройки пула `php-fpm.conf`     | [Глобальная опция `frankenphp`](config.md#caddyfile-config) |
| Блок `server {}` Nginx            | Блок сайта `Caddyfile`                                   |
| `php_value` / `php_admin_value`   | [Директива `php_ini` в Caddyfile](config.md#php-config)  |
| `pm = static` / `pm.max_children` | `num_threads`                                            |
| `pm = dynamic`                    | [`max_threads auto`](performance.md#max_threads)         |
| Фильтрация запросов веб-сервером  | Маршруты и матчеры Caddy                                 |

## Фильтрация HTTP-запросов

При миграции со стандартного пакета Nginx или Apache проверьте любую фильтрацию запросов, которая выполнялась веб-сервером до того, как PHP-FPM получал запрос. FrankenPHP построен на Caddy, поэтому действительные HTTP-методы и заголовки проходят через обычную маршрутизацию Caddy, если только ваш `Caddyfile` или другой прокси-сервер не отклонит их первым.

Например, некоторые веб-серверы по умолчанию отклоняют `TRACE` запросы. Если ваше приложение должно сохранить такое поведение, добавьте явный матчер перед `php_server`:

```caddyfile
example.com {
    @trace method TRACE
    respond @trace 405

    root /var/www/app/public
    php_server
}
```

Примените тот же подход к любой более строгой политике методов или заголовков, которую обеспечивал ваш предыдущий веб-сервер. То же самое относится к именам заголовков запросов: некоторые веб-серверы или прокси-серверы отклоняют или отбрасывают допустимые, но необычные имена HTTP-полей до того, как PHP-FPM их увидит, в то время как FrankenPHP может передать их PHP-приложению.

## Шаг 1: замените конфигурацию веб-сервера

Типичная конфигурация Nginx + PHP-FPM:

```nginx
# /etc/nginx/sites-available/example.com
server {
    listen 80;
    server_name example.com;
    root /var/www/app/public;
    index index.php;

    location / {
        try_files $uri $uri/ /index.php$is_args$args;
    }

    location ~ \.php$ {
        fastcgi_pass unix:/run/php/php-fpm.sock;
        fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
        include fastcgi_params;
    }
}
```

Превращается в единственный `Caddyfile`:

```caddyfile
example.com {
    root /var/www/app/public
    php_server
}
```

Вот и всё. Директива `php_server` обрабатывает маршрутизацию PHP, поведение, похожее на `try_files`, и обслуживание статических файлов.

## Шаг 2: перенесите конфигурацию PHP

Ваш существующий `php.ini` работает как есть. Смотрите [Конфигурация](config.md), чтобы узнать, куда его поместить в зависимости от вашего метода установки.

Одно значение меняет смысл: `max_execution_time` считает реальное время в FrankenPHP, тогда как сборки PHP-FPM без ZTS в Linux считают время ЦП. Ожидание запроса к базе данных, HTTP-вызова, `sleep()` или блокировки файла сессии теперь учитывается в лимите, поэтому значение, настроенное для PHP-FPM, может быть слишком низким.

Вы также можете установить директивы непосредственно в `Caddyfile`:

```caddyfile
{
    frankenphp {
        php_ini memory_limit 256M
        php_ini max_execution_time 30
    }
}

example.com {
    root /var/www/app/public
    php_server
}
```

## Шаг 3: настройте размер пула

В PHP-FPM вы настраиваете `pm.max_children` для управления количеством рабочих процессов. В FrankenPHP эквивалентом является `num_threads`:

```caddyfile
{
    frankenphp {
        num_threads 16
    }
}
```

По умолчанию FrankenPHP запускает 2 потока на ЦП. Для динамического масштабирования, подобного `pm = dynamic` в PHP-FPM:

```caddyfile
{
    frankenphp {
        num_threads 4
        max_threads auto
    }
}
```

## Шаг 4: миграция Docker

Типичная настройка Docker с PHP-FPM, использующая два контейнера (Nginx + PHP-FPM), может быть заменена одним контейнером:

**До:**

```yaml
# compose.yaml
services:
  nginx:
    image: nginx:1
    volumes:
      # монтирование конфигурации Nginx в контейнер
      - ./config:/etc/nginx/conf.d
      - .:/var/www/app
    ports:
      - "80:80"
      - "443:443"

  php:
    image: php:8.5-fpm
    volumes:
      - .:/var/www/app
```

**После:**

```yaml
# compose.yaml
services:
  php:
    image: dunglas/frankenphp:1-php8.5
    volumes:
      - .:/var/www/app
      # монтирование Caddyfile в контейнер
      - ./config:/etc/frankenphp
      - caddy_data:/data
      - caddy_config:/config
    ports:
      - "80:80"
      - "443:443"
      - "443:443/udp"

volumes:
  caddy_data:
  caddy_config:
```

Если вам нужны дополнительные расширения PHP, смотрите [Создание пользовательского образа Docker](docker.md#how-to-install-more-php-extensions).

Для специфичных для фреймворков настроек Docker смотрите [Symfony Docker](https://github.com/dunglas/symfony-docker) и [запуск Laravel с образом Docker FrankenPHP](laravel.md#running-laravel-with-the-frankenphp-docker-image).

## Шаг 5: рассмотрите режим воркера (необязательно)

В [классическом режиме](classic.md) FrankenPHP работает как PHP-FPM: каждое приложение загружается с нуля при каждом запросе. Это безопасная отправная точка для миграции.

Для лучшей производительности вы можете переключиться в [режим воркера](worker.md), который загружает ваше приложение один раз и сохраняет его в памяти:

```caddyfile
example.com {
    root /var/www/app/public
    php_server {
        root /var/www/app/public
        worker index.php 4
    }
}
```

> [!CAUTION]
>
> Режим воркера сохраняет ваше приложение в памяти между запросами. Убедитесь, что ваш код не полагается на сброс глобального состояния между запросами. Фреймворки, такие как [Symfony](worker.md#worker-mode-for-symfony), [Laravel](laravel.md#laravel-octane) и [API Platform](https://api-platform.com), имеют нативную поддержку этого режима.

## Что можно удалить

После миграции вам больше не понадобятся:

- Nginx или Apache
- PHP-FPM (сервис/процесс `php-fpm`)
- Конфигурация FastCGI
- Самостоятельно управляемые TLS-сертификаты (Caddy обрабатывает их автоматически)
