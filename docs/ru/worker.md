---
title: Режим воркеров FrankenPHP: сохраняйте PHP-приложение в памяти
description: Запускайте FrankenPHP в режиме воркеров, чтобы сохранять ваше PHP-приложение инициализированным между запросами, сокращать накладные расходы на запуск и обслуживать ответы за миллисекунды.
---

# Использование воркеров FrankenPHP

Загрузите приложение один раз и держите его в памяти.
FrankenPHP будет обрабатывать входящие запросы за несколько миллисекунд.

## Запуск worker-скриптов FrankenPHP

### Запуск worker-скрипта FrankenPHP с Docker

Установите значение переменной окружения `FRANKENPHP_CONFIG` на `worker /path/to/your/worker/script.php`:

```bash
docker run \
    -e FRANKENPHP_CONFIG="worker /app/path/to/your/worker/script.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### Запуск worker-скрипта FrankenPHP с автономным бинарным файлом

Используйте опцию `--worker` команды `php-server` для обслуживания содержимого текущей директории в режиме воркера:

```bash
frankenphp php-server --worker /path/to/your/worker/script.php
```

Если ваше PHP-приложение [встроено в бинарник](embed.md), вы можете добавить пользовательский `Caddyfile` в корневую директорию приложения.
Он будет использоваться автоматически.

Также можно [перезапускать воркер при изменении файлов](config.md#отслеживание-изменений-файлов) с помощью опции `--watch`.
Следующая команда выполнит перезапуск, если будет изменён любой файл с расширением `.php` в директории `/path/to/your/app/` или её поддиректориях:

```bash
frankenphp php-server --worker /path/to/your/worker/script.php --watch="/path/to/your/app/**/*.php"
```

Эта функция часто используется в сочетании с [горячей перезагрузкой](hot-reload.md).

## Worker-режим для Symfony

См. [документацию по worker-режиму FrankenPHP для Symfony](symfony.md#symfony-worker-режим-с-frankenphp).

## Worker-режим для Laravel Octane

См. [документацию по Laravel Octane для FrankenPHP](laravel.md#laravel-octane).

## Написание пользовательского worker-скрипта FrankenPHP

Следующий пример показывает, как создать собственный worker-скрипт без использования сторонних библиотек:

```php
<?php
// public/index.php

// Инициализируйте ваше приложение
require __DIR__.'/vendor/autoload.php';

$myApp = new \App\Kernel();
$myApp->boot();

