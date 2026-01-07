# WordPress

自動HTTPS、HTTP/3、Zstandard圧縮を備えたモダンで高性能なスタックを楽しむために、FrankenPHPで[WordPress](https://wordpress.org/)を実行してください。

## 最小限のインストール

1. [WordPressをダウンロード](https://wordpress.org/download/)
2. ZIPアーカイブを解凍し、解凍したディレクトリでターミナルを開きます
3. 実行:

   ```console
   frankenphp php-server
   ```

4. `http://localhost/wp-admin/`にアクセスし、インストール手順に従います
5. お楽しみください！

本番環境向けのセットアップには、次のような`Caddyfile`を使用して`frankenphp run`を実行することをお勧めします:

```caddyfile
example.com

php_server
encode zstd br gzip
log
```

## ホットリロード

WordPressで[ホットリロード](hot-reload.md)機能を使用するには、[Mercure](mercure.md)を有効にし、`hot_reload`サブディレクティブを`Caddyfile`の`php_server`ディレクティブに追加します:

```caddyfile
localhost

mercure {
    anonymous
}

php_server {
    hot_reload
}
```

次に、WordPressテーマの`functions.php`ファイルにJavaScriptライブラリをロードするために必要なコードを追加します:

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

最後に、WordPressのルートディレクトリから`frankenphp run`を実行します。
