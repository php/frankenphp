# Горячая перезагрузка (Hot Reload)

FrankenPHP включает встроенную функцию **горячей перезагрузки (hot reload)**, разработанную для значительного улучшения опыта разработчика.

![Mercure](hot-reload.png)

Эта функция предоставляет рабочий процесс, схожий с **Hot Module Replacement (HMR)**, который встречается в современных инструментах JavaScript (таких как Vite или webpack).
Вместо ручного обновления браузера после каждого изменения файла (PHP-кода, шаблонов, файлов JavaScript и CSS...),
FrankenPHP обновляет содержимое в реальном времени.

Горячая перезагрузка нативно работает с WordPress, Laravel, Symfony и любым другим PHP-приложением или фреймворком.

При включении FrankenPHP отслеживает текущую рабочую директорию на предмет изменений в файловой системе.
Когда файл модифицируется, он отправляет обновление [Mercure](mercure.md) в браузер.

В зависимости от вашей настройки, браузер будет либо:

- **Преобразовывать DOM** (сохраняя позицию прокрутки и состояние ввода), если загружен [Idiomorph](https://github.com/bigskysoftware/idiomorph).
- **Перезагружать страницу** (стандартная живая перезагрузка), если Idiomorph отсутствует.

## Конфигурация

Чтобы включить горячую перезагрузку, активируйте Mercure, затем добавьте поддирективу `hot_reload` в директиву `php_server` в вашем `Caddyfile`.

> [!WARNING]
> Эта функция предназначена **только для сред разработки**.
> Не включайте `hot_reload` в продакшене, так как отслеживание файловой системы влечет за собой накладные расходы на производительность и открывает внутренние конечные точки.

```caddyfile
localhost

mercure {
    anonymous
}

root public/
php_server {
    hot_reload
}
```

По умолчанию FrankenPHP будет отслеживать все файлы в текущей рабочей директории, соответствующие этому шаблону glob: `./**/*.{css,env,gif,htm,html,jpg,jpeg,js,mjs,php,png,svg,twig,webp,xml,yaml,yml}`

Можно явно задать файлы для отслеживания, используя синтаксис glob:

```caddyfile
localhost

mercure {
    anonymous
}

root public/
php_server {
    hot_reload src/**/*{.php,.js} config/**/*.yaml
}
```

Используйте полную форму для указания темы Mercure, а также каталогов или файлов для отслеживания, предоставляя пути опции `hot_reload`:

```caddyfile
localhost

mercure {
    anonymous
}

root public/
php_server {
    hot_reload {
        topic hot-reload-topic
        watch src/**/*.php
        watch assets/**/*.{ts,json}
        watch templates/
        watch public/css/
    }
}
```

## Клиентская интеграция

В то время как сервер обнаруживает изменения, браузеру необходимо подписаться на эти события для обновления страницы.
FrankenPHP предоставляет URL-адрес Mercure Hub для подписки на изменения файлов через переменную окружения `$_SERVER['FRANKENPHP_HOT_RELOAD']`.

Удобная JavaScript-библиотека [frankenphp-hot-reload](https://www.npmjs.com/package/frankenphp-hot-reload) также доступна для обработки логики на стороне клиента.
Чтобы использовать ее, добавьте следующее в ваш основной макет:

```php
<!DOCTYPE html>
<title>FrankenPHP Hot Reload</title>
<?php if (isset($_SERVER['FRANKENPHP_HOT_RELOAD'])): ?>
<meta name="frankenphp-hot-reload:url" content="<?=$_SERVER['FRANKENPHP_HOT_RELOAD']?>">
<script src="https://cdn.jsdelivr.net/npm/idiomorph"></script>
<script src="https://cdn.jsdelivr.net/npm/frankenphp-hot-reload/+esm" type="module"></script>
<?php endif ?>
```

Библиотека автоматически подпишется на Mercure hub, получит текущий URL в фоновом режиме при обнаружении изменения файла и преобразует DOM.
Она доступна как [npm](https://www.npmjs.com/package/frankenphp-hot-reload) пакет и на [GitHub](https://github.com/dunglas/frankenphp-hot-reload).

В качестве альтернативы вы можете реализовать свою собственную клиентскую логику, подписавшись непосредственно на Mercure hub, используя нативный JavaScript-класс `EventSource`.

### Режим воркера

Если вы запускаете свое приложение в [режиме воркера (Worker Mode)](https://frankenphp.dev/docs/worker/), скрипт вашего приложения остается в памяти.
Это означает, что изменения в вашем PHP-коде не будут отражены немедленно, даже если браузер перезагрузится.

Для наилучшего опыта разработки вы должны комбинировать `hot_reload` с [поддирективой `watch` в директиве `worker`](config.md#watching-for-file-changes).

- `hot_reload`: обновляет **браузер** при изменении файлов
- `worker.watch`: перезапускает воркер при изменении файлов

```caddy
localhost

mercure {
    anonymous
}

root public/
php_server {
    hot_reload
    worker {
        file /path/to/my_worker.php
        watch
    }
}
```

### Как это работает

1.  **Отслеживание**: FrankenPHP отслеживает файловую систему на предмет модификаций, используя под капотом [библиотеку `e-dant/watcher`](https://github.com/e-dant/watcher) (мы внесли свой вклад в Go-бинденг).
2.  **Перезапуск (режим воркера)**: если `watch` включен в конфигурации воркера, PHP-воркер перезапускается для загрузки нового кода.
3.  **Отправка**: JSON-полезная нагрузка, содержащая список измененных файлов, отправляется во встроенный [Mercure hub](https://mercure.rocks).
4.  **Получение**: Браузер, слушающий через JavaScript-библиотеку, получает событие Mercure.
5.  **Обновление**:

    - Если обнаружен **Idiomorph**, он получает обновленное содержимое и преобразует текущий HTML, чтобы он соответствовал новому состоянию, применяя изменения мгновенно без потери состояния.
    - В противном случае вызывается `window.location.reload()` для обновления страницы.
