# WordPress

[WordPress](https://wordpress.org/)'i FrankenPHP ile çalıştırarak otomatik HTTPS, HTTP/3 ve Zstandard sıkıştırma özelliklerine sahip modern, yüksek performanslı bir yığının keyfini çıkarın.

## Minimal Kurulum

1.  [WordPress'i İndirin](https://wordpress.org/download/)
2.  ZIP arşivini çıkarın ve çıkarılan dizinde bir terminal açın
3.  Çalıştırın:

    ```console
    frankenphp php-server
    ```

4.  `http://localhost/wp-admin/` adresine gidin ve kurulum talimatlarını izleyin
5.  Keyfini çıkarın!

Üretime hazır bir kurulum için, `frankenphp run` komutunu aşağıdaki gibi bir `Caddyfile` ile kullanmayı tercih edin:

```caddyfile
example.com

php_server
encode zstd br gzip
log
```

## Anında Yenileme

WordPress ile [anında yenileme](hot-reload.md) özelliğini kullanmak için, [Mercure](mercure.md)'u etkinleştirin ve `Caddyfile` dosyanızdaki `php_server` direktifine `hot_reload` alt direktifini ekleyin:

```caddyfile
localhost

mercure {
    anonymous
}

php_server {
    hot_reload
}
```

Ardından, JavaScript kütüphanelerini WordPress temanızın `functions.php` dosyasına yüklemek için gerekli kodu ekleyin:

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

Son olarak, WordPress kök dizininden `frankenphp run` komutunu çalıştırın.
