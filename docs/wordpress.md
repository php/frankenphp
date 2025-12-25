# WordPress

Run [WordPress](https://wordpress.org/) with FrankenPHP to enjoy a modern, high-performance stack with automatic HTTPS, HTTP/3, and Zstandard compression.

## Minimal Installation

1. [Download WordPress](https://wordpress.org/download/)
2. Extract the ZIP archive and open a terminal in the extracted directory
3. Run:
   ```console
   frankenphp php-server
   ```
4. Go to `http://localhost/wp-admin/` and follow the installation instructions
5. Enjoy!

For a production-ready setup, prefer using `frankenphp run` with a `Caddyfile` like this one:

```caddyfile
example.com

php_server
encode zstd br gzip
log
```

## Hot Reload

To use the [hot reload](hot-reload.md) feature with WordPress, enable [Mercure](mercure.md) and add the `hot_reload` sub-directive to the `php_server` directive in your `Caddyfile`:

```caddyfile
localhost

mercure {
    anonymous
}

php_server {
    hot_reload
}
```

Then, add the code needed to load the JavaScript libraries in the `functions.php` file of your WordPress theme:

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

Finally, run `frankenphp run` from the WordPress root directory.
