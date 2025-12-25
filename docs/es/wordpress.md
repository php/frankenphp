# WordPress

Ejecute [WordPress](https://wordpress.org/) con FrankenPHP para disfrutar de una pila moderna y de alto rendimiento con HTTPS automático, HTTP/3 y compresión Zstandard.

## Instalación Mínima

1. [Descargue WordPress](https://wordpress.org/download/)
2. Extraiga el archivo ZIP y abra una terminal en el directorio extraído
3. Ejecute:
   ```console
   frankenphp php-server
   ```
4. Vaya a `http://localhost/wp-admin/` y siga las instrucciones de instalación
5. ¡Listo!

Para una configuración lista para producción, prefiera usar `frankenphp run` con un `Caddyfile` como este:

```caddyfile
example.com

php_server
encode zstd br gzip
log
```

## Hot Reload

Para usar la función de [Hot reload](hot-reload.md) con WordPress, active [Mercure](mercure.md) y agregue la subdirectiva `hot_reload` a la directiva `php_server` en su `Caddyfile`:

```caddyfile
localhost

mercure {
    anonymous
}

php_server {
    hot_reload
}
```

Luego, agregue el código necesario para cargar las bibliotecas JavaScript en el archivo `functions.php` de su tema de WordPress:

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

Finalmente, ejecute `frankenphp run` desde el directorio raíz de WordPress.