// Обработчик за пределами цикла для лучшей производительности (выполняет меньше работы)
$handler = static function () use ($myApp) {
    try {
        // Вызывается при получении запроса,
        // суперглобальные переменные, php://input и подобные сбрасываются
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

    // Сделайте что-либо после отправки HTTP-ответа
    $myApp->terminate();

    // Вызовите сборщик мусора, чтобы уменьшить вероятность его запуска в середине генерации страницы
    gc_collect_cycles();

    if (!$keepRunning) break;
}

// Очистка
$myApp->shutdown();
```

Запустите ваше приложение и используйте переменную окружения `FRANKENPHP_CONFIG` для настройки вашего воркера:

```bash
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

По умолчанию запускается 2 воркера на каждый CPU.
Вы также можете настроить количество запускаемых воркеров:

```bash
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php 42" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### Использование PSR-15

Если ваше приложение использует [PSR-15](https://www.php-fig.org/psr/psr-15/) вместо суперглобальных переменных, преобразуйте запрос/ответ на границах обработчика с помощью [`nyholm/psr7`](https://github.com/Nyholm/psr7) и [`nyholm/psr7-server`](https://github.com/Nyholm/psr7-server):

```console
composer require nyholm/psr7 nyholm/psr7-server psr/http-server-handler
```

```php
<?php
// public/index.php

require __DIR__.'/vendor/autoload.php';

use Nyholm\Psr7\Factory\Psr17Factory;
use Nyholm\Psr7Server\ServerRequestCreator;

$myApp = new \App\Kernel(); // реализует Psr\Http\Server\RequestHandlerInterface
$myApp->boot();

$psr17Factory = new Psr17Factory();
$creator = new ServerRequestCreator(
    $psr17Factory, // ServerRequestFactory
    $psr17Factory, // UriFactory
    $psr17Factory, // UploadedFileFactory
    $psr17Factory, // StreamFactory
);

$handler = static function () use ($myApp, $creator) {
    $response = $myApp->handle($creator->fromGlobals());

    http_response_code($response->getStatusCode());
    foreach ($response->getHeaders() as $name => $values) {
        foreach ($values as $value) {
            header("$name: $value", false);
        }
    }
    echo $response->getBody();
};

$maxRequests = (int)($_SERVER['MAX_REQUESTS'] ?? 0);
for ($nbRequests = 0; !$maxRequests || $nbRequests < $maxRequests; ++$nbRequests) {
    $keepRunning = \frankenphp_handle_request($handler);
    gc_collect_cycles();
    if (!$keepRunning) break;
}
```

### Перезапуск worker-скрипта после определённого количества запросов

Поскольку PHP изначально не предназначался для долгоживущих процессов, многие библиотеки и устаревший код всё ещё вызывают утечки памяти.
Обходной путь для использования такого кода в режиме воркера состоит в перезапуске скрипта воркера после обработки определённого количества запросов:

Предыдущий сниппет воркера позволяет настроить максимальное количество запросов для обработки, установив переменную окружения с именем `MAX_REQUESTS`.

### Перезапуск воркеров вручную

Хотя можно перезапускать воркеры [при изменении файлов](config.md#отслеживание-изменений-файлов), также можно корректно перезапустить все воркеры через [API администрирования Caddy](https://caddyserver.com/docs/api). Если администрирование включено в вашем [Caddyfile](config.md#настройка-caddyfile), вы можете отправить запрос POST на конечную точку перезапуска следующим образом:

```bash
curl -X POST http://localhost:2019/frankenphp/workers/restart
```

### Сбои worker-скрипта

Если worker-скрипт завершится с ненулевым кодом выхода, FrankenPHP перезапустит его со стратегией экспоненциальной задержки.
Если скрипт воркера остаётся активным дольше, чем время последней задержки × 2,
FrankenPHP не будет применять штраф к worker-скрипту и перезапустит его снова.
Однако, если worker-скрипт продолжает завершаться с ненулевым кодом выхода в течение короткого промежутка времени
(например, из-за опечатки в скрипте), FrankenPHP завершит работу с ошибкой: `too many consecutive failures`.

Количество последовательных сбоев можно настроить в вашем [Caddyfile](config.md#настройка-caddyfile) с помощью опции `max_consecutive_failures`:

```caddyfile
frankenphp {
    worker {
        # ...
        max_consecutive_failures 10
    }
}
```

### Количество потоков

Когда включен режим воркера, требуется дополнительный поток для обработки входящих запросов,
поэтому значение по умолчанию для `num_threads` составляет `1 + 2x(количество доступных CPU)`.

Таким образом, в этом режиме, если `num_threads` и `num` заданы явно, `num_threads` должно быть строго больше, чем `num`.

## Поведение суперглобальных переменных

[PHP суперглобальные переменные](https://www.php.net/manual/language.variables.superglobals.php) (`$_SERVER`, `$_ENV`, `$_GET`...)
ведут себя следующим образом:

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
```

Большинство суперглобальных переменных (`$_GET`, `$_POST`, `$_COOKIE`, `$_FILES`, `$_SERVER`, `$_REQUEST`) автоматически сбрасываются между запросами.
Однако, **`$_ENV` в настоящее время не сбрасывается между запросами**.
Это означает, что любые изменения, внесённые в `$_ENV` во время запроса, будут сохраняться и быть видны последующим запросам, обрабатываемым тем же потоком воркера.
Избегайте хранения данных, специфичных для запроса или конфиденциальных, в `$_ENV`.

## Сохранение состояния

Поскольку режим воркера сохраняет PHP-процесс активным между запросами, следующее состояние сохраняется между запросами:

- **Статические переменные**: Переменные, объявленные с `static` внутри функций или методов, сохраняют свои значения между запросами.
- **Статические свойства классов**: Статические свойства классов сохраняются между запросами.
- **Глобальные переменные**: Переменные в глобальной области видимости worker-скрипта сохраняются между запросами.
- **Кэши в памяти**: Любые данные, хранящиеся в памяти (массивы, объекты) за пределами обработчика запросов, сохраняются.
- **Настройки среды выполнения**: Изменения, сделанные с помощью `ini_set()`, `stream_context_set_default()`, `date_default_timezone_set()`, `chdir()`, `stream_wrapper_register()`, `set_error_handler()` или `set_exception_handler()` остаются в силе для последующих запросов. Никогда не храните там значения, специфичные для запроса или пользователя, например, учётные данные в контексте потока по умолчанию: вместо этого передавайте явный контекст при каждом вызове.
- **Настройки процесса**: `umask()` и `setlocale()` изменяют весь процесс FrankenPHP, а не только текущий поток. Режим воркера не восстанавливает их между запросами, и в любом режиме они также влияют на запросы, выполняющиеся одновременно. Установите их один раз при запуске или вызывайте `chmod()` для созданных файлов вместо изменения umask.

Это сделано намеренно и является тем, что делает режим воркера быстрым. Однако это требует внимания, чтобы избежать непредвиденных побочных эффектов:

```php
<?php
function getCounter(): int {
    static $count = 0;
    return ++$count; // Увеличивается между запросами!
}

$handler = static function () {
    echo getCounter(); // 1, 2, 3, ... для каждого запроса в этом потоке
};

while (\frankenphp_handle_request($handler)) {
    // ...
}
```

При написании worker-скриптов убедитесь, что вы сбрасываете любое состояние, специфичное для запроса, между запросами.
Фреймворки, такие как [Symfony](symfony.md) и [Laravel Octane](laravel.md), позаботятся о сбросе большинства состояний за вас, но вам всё равно может потребоваться сбросить свои собственные сервисы. В Symfony сервисы, которые содержат состояние, специфичное для запроса, должны реализовывать [`Symfony\Contracts\Service\ResetInterface`](https://github.com/symfony/contracts/blob/main/Service/ResetInterface.php), чтобы они сбрасывались ядром между запросами.
