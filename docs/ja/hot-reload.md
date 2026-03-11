# ホットリロード

FrankenPHPには、開発者のエクスペリエンスを大幅に向上させるために設計された組み込みの**ホットリロード**機能が含まれています。

![Hot Reload](hot-reload.png)

この機能は、ViteやwebpackなどのモダンなJavaScriptツールにおける**ホットモジュールリプレースメント (HMR)** に似たワークフローを提供します。
ファイルの変更（PHPコード、テンプレート、JavaScript、CSSファイルなど）のたびに手動でブラウザをリフレッシュする代わりに、
FrankenPHPはページの内容をリアルタイムで更新します。

ホットリロードは、WordPress、Laravel、Symfony、その他すべてのPHPアプリケーションやフレームワークでネイティブに動作します。

有効にすると、FrankenPHPは現在の作業ディレクトリにおけるファイルシステムの変更を監視します。
ファイルが変更されると、[Mercure](mercure.md)の更新をブラウザにプッシュします。

設定に応じて、ブラウザは以下のいずれかを実行します。

- [Idiomorph](https://github.com/bigskysoftware/idiomorph)がロードされている場合、**DOMをモーフィング**します（スクロール位置と入力状態を保持）。
- Idiomorphが存在しない場合、**ページをリロード**します（標準のライブリロード）。

## 設定

ホットリロードを有効にするには、Mercureを有効にしてから、`Caddyfile`の`php_server`ディレクティブに`hot_reload`サブディレクティブを追加します。

> [!WARNING]
>
> この機能は**開発環境のみ**を対象としています。
> `hot_reload`を本番環境で有効にしないでください。この機能は安全ではなく（機密性の高い内部詳細を公開します）、アプリケーションの速度を低下させます。
>
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

デフォルトでは、FrankenPHPは現在の作業ディレクトリ内の以下のグロブパターンに一致するすべてのファイルを監視します: `./**/*.{css,env,gif,htm,html,jpg,jpeg,js,mjs,php,png,svg,twig,webp,xml,yaml,yml}`

グロブ構文を使用して、監視するファイルを明示的に設定することも可能です。

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

使用するMercureトピック、および監視するディレクトリまたはファイルを指定するには、`hot_reload`のロングフォームを使用します。

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

サーバーは変更を検出しますが、ブラウザはこれらのイベントを購読してページを更新する必要があります。
FrankenPHPは、ファイル変更を購読するために使用するMercure HubのURLを、`$_SERVER['FRANKENPHP_HOT_RELOAD']`環境変数を通じて公開します。

クライアントサイドのロジックを処理するための便利なJavaScriptライブラリ、[frankenphp-hot-reload](https://www.npmjs.com/package/frankenphp-hot-reload)も利用可能です。
これを使用するには、メインレイアウトに以下を追加します。

```php
<!DOCTYPE html>
<title>FrankenPHP Hot Reload</title>
<?php if (isset($_SERVER['FRANKENPHP_HOT_RELOAD'])): ?>
<meta name="frankenphp-hot-reload:url" content="<?=$_SERVER['FRANKENPHP_HOT_RELOAD']?>">
<script src="https://cdn.jsdelivr.net/npm/idiomorph"></script>
<script src="https://cdn.jsdelivr.net/npm/frankenphp-hot-reload/+esm" type="module"></script>
<?php endif ?>
```

このライブラリは自動的にMercureハブを購読し、ファイル変更が検出されるとバックグラウンドで現在のURLをフェッチし、DOMをモーフィングします。
[npm](https://www.npmjs.com/package/frankenphp-hot-reload)パッケージとして、また[GitHub](https://github.com/dunglas/frankenphp-hot-reload)で利用できます。

または、`EventSource`ネイティブJavaScriptクラスを使用してMercureハブに直接購読することで、独自のクライアントサイドロジックを実装することもできます。

### 既存のDOMノードを保持する

まれに、[Symfonyのウェブデバッグツールバーなどの開発ツール](https://github.com/symfony/symfony/pull/62970)を使用している場合など、特定のDOMノードを保持したいことがあります。
そのためには、関連するHTML要素に`data-frankenphp-hot-reload-preserve`属性を追加します。

```html
<div data-frankenphp-hot-reload-preserve><!-- My debug bar --></div>
```

## ワーカーモード

アプリケーションを[ワーカーモード](https://frankenphp.dev/docs/worker/)で実行している場合、アプリケーションスクリプトはメモリに常駐します。
これは、ブラウザがリロードされても、PHPコードの変更がすぐに反映されないことを意味します。

最高の開発者エクスペリエンスのためには、`hot_reload`を[ワーカーディレクティブ内の`watch`サブディレクティブ](config.md#watching-for-file-changes)と組み合わせるべきです。

- `hot_reload`: ファイルが変更されたときに**ブラウザ**をリフレッシュします
- `worker.watch`: ファイルが変更されたときにワーカーを再起動します

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

## 仕組み

1.  **監視**: FrankenPHPは、内部で[`e-dant/watcher`ライブラリ](https://github.com/e-dant/watcher)を使用してファイルシステムの変更を監視します（私たちはGoバインディングに貢献しました）。
2.  **再起動 (ワーカーモード)**: ワーカー設定で`watch`が有効になっている場合、新しいコードをロードするためにPHPワーカーが再起動されます。
3.  **プッシュ**: 変更されたファイルのリストを含むJSONペイロードが、組み込みの[Mercureハブ](https://mercure.rocks)に送信されます。
4.  **受信**: JavaScriptライブラリを介してリッスンしているブラウザがMercureイベントを受信します。
5.  **更新**:

    -   **Idiomorph**が検出された場合、更新されたコンテンツをフェッチし、現在のHTMLを新しい状態に合わせてモーフィングし、状態を失うことなく即座に変更を適用します。
    -   それ以外の場合、`window.location.reload()`が呼び出されてページがリフレッシュされます。
