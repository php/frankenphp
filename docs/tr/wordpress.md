# WordPress

[WordPress](https://wordpress.org/)'i FrankenPHP ile çalıştırarak otomatik HTTPS, HTTP/3 ve Zstandard sıkıştırmalı modern, yüksek performanslı bir yığının tadını çıkarın.

## WordPress'i FrankenPHP ile kurma

1. [WordPress'i indirin](https://wordpress.org/download/)
2. ZIP arşivini çıkarın ve çıkarılan dizinde bir terminal açın
3. Şunu çalıştırın:

   ```console
   frankenphp php-server
   ```

4. `http://localhost/wp-admin/` adresine gidin ve kurulum yönergelerini izleyin
5. Tadını çıkarın!

Üretime hazır bir kurulum için `frankenphp run` ile şöyle bir `Caddyfile` kullanmayı tercih edin:

```caddyfile
example.com

php_server
encode zstd br gzip
log
```

## WordPress için sıcak yeniden yükleme

[Sıcak yeniden yükleme](hot-reload.md) özelliğini WordPress ile kullanmak için [Mercure](mercure.md)'ü etkinleştirin ve `Caddyfile`'ınızdaki `php_server` yönergesine `hot_reload` alt yönergesini ekleyin:

```caddyfile
localhost

mercure {
    anonymous
}

php_server {
    hot_reload
}
```

Ardından WordPress temanızın `functions.php` dosyasına JavaScript kütüphanelerini yüklemek için gereken kodu ekleyin:

```php
// wp-content/themes/<your-theme>/functions.php
function hot_reload() {
    ?>
    <?php if (isset($_SERVER['FRANKENPHP_HOT_RELOAD'])): ?>
        <meta name="frankenphp-hot-reload:url" content="<?=$_SERVER['FRANKENPHP_HOT_RELOAD']?>">
        <script src="https://cdn.jsdelivr.net/npm/idiomorph/dist/idiomorph.min.js"></script>
        <script src="https://cdn.jsdelivr.net/npm/frankenphp-hot-reload/+esm" type="module"></script>
    <?php endif ?>
    <?php
}
add_action('wp_head', 'hot_reload');
```

Son olarak WordPress kök dizininden `frankenphp run` çalıştırın.
