# Hot Reload

FrankenPHP includes a built-in **hot reload** feature designed to vastly improve the developer experience.

![Mercure](hot-reload.png)

This feature provides a workflow similar to **Hot Module Replacement (HMR)** found in modern JavaScript tooling (like Vite or webpack).
Instead of manually refreshing the browser after every file change (PHP code, templates, JavaScript and CSS files...),
FrankenPHP updates the content in real-time.

Hot Reload natively works with WordPress, Laravel, Symfony, and any other PHP application or framework.

When enabled, FrankenPHP watches your current working directory for filesystem changes.
When a file is modified, it pushes a [Mercure](mercure.md) update to the browser.

Depending on your setup, the browser will either:

- **Morph the DOM** (preserving scroll position and input state) if [Idiomorph](https://github.com/bigskysoftware/idiomorph) is loaded.
- **Reload the page** (standard live reload) if Idiomorph is not present.

## Configuration

To enable hot reloading, enable Mercure, then add the `hot_reload` sub-directive to the `php_server` directive in your `Caddyfile`.

> [!WARNING]
> This feature is intended for **development environments only**.
> Do not enable `hot_reload` in production, as watching the filesystem incurs performance overhead and exposes internal endpoints.

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

By default, FrankenPHP will watch all files in the current working directory matching this glob pattern: `./**/*.{css,env,gif,htm,html,jpg,jpeg,js,mjs,php,png,svg,twig,webp,xml,yaml,yml}`

It's possible to explicitly set the files to watch using the glob syntax:

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

Use the long form to specify the Mercure topic to use as well as which directories or files to watch by providing paths to the `hot_reload` option:

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

## Client-Side Integration

While the server detects changes, the browser needs to subscribe to these events to update the page.
FrankenPHP exposes the Mercure Hub URL to use for subscribing to file changes via the `$_SERVER['FRANKENPHP_HOT_RELOAD']` environment variable.

A convenience JavaScript library, [frankenphp-hot-reload](https://www.npmjs.com/package/frankenphp-hot-reload), is also available to handle the client-side logic.
To use it, add the following to your main layout:

```php
<!DOCTYPE html>
<title>FrankenPHP Hot Reload</title>
<?php if (isset($_SERVER['FRANKENPHP_HOT_RELOAD']): ?>
<meta name="frankenphp-hot-reload:url" content="<?=$_SERVER['FRANKENPHP_HOT_RELOAD']?>">
<script src="https://cdn.jsdelivr.net/npm/idiomorph"></script>
<script src="https://cdn.jsdelivr.net/npm/frankenphp-hot-reload/+esm" type="module"></script>
<?php endif ?>
```

The library will automatically subscribe to the Mercure hub, fetch the current URL in the background when a file change is detected and morph the DOM.
It is available as a [npm](https://www.npmjs.com/package/frankenphp-hot-reload) package and on [GitHub](https://github.com/dunglas/frankenphp-hot-reload).

Alternatively, you can implement your own client-side logic by subscribing directly to the Mercure hub using the `EventSource` native JavaScript class.

### Worker Mode

If you are running your application in [Worker Mode](https://frankenphp.dev/docs/worker/), your application script remains in memory.
This means changes to your PHP code will not be reflected immediately, even if the browser reloads.

For the best developer experience, you should combine `hot_reload` with [the `watch` sub-directive in the `worker` directive](config.md#watching-for-file-changes).

- `hot_reload`: refreshes the **browser** when files change
- `worker.watch`: restarts the worker when files change

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

### How it works

1. **Watch**: FrankenPHP monitors the filesystem for modifications using [the `e-dant/watcher` library](https://github.com/e-dant/watcher) under the hood (we contributed the Go binding).
2. **Restart (Worker Mode)**: if `watch` is enabled in the worker config, the PHP worker is restarted to load the new code.
3. **Push**: a JSON payload containing the list of changed files is sent to the built-in [Mercure hub](https://mercure.rocks).
4. **Receive**: The browser, listening via the JavaScript library, receives the Mercure event.
5. **Update**:

- If **Idiomorph** is detected, it fetches the updated content and morphs the current HTML to match the new state, applying changes instantly without losing state.
- Otherwise, `window.location.reload()` is called to refresh the page.
