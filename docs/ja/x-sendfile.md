# 大きな静的ファイルを効率的に配信する （`X-Sendfile`/`X-Accel-Redirect`）

通常、静的ファイルはウェブサーバーによって直接配信されますが、
時にはファイルを送信する前にPHPコードを実行する必要があります。
例えば、アクセス制御、統計、カスタムHTTPヘッダーなど

残念ながら、PHPを使用して大きな静的ファイルを配信することは、
ウェブサーバーを直接使うより非効率的です（メモリ過負荷、パフォーマンス低下など）。

FrankenPHPでは、カスタマイズされたPHPコードを実行した**後**に、
静的ファイルの送信をウェブサーバーに委譲できます。

この機能を使うには、PHPアプリケーションは提供するファイルのパスを含む
カスタムHTTPヘッダーを定義するだけです。残りの処理はFrankenPHPが行います。

この機能は、Apacheでは **`X-Sendfile`** 、NGINXでは **`X-Accel-Redirect`** として知られています。

以下の例では、プロジェクトのドキュメントルートが`public/`ディレクトリであり、
`public/`ディレクトリの外部に保存されたファイルを
`private-files/`ディレクトリからPHPで提供したいと仮定します。

## 設定方法

まず、この機能を有効にするために以下の設定を`Caddyfile`に追加します：

```patch
	root public/
	# ...

+	# Symfony や Laravel など、Symfony HttpFoundation コンポーネントを使用するプロジェクトに必要
+	request_header X-Sendfile-Type x-accel-redirect
+	request_header X-Accel-Mapping ../private-files=/private-files
+
+	intercept {
+		@accel header X-Accel-Redirect *
+		handle_response @accel {
+			root private-files/
+			rewrite * {resp.header.X-Accel-Redirect}
+			method * GET
+
+			# セキュリティ強化のため、 PHP によって設定された X-Accel-Redirect ヘッダーを削除
+			header -X-Accel-Redirect
+
+			file_server
+		}
+	}

	php_server
```

## プレーンなPHPの場合

`private-files/`からの相対パスを`X-Accel-Redirect`ヘッダーの値として設定します：

```php
header('X-Accel-Redirect: file.txt');
```

## Symfony HttpFoundationコンポーネントを使用するプロジェクトの場合（Symfony、Laravel、Drupalなど）

SymfonyのHttpFoundationは[この機能をネイティブサポート](https://symfony.com/doc/current/components/http_foundation.html#serving-files)しており、
`X-Accel-Redirect`ヘッダーの正しい値を自動的に決定してレスポンスに追加します。

```php
use Symfony\Component\HttpFoundation\BinaryFileResponse;

BinaryFileResponse::trustXSendfileTypeHeader();
$response = new BinaryFileResponse(__DIR__.'/../private-files/file.txt');

// ...
```
