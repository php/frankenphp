# WordPress

使用 FrankenPHP 运行 [WordPress](https://wordpress.org/)，享受现代、高性能的堆栈，具备自动 HTTPS、HTTP/3 和 Zstandard 压缩功能。

## 最小安装

1. [下载 WordPress](https://wordpress.org/download/)
2. 解压 ZIP 存档并在解压后的目录中打开终端
3. 运行：

   ```console
   frankenphp php-server
   ```

4. 访问 `http://localhost/wp-admin/` 并按照安装说明进行操作
5. 享受吧！

对于生产环境就绪的设置，请优先使用 `frankenphp run` 配合如下 `Caddyfile`：

```caddyfile
example.com

php_server
encode zstd br gzip
log
```

## 热重载

要将 [热重载](hot-reload.md) 功能与 WordPress 配合使用，请启用 [Mercure](mercure.md) 并在您的 `Caddyfile` 中为 `php_server` 指令添加 `hot_reload` 子指令：

```caddyfile
localhost

mercure {
    anonymous
}

php_server {
    hot_reload
}
```

然后，在您的 WordPress 主题的 `functions.php` 文件中添加加载 JavaScript 库所需的代码：

```php
function hot_reload() {
    ?>
    <?php if (isset($_SERVER['FRANKENPHP_HOT_RELOAD'])): ?>
        <meta name="frankenphp-hot-reload:url" content="<?=$_SERVER['FRANKENPHP_HOT_RELOAD']?>">
        <script src="https://cdn.jsdelivr.net/npm/idiomorph"></script>
        <script src="https://cdn.jsdelivr.net/npm/frankenphp-hot-reload/+esm" type="module"></script>
    <?php endif ?>
    <?php
}
add_action('wp_head', 'hot_reload');
```

最后，从 WordPress 根目录运行 `frankenphp run`。
