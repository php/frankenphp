# WordPress

Запускайте [WordPress](https://wordpress.org/) с FrankenPHP, чтобы насладиться современным, высокопроизводительным стеком с автоматическим HTTPS, HTTP/3 и сжатием Zstandard.

## Минимальная установка

1. [Скачайте WordPress](https://wordpress.org/download/)
2. Извлеките ZIP-архив и откройте терминал в извлеченной директории
3. Запустите:

   ```console
   frankenphp php-server
   ```

4. Перейдите по адресу `http://localhost/wp-admin/` и следуйте инструкциям по установке
5. Наслаждайтесь!

Для готовой к продакшену установки предпочтительнее использовать `frankenphp run` с таким `Caddyfile`:

```caddyfile
example.com

php_server
encode zstd br gzip
log
```

## Горячая перезагрузка

Чтобы использовать функцию [горячей перезагрузки](hot-reload.md) с WordPress, включите [Mercure](mercure.md) и добавьте поддирективу `hot_reload` к директиве `php_server` в вашем `Caddyfile`:

```caddyfile
localhost

mercure {
    anonymous
}

php_server {
    hot_reload
}
```

Затем добавьте код, необходимый для загрузки JavaScript-библиотек, в файл `functions.php` вашей темы WordPress:

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

Наконец, запустите `frankenphp run` из корневой директории WordPress.
