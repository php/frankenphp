# ホットリロード

FrankenPHPには、開発者の体験を大幅に向上させるために設計された組み込みの**ホットリロード**機能が含まれています。

![Mercure](hot-reload.png)

この機能は、最新のJavaScriptツール（Viteやwebpackなど）に見られる**ホットモジュールリプレイスメント（HMR）**に似たワークフローを提供します。
ファイル変更（PHPコード、テンプレート、JavaScript、CSSファイルなど）のたびに手動でブラウザを更新する代わりに、FrankenPHPはコンテンツをリアルタイムで更新します。

ホットリロードは、WordPress、Laravel、Symfony、およびその他のあらゆるPHPアプリケーションやフレームワークでネイティブに動作します。

有効にすると、FrankenPHPは現在の作業ディレクトリのファイルシステム変更を監視します。
ファイルが変更されると、[Mercure](mercure.md)の更新をブラウザにプッシュします。

設定によっては、ブラウザは次のいずれかを行います。

- [Idiomorph](https://github.com/bigskysoftware/idiomorph)がロードされている場合、**DOMをモーフィング**します（スクロール位置と入力状態を保持）。
- Idiomorphが存在しない場合、**ページをリロード**します（標準のライブリロード）。

## 設定

ホットリロードを有効にするには、Mercureを有効にし、`Caddyfile`の`php_server`ディレクティブに`hot_reload`サブディレクティブを追加します。

> [!WARNING]
> この機能は**開発環境でのみ**使用することを目的としています。
> ファイルシステムの監視はパフォーマンスのオーバーヘッドを引き起こし、内部エンドポイントを公開するため、本番環境で`hot_reload`を有効にしないでください。

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

デフォルトでは、FrankenPHPは現在の作業ディレクトリ内の以下のglobパターンに一致するすべてのファイルを監視します: `./**/*.{css,env,gif,htm,html,jpg,jpeg,js,mjs,php,png,svg,twig,webp,xml,yaml,yml}`

glob構文を使用して、監視するファイルを明示的に設定することもできます。

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

Mercureのトピックと監視するディレクトリやファイルを指定するには、`hot_reload`オプションにパスを指定する長い形式を使用します。

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

## クライアントサイドの統合

サーバーが変更を検出する一方で、ブラウザはページを更新するためにこれらのイベントを購読する必要があります。
FrankenPHPは、ファイル変更を購読するために使用するMercureハブのURLを、`$_SERVER['FRANKENPHP_HOT_RELOAD']`環境変数を通じて公開します。

クライアントサイドのロジックを処理するための便利なJavaScriptライブラリ、[frankenphp-hot-reload](https://www.npmjs.com/package/frankenphp-hot-reload)も利用可能です。
これを使用するには、メインレイアウトに以下を追加します。

```php
<!DOCTYPE html>
<title>FrankenPHP ホットリロード</title>
<?php if (isset($_SERVER['FRANKENPHP_HOT_RELOAD'])): ?>
<meta name="frankenphp-hot-reload:url" content="<?=$_SERVER['FRANKENPHP_HOT_RELOAD']?>">
<script src="https://cdn.jsdelivr.net/npm/idiomorph"></script>
<script src="https://cdn.jsdelivr.net/npm/frankenphp-hot-reload/+esm" type="module"></script>
<?php endif ?>
```

このライブラリは自動的にMercureハブを購読し、ファイル変更が検出されるとバックグラウンドで現在のURLを取得し、DOMをモーフィングします。
これは[npm](https://www.npmjs.com/package/frankenphp-hot-reload)パッケージとして、また[GitHub](https://github.com/dunglas/frankenphp-hot-reload)で利用可能です。

または、`EventSource`ネイティブJavaScriptクラスを使用してMercureハブを直接購読することで、独自のクライアントサイドロジックを実装することもできます。

### ワーカーモード

アプリケーションを[ワーカーモード](https://frankenphp.dev/docs/worker/)で実行している場合、アプリケーションスクリプトはメモリに残ります。
これは、ブラウザがリロードされても、PHPコードの変更がすぐに反映されないことを意味します。

最高の開発者体験を得るには、`hot_reload`を[ワーカーディレクティブ内の`watch`サブディレクティブ](config.md#watching-for-file-changes)と組み合わせる必要があります。

- `hot_reload`: ファイル変更時に**ブラウザ**を更新します
- `worker.watch`: ファイル変更時にワーカーを再起動します

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

### 仕組み

1. **監視**: FrankenPHPは、内部で[e-dant/watcherライブラリ](https://github.com/e-dant/watcher)を使用してファイルシステムの変更を監視します（我々はGoバインディングに貢献しました）。
2. **再起動（ワーカーモード）**: ワーカー設定で`watch`が有効になっている場合、PHPワーカーは新しいコードをロードするために再起動されます。
3. **プッシュ**: 変更されたファイルのリストを含むJSONペイロードが、組み込みの[Mercureハブ](https://mercure.rocks)に送信されます。
4. **受信**: JavaScriptライブラリを介してリッスンしているブラウザは、Mercureイベントを受信します。
5. **更新**:

- **Idiomorph**が検出された場合、更新されたコンテンツを取得し、現在のHTMLを新しい状態にモーフィングして、状態を失うことなく即座に変更を適用します。
- それ以外の場合、ページを更新するために`window.location.reload()`が呼び出されます。
