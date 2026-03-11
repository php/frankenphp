# Горячая перезагрузка

FrankenPHP включает встроенную функцию **горячей перезагрузки**, разработанную для значительного улучшения опыта разработчика.

![Горячая перезагрузка](hot-reload.png)

Эта функция обеспечивает рабочий процесс, похожий на **Hot Module Replacement (HMR)** в современных инструментах JavaScript, таких как Vite или webpack.
Вместо ручного обновления браузера после каждого изменения файла (PHP-код, шаблоны, файлы JavaScript и CSS...), FrankenPHP обновляет содержимое страницы в реальном времени.

Горячая перезагрузка нативно работает с WordPress, Laravel, Symfony и любым другим PHP-приложением или фреймворком.

Когда функция включена, FrankenPHP отслеживает изменения файловой системы в вашем текущем рабочем каталоге.
При изменении файла он отправляет обновление [Mercure](mercure.md) в браузер.

В зависимости от вашей настройки, браузер будет либо:

- **Морфировать DOM** (сохраняя позицию прокрутки и состояние ввода), если загружен [Idiomorph](https://github.com/bigskysoftware/idiomorph).
- **Перезагружать страницу** (стандартная живая перезагрузка), если Idiomorph отсутствует.

## Конфигурация

Чтобы включить горячую перезагрузку, включите Mercure, затем добавьте поддирективу `hot_reload` в директиву `php_server` в вашем `Caddyfile`.

> [!WARNING]
>
> Эта функция предназначена **только для сред разработки**.
> Не включайте `hot_reload` в продакшене, так как эта функция небезопасна (раскрывает конфиденциальные внутренние детали) и замедляет работу приложения.
>
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

По умолчанию FrankenPHP будет отслеживать все файлы в текущем рабочем каталоге, соответствующие следующему глобальному шаблону: `./**/*.{css,env,gif,htm,html,jpg,jpeg,js,mjs,php,png,svg,twig,webp,xml,yaml,yml}`

Можно явно задать файлы для отслеживания, используя синтаксис глобов:

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

Используйте полную форму `hot_reload`, чтобы указать используемый топик Mercure, а также каталоги или файлы для отслеживания:

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

## Интеграция на стороне клиента

В то время как сервер обнаруживает изменения, браузер должен подписаться на эти события, чтобы обновить страницу.
FrankenPHP предоставляет URL-адрес Mercure Hub для подписки на изменения файлов через переменную окружения `$_SERVER['FRANKENPHP_HOT_RELOAD']`.

Удобная библиотека JavaScript, [frankenphp-hot-reload](https://www.npmjs.com/package/frankenphp-hot-reload), также доступна для обработки логики на стороне клиента.
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

Библиотека автоматически подпишется на хаб Mercure, получит текущий URL-адрес в фоновом режиме при обнаружении изменения файла и морфирует DOM.
Она доступна как [npm](https://www.npmjs.com/package/frankenphp-hot-reload) пакет и на [GitHub](https://github.com/dunglas/frankenphp-hot-reload).

В качестве альтернативы вы можете реализовать свою собственную клиентскую логику, подписавшись непосредственно на хаб Mercure с помощью нативного JavaScript-класса `EventSource`.

### Сохранение существующих узлов DOM

В редких случаях, например, при использовании инструментов разработки [таких как панель отладки Symfony web debug toolbar](https://github.com/symfony/symfony/pull/62970),
вы можете захотеть сохранить определенные узлы DOM.
Для этого добавьте атрибут `data-frankenphp-hot-reload-preserve` к соответствующему HTML-элементу:

```html
<div data-frankenphp-hot-reload-preserve><!-- Моя панель отладки --></div>
```

## Режим Worker

Если вы запускаете свое приложение в [режиме Worker](https://frankenphp.dev/docs/worker/), скрипт вашего приложения остается в памяти.
Это означает, что изменения в вашем PHP-коде не будут отражены немедленно, даже если браузер перезагрузится.

Для лучшего опыта разработчика вы должны объединить `hot_reload` с [поддирективой `watch` в директиве `worker`](config.md#watching-for-file-changes).

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

## Как это работает

1.  **Отслеживание**: FrankenPHP отслеживает изменения файловой системы, используя библиотеку [`e-dant/watcher`](https://github.com/e-dant/watcher) (мы внесли вклад в Go-биндинг).
2.  **Перезапуск (Режим Worker)**: если `watch` включен в конфигурации воркера, PHP-воркер перезапускается для загрузки нового кода.
3.  **Отправка**: JSON-полезная нагрузка, содержащая список измененных файлов, отправляется во встроенный [Mercure hub](https://mercure.rocks).
4.  **Получение**: Браузер, прослушивающий через JavaScript-библиотеку, получает событие Mercure.
5.  **Обновление**:

- Если обнаружен **Idiomorph**, он получает обновленное содержимое и морфирует текущий HTML, чтобы соответствовать новому состоянию, применяя изменения мгновенно без потери состояния.
- В противном случае вызывается `window.location.reload()` для обновления страницы.
