# Использование воркеров FrankenPHP

Загрузите приложение один раз и держите его в памяти.
FrankenPHP будет обрабатывать входящие запросы за несколько миллисекунд.

## Запуск worker-скриптов

### Docker

Установите значение переменной окружения `FRANKENPHP_CONFIG` на `worker /path/to/your/worker/script.php`:

```console
docker run \
    -e FRANKENPHP_CONFIG="worker /app/path/to/your/worker/script.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### Автономный бинарный файл

Используйте опцию `--worker` команды `php-server` для обслуживания содержимого текущей директории в режиме воркера:

```console
frankenphp php-server --worker /path/to/your/worker/script.php
```

Если ваше PHP-приложение [встроено в бинарник](embed.md), вы можете добавить пользовательский `Caddyfile` в корневую директорию приложения.
Он будет использоваться автоматически.

Также можно [перезапускать воркер при изменении файлов](config.md#watching-for-file-changes) с помощью опции `--watch`.
Следующая команда выполнит перезапуск, если будет изменён любой файл с расширением `.php` в директории `/path/to/your/app/` или её поддиректориях:

```console
frankenphp php-server --worker /path/to/your/worker/script.php --watch="/path/to/your/app/**/*.php"
```

Эта функция часто используется в сочетании с [горячей перезагрузкой](hot-reload.md).

## Symfony Runtime

> [!СОВЕТ]
> Следующий раздел актуален только для версий Symfony до 7.4, где была введена нативная поддержка режима воркеров FrankenPHP.

Worker режим FrankenPHP поддерживается компонентом [Symfony Runtime](https://symfony.com/doc/current/components/runtime.html).
Чтобы запустить любое Symfony-приложение в worker режиме, установите пакет FrankenPHP для [PHP Runtime](https://github.com/php-runtime/runtime):

```console
composer require runtime/frankenphp-symfony
```

Запустите сервер приложения, задав переменную окружения `APP_RUNTIME` для использования FrankenPHP Symfony Runtime:

```console
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php" \
    -e APP_RUNTIME=Runtime\\FrankenPhpSymfony\\Runtime \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

## Laravel Octane

Подробнее см. в [отдельной документации](laravel.md#laravel-octane).

## Пользовательские приложения

Следующий пример показывает, как создать собственный worker-скрипт без использования сторонних библиотек:

```php
<?php
// public/index.php

// Инициализация вашего приложения
require __DIR__.'/vendor/autoload.php';

$myApp = new \App\Kernel();
$myApp->boot();

// Обработчик запросов за пределами цикла для повышения производительности (выполняет меньше работы)
$handler = static function () use ($myApp) {
    try {
        // Выполняется при получении запроса,
        // суперглобальные переменные, php://input и прочие сбрасываются
        echo $myApp->handle($_GET, $_POST, $_COOKIE, $_FILES, $_SERVER);
    } catch (\Throwable $exception) {
        // `set_exception_handler` вызывается только когда worker-скрипт завершается,
        // что может быть не тем, что вы ожидаете, поэтому перехватывайте и обрабатывайте исключения здесь
        (new \MyCustomExceptionHandler)->handleException($exception);
    }
};

$maxRequests = (int)($_SERVER['MAX_REQUESTS'] ?? 0);
for ($nbRequests = 0; !$maxRequests || $nbRequests < $maxRequests; ++$nbRequests) {
    $keepRunning = \frankenphp_handle_request($handler);

    // Действия после отправки HTTP-ответа
    $myApp->terminate();

    // Вызов сборщика мусора, чтобы снизить вероятность его запуска в процессе генерации страницы
    gc_collect_cycles();

    if (!$keepRunning) break;
}

// Очистка
$myApp->shutdown();
```

Запустите ваше приложение и используйте переменную окружения `FRANKENPHP_CONFIG` для настройки вашего воркера:

```console
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

По умолчанию запускается 2 воркера на каждый CPU. Вы также можете настроить количество запускаемых воркеров:

```console
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php 42" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### Перезапуск worker-скрипта после определённого количества запросов

PHP изначально не предназначался для долгоживущих процессов, поэтому есть ещё много библиотек и устаревшего кода, которые вызывают утечки памяти.
Обходной путь для использования такого кода в режиме воркера состоит в перезапуске скрипта воркера после обработки определённого количества запросов:

Предыдущий сниппет воркера позволяет настроить максимальное количество запросов для обработки, установив переменную окружения с именем `MAX_REQUESTS`.

### Перезапуск воркеров вручную

Хотя можно перезапускать воркеры [при изменении файлов](config.md#watching-for-file-changes), также можно корректно перезапустить все воркеры через [API администрирования Caddy](https://caddyserver.com/docs/api). Если администрирование включено в вашем [Caddyfile](config.md#caddyfile-config), вы можете отправить запрос POST на конечную точку перезапуска следующим образом:

```console
curl -X POST http://localhost:2019/frankenphp/workers/restart
```

### Сбои worker-скрипта

Если worker-скрипт завершится с ненулевым кодом выхода, FrankenPHP перезапустит его со стратегией экспоненциальной задержки.
Если скрипт воркера остается активным дольше, чем время последней задержки \* 2, FrankenPHP не будет применять штраф к worker-скрипту и перезапустит его снова.
Однако, если worker-скрипт продолжает завершаться с ненулевым кодом выхода в течение короткого промежутка времени
(например, из-за опечатки в скрипте), FrankenPHP завершит работу с ошибкой: `too many consecutive failures`.

Количество последовательных сбоев можно настроить в вашем [Caddyfile](config.md#caddyfile-config) с помощью опции `max_consecutive_failures`:

```caddyfile
frankenphp {
    worker {
        # ...
        max_consecutive_failures 10
    }
}
```

## Поведение суперглобальных переменных

[PHP суперглобальные переменные](https://www.php.net/manual/en/language.variables.superglobals.php) (`$_SERVER`, `$_ENV`, `$_GET`...) ведут себя следующим образом:

- до первого вызова `frankenphp_handle_request()`, суперглобальные переменные содержат значения, связанные с самим worker-скриптом
- во время и после вызова `frankenphp_handle_request()`, суперглобальные переменные содержат значения, сгенерированные на основе обработанного HTTP-запроса, каждый вызов `frankenphp_handle_request()` изменяет значения суперглобальных переменных

Чтобы получить доступ к суперглобальным переменным worker-скрипта внутри колбэка, необходимо скопировать их и импортировать копию в область видимости колбэка:

```php
<?php
// Копирование суперглобальной переменной $_SERVER воркера перед первым вызовом frankenphp_handle_request()
$workerServer = $_SERVER;

$handler = static function () use ($workerServer) {
    var_dump($_SERVER); // $_SERVER, привязанный к запросу
    var_dump($workerServer); // $_SERVER worker-скрипта
};

// ...
