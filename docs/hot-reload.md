# Hot Reload

FrankenPHP includes a built-in **hot reload** feature designed to vastly improve the developer experience.

This feature provides a workflow similar to **Hot Module Replacement (HMR)** found in modern JavaScript tooling (like Vite or Webpack).
Instead of manually refreshing the browser after every file change (PHP code, templates, JavaScript and CSS files...),
FrankenPHP updates the content in real-time.

Hot Reload natively works with WordPress, Laravel, Symfony, and any other PHP application or framework.

When enabled, FrankenPHP watches your current working directory for filesystem changes.
When a file is modified, it pushes a [Mercure](mercure.md) update to the browser.

Depending on your setup, the browser will either:

* **Morph the DOM** (preserving scroll position and input state) if [Idiomorph](https://github.com/bigskysoftware/idiomorph) is loaded.
* **Reload the page** (standard live reload) if Idiomorph is not present.

## Configuration

To enable hot reloading, enable Mercure, then add the `hot_reload` option to the `php_server` directive in your `Caddyfile`.

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

You can also explicitly specify the Mercure topic to use as well as which directories or files to watch by providing paths to the `hot_reload` option:

```caddyfile
localhost

mercure {
    anonymous
}

root public/
php_server {
    hot_reload {
        topic hot-reload-topic
        watch src/
        watch templates/
        watch public/js/
        watch public/css/
    }
}
```

### Worker Mode

If you are running your application in [Worker Mode](https://frankenphp.dev/docs/worker/), your application script remains in memory.
This means changes to your PHP code will not be reflected immediately, even if the browser reloads.

For the best developer experience, you should combine `hot_reload` with the `watch` option in the `worker` directive.

* `hot_reload`: Refreshes the **browser** when files change.
* `watch`: Refreshes the **application kernel** (restarts the worker) when files change.

```caddy
localhost

mercure {
    anonymous
}

root public/
php_server {
    hot_reload
    worker {
        file my_worker.php
        watch
    }
}
```

## Client-Side Integration

While the server detects changes, the browser needs to subscribe to these events to update the page. FrankenPHP exposes the Mercure Hub URL required for the subscription via the `$_SERVER['FRANKENPHP_HOT_RELOAD']` environment variable.

You must include the URL in a meta tag and load the FrankenPHP Hot Reload library.

Add the following to your main layout or HTML template:

```php
<!DOCTYPE html>
<title>FrankenPHP Hot Reload</title>
<?php if (isset($_SERVER['FRANKENPHP_HOT_RELOAD']): ?>
<meta name="frankenphp-hot-reload:url" content="<?=$_SERVER['FRANKENPHP_HOT_RELOAD']?>">
<script src="https://cdn.jsdelivr.net/npm/idiomorph"></script>
<script src="https://cdn.jsdelivr.net/npm/frankenphp-hot-reload/+esm" type="module"></script>
<?php endif ?>
```

### How it works

1. **Watch**: FrankenPHP monitors the filesystem for modifications.
2. **Restart (Worker Mode)**: If `watch` is enabled in the worker config, the PHP worker is restarted to load the new code.
3. **Push**: A payload containing the list of changed files is sent to the built-in Mercure Hub.
4. **Receive**: The browser, listening via the JS library, receives the event.
5. **Update**:
* If **Idiomorph** is detected, it fetches the updated content and morphs the current HTML to match the new state, applying changes instantly without losing state.
* Otherwise, `window.location.reload()` is called to refresh the page.
