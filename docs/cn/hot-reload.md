# 热重载

FrankenPHP 内置的**热重载**功能旨在极大改善开发者体验。

![Mercure](hot-reload.png)

此功能提供类似于现代 JavaScript 工具（如 Vite 或 webpack）中**模块热替换 (HMR)** 的工作流。
开发者无需在每次文件更改（PHP 代码、模板、JavaScript 和 CSS 文件...）后手动刷新浏览器，
FrankenPHP 会实时更新内容。

热重载原生支持 WordPress、Laravel、Symfony 以及任何其他 PHP 应用或框架。

启用后，FrankenPHP 会监视当前工作目录的文件系统更改。
当文件被修改时，它会将 [Mercure](mercure.md) 更新推送到浏览器。

根据您的设置，浏览器将：

- 如果加载了 [Idiomorph](https://github.com/bigskysoftware/idiomorph)，则**修改 DOM**（保留滚动位置和输入状态）。
- 如果没有 Idiomorph，则**重新加载页面**（标准实时重载）。

## 配置

要启用热重载，请先启用 Mercure，然后在 `Caddyfile` 中的 `php_server` 指令中添加 `hot_reload` 子指令。

> [!WARNING]
> 此功能仅适用于**开发环境**。
> 请勿在生产环境中启用 `hot_reload`，因为监视文件系统会带来性能开销并暴露内部端点。

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

默认情况下，FrankenPHP 将监视当前工作目录中匹配以下全局模式的所有文件：`./**/*.{css,env,gif,htm,html,jpg,jpeg,js,mjs,php,png,svg,twig,webp,xml,yaml,yml}`

可以通过全局语法明确设置要监视的文件：

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

使用长格式来指定要使用的 Mercure 主题以及通过向 `hot_reload` 选项提供路径来监视哪些目录或文件：

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

## 客户端集成

虽然服务器检测到更改，但浏览器需要订阅这些事件才能更新页面。
FrankenPHP 通过 `$_SERVER['FRANKENPHP_HOT_RELOAD']` 环境变量暴露 Mercure Hub URL，用于订阅文件更改。

还提供了一个便利的 JavaScript 库 [frankenphp-hot-reload](https://www.npmjs.com/package/frankenphp-hot-reload) 来处理客户端逻辑。
要使用它，请将以下内容添加到您的主布局中：

```php
<!DOCTYPE html>
<title>FrankenPHP Hot Reload</title>
<?php if (isset($_SERVER['FRANKENPHP_HOT_RELOAD'])): ?>
<meta name="frankenphp-hot-reload:url" content="<?=$_SERVER['FRANKENPHP_HOT_RELOAD']?>">
<script src="https://cdn.jsdelivr.net/npm/idiomorph"></script>
<script src="https://cdn.jsdelivr.net/npm/frankenphp-hot-reload/+esm" type="module"></script>
<?php endif ?>
```

该库将自动订阅 Mercure hub，当检测到文件更改时在后台获取当前 URL 并修改 DOM。
它作为一个 [npm](https://www.npmjs.com/package/frankenphp-hot-reload) 包和在 [GitHub](https://github.com/dunglas/frankenphp-hot-reload) 上可用。

或者，您可以通过使用 `EventSource` 原生 JavaScript 类直接订阅 Mercure hub 来实现自己的客户端逻辑。

### Worker 模式

如果您在 [Worker 模式](https://frankenphp.dev/docs/worker/)下运行您的应用程序，您的应用程序脚本会保留在内存中。
这意味着即使浏览器重新加载，您对 PHP 代码的更改也不会立即反映。

为了获得最佳的开发体验，您应该将 `hot_reload` 与 [worker 指令中的 `watch` 子指令](config.md#watching-for-file-changes)结合使用。

- `hot_reload`：当文件更改时刷新**浏览器**
- `worker.watch`：当文件更改时重启 worker

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

### 工作原理

1.  **监视**：FrankenPHP 使用底层 [e-dant/watcher 库](https://github.com/e-dant/watcher)监视文件系统修改（我们贡献了 Go 绑定）。
2.  **重启 (Worker 模式)**：如果在 worker 配置中启用了 `watch`，PHP worker 将重新启动以加载新代码。
3.  **推送**：包含更改文件列表的 JSON 有效载荷被发送到内置的 [Mercure hub](https://mercure.rocks)。
4.  **接收**：浏览器通过 JavaScript 库监听，接收 Mercure 事件。
5.  **更新**：

- 如果检测到 **Idiomorph**，它会获取更新的内容并修改当前的 HTML 以匹配新状态，即时应用更改而不会丢失状态。
- 否则，将调用 `window.location.reload()` 以刷新页面。
