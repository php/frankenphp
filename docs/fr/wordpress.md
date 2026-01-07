# WordPress

Exécutez [WordPress](https://wordpress.org/) avec FrankenPHP pour profiter d'une pile moderne et performante avec HTTPS automatique, HTTP/3 et la compression Zstandard.

## Installation Minimale

1. [Téléchargez WordPress](https://wordpress.org/download/)
2. Extrayez l'archive ZIP et ouvrez un terminal dans le répertoire extrait
3. Exécutez :

   ```console
   frankenphp php-server
   ```

4. Allez sur `http://localhost/wp-admin/` et suivez les instructions d'installation
5. Profitez-en !

Pour une configuration prête pour la production, préférez utiliser `frankenphp run` avec un `Caddyfile` comme celui-ci :

```caddyfile
example.com

php_server
encode zstd br gzip
log
```

## Rechargement à chaud

Pour utiliser la fonctionnalité de [rechargement à chaud](hot-reload.md) avec WordPress, activez [Mercure](mercure.md) et ajoutez la sous-directive `hot_reload` à la directive `php_server` dans votre `Caddyfile` :

```caddyfile
localhost

mercure {
    anonymous
}

php_server {
    hot_reload
}
```

Ensuite, ajoutez le code nécessaire pour charger les bibliothèques JavaScript dans le fichier `functions.php` de votre thème WordPress :

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

Enfin, exécutez `frankenphp run` depuis le répertoire racine de WordPress.
