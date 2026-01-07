# WordPress

Execute [WordPress](https://wordpress.org/) com FrankenPHP para desfrutar de uma pilha moderna e de alta performance com HTTPS automático, HTTP/3 e compressão Zstandard.

## Instalação Mínima

1. [Baixe o WordPress](https://wordpress.org/download/)
2. Extraia o arquivo ZIP e abra um terminal no diretório extraído
3. Execute:

   ```console
   frankenphp php-server
   ```

4. Acesse `http://localhost/wp-admin/` e siga as instruções de instalação
5. Aproveite!

Para uma configuração pronta para produção, prefira usar `frankenphp run` com um `Caddyfile` como este:

```caddyfile
example.com

php_server
encode zstd br gzip
log
```

## Recarregamento Instantâneo

Para usar o recurso de [recarregamento instantâneo](hot-reload.md) com WordPress, habilite o [Mercure](mercure.md) e adicione a sub-diretiva `hot_reload` à diretiva `php_server` no seu `Caddyfile`:

```caddyfile
localhost

mercure {
    anonymous
}

php_server {
    hot_reload
}
```

Em seguida, adicione o código necessário para carregar as bibliotecas JavaScript no arquivo `functions.php` do seu tema WordPress:

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

Finalmente, execute `frankenphp run` a partir do diretório raiz do WordPress.
