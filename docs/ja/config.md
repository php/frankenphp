# 設定

FrankenPHP、Caddy、そしてMercureやVulcainモジュールは、[Caddyでサポートされる形式](https://caddyserver.com/docs/getting-started#your-first-config)を使用して設定できます。

[Dockerイメージ](docker.md)では、`Caddyfile`は`/etc/frankenphp/Caddyfile`に配置されています。
静的バイナリは、`frankenphp run`コマンドを実行したディレクトリ内の`Caddyfile`を参照します。
また、`-c`または`--config`オプションでカスタムのパスを指定できます。

PHP自体の設定は[`php.ini` ファイルを使用](https://www.php.net/manual/en/configuration.file.php)して行えます。

インストール方法に応じて、PHPインタープリターは上記いずれかの場所にある設定ファイルを参照します。

## Docker

- `php.ini`: `/usr/local/etc/php/php.ini`（デフォルトでは`php.ini`は含まれていません）
- 追加の設定ファイル: `/usr/local/etc/php/conf.d/*.ini`
- PHP拡張モジュール: `/usr/local/lib/php/extensions/no-debug-zts-<YYYYMMDD>/`
- PHPプロジェクトが提供する公式テンプレートをコピーすることを推奨します：

```dockerfile
FROM dunglas/frankenphp

# 本番環境：
RUN cp $PHP_INI_DIR/php.ini-production $PHP_INI_DIR/php.ini

# または開発環境：
RUN cp $PHP_INI_DIR/php.ini-development $PHP_INI_DIR/php.ini
```

## RPMおよびDebianパッケージ

- `php.ini`: `/etc/frankenphp/php.ini`（本番環境向けのプリセットの`php.ini`ファイルがデフォルトで提供されます）
- 追加の設定ファイル: `/etc/frankenphp/php.d/*.ini`
- PHP拡張モジュール: `/usr/lib/frankenphp/modules/`

## 静的バイナリ

- `php.ini`: `frankenphp run`または`frankenphp php-server`を実行したディレクトリ内、なければ`/etc/frankenphp/php.ini`を参照
- 追加の設定ファイル: `/etc/frankenphp/php.d/*.ini`
- PHP拡張モジュール: ロードできません、バイナリ自体にバンドルする必要があります
- [PHPソース](https://github.com/php/php-src/)で提供される`php.ini-production`または`php.ini-development`のいずれかをコピーしてください

## Caddyfileの設定

`php_server`または`php`の[HTTPディレクティブ](https://caddyserver.com/docs/caddyfile/concepts#directives)は、サイトブロック内で使用してPHPアプリを配信できます。

最小構成の例：

```caddyfile
localhost {
	# 圧縮を有効化（オプション）
	encode zstd br gzip
	# 現在のディレクトリ内のPHPファイルを実行し、アセットを配信
	php_server
}
```

グローバルオプションを使用してFrankenPHPを明示的に設定することもできます：
`frankenphp`の[グローバルオプション](https://caddyserver.com/docs/caddyfile/concepts#global-options)を使用してFrankenPHPを構成できます。

```caddyfile
{
	frankenphp {
		num_threads <num_threads> # 開始するPHPスレッド数を設定します。デフォルト: 利用可能なCPU数の2倍。
		max_threads <num_threads> # 実行時に追加で開始できるPHPスレッドの最大数を制限します。デフォルト: num_threads。 'auto'を設定可能。
		max_wait_time <duration> # リクエストがタイムアウトする前にPHPのスレッドが空くのを待つ最大時間を設定します。デフォルト: 無効。
		php_ini <key> <value> # php.iniのディレクティブを設定します。複数のディレクティブを設定するために何度でも使用できます。
		worker {
			file <path> # ワーカースクリプトのパスを設定します。
			num <num> # 開始するPHPスレッド数を設定します。デフォルト: 利用可能なCPU数の2倍。
			env <key> <value> # 追加の環境変数を指定された値に設定する。複数の環境変数に対して複数回指定することができます。
			watch <path> # ファイル変更を監視するパスを設定します。複数のパスに対して複数回指定できます。
			name <name> # ワーカーの名前を設定します。ログとメトリクスで使用されます。デフォルト: ワーカーファイルの絶対パス
			max_consecutive_failures <num> # workerが不健全とみなされるまでの、連続失敗の最大回数を設定します。 -1 はワーカーを常に再起動することを意味します。デフォルトは 6 です。
		}
	}
}

# ...
```

代わりに、`worker`オプションのワンライナー形式を使用することもできます：

```caddyfile
{
	frankenphp {
		worker <file> <num>
	}
}

# ...
```

同じサーバーで複数のアプリを提供する場合は、複数のワーカーを定義することもできます：

```caddyfile
app.example.com {
    root /path/to/app/public
	php_server {
		root /path/to/app/public # キャッシュ効率を高める
		worker index.php <num>
	}
}

other.example.com {
    root /path/to/other/public
	php_server {
		root /path/to/other/public
		worker index.php <num>
	}
}

# ...
```

通常は`php_server`ディレクティブを使えば十分ですが、
より細かい制御が必要な場合は、より低レベルの`php`ディレクティブを使用できます。
`php`ディレクティブは、対象がPHPファイルかどうかを確認せず、すべての入力をPHPに渡します。
詳しくは[パフォーマンスページ](performance.md#try_files)をお読みください。

`php_server`ディレクティブの使用は、以下の設定と同等です：

```caddyfile
route {
	# ディレクトリへのリクエストに末尾スラッシュを追加
	@canonicalPath {
		file {path}/index.php
		not path */
	}
	redir @canonicalPath {path}/ 308
	# 要求されたファイルが存在しない場合は、indexファイルを試行
	@indexFiles file {
		try_files {path} {path}/index.php index.php
		split_path .php
	}
	rewrite @indexFiles {http.matchers.file.relative}
	# FrankenPHP!
	@phpFiles path *.php
	php @phpFiles
	file_server
}
```

`php_server`と`php`ディレクティブには以下のオプションがあります：

```caddyfile
php_server [<matcher>] {
	root <directory> # サイトのルートフォルダを設定します。デフォルト: `root`ディレクティブ。
	split_path <delim...> # URIを2つの部分に分割するための部分文字列を設定します。最初にマッチする部分文字列がURIから「パス情報」を分割するために使用されます。最初の部分はマッチする部分文字列で接尾辞が付けられ、実際のリソース（CGIスクリプト）名とみなされます。2番目の部分はスクリプトが使用する PATH_INFO に設定されます。デフォルト: `.php`
	resolve_root_symlink false # シンボリックリンクが存在する場合`root`ディレクトリをシンボリックリンクの評価によって実際の値に解決することを無効にする（デフォルトで有効）。
	env <key> <value> # 追加の環境変数を指定された値に設定する。複数の環境変数を指定する場合は、複数回指定することができます。
	file_server off # 組み込みのfile_serverディレクティブを無効にします。
	worker { # このサーバー固有のワーカーを作成します。複数のワーカーに対して複数回指定できます。
		file <path> # ワーカースクリプトへのパスを設定します。php_serverのルートからの相対パスとなります。
		num <num> # 起動するPHPスレッド数を設定します。デフォルトは利用可能なスレッド数の 2 倍です。
		name <name> # ログとメトリクスで使用されるワーカーの名前を設定します。デフォルト: ワーカーファイルの絶対パス。php_server ブロックで定義されている場合は、常にm#で始まります。
		watch <path> # ファイルの変更を監視するパスを設定する。複数のパスに対して複数回指定することができる。
		env <key> <value> # 追加の環境変数を指定された値に設定する。複数の環境変数を指定する場合は、複数回指定することができます。このワーカーの環境変数もphp_serverの親から継承されますが、 ここで上書きすることもできます。
		match <path> # ワーカーをパスパターンにマッチさせます。try_filesを上書きし、php_serverディレクティブでのみ使用できます。
	}
	worker <other_file> <num> # グローバルfrankenphpブロックのような短縮形式も使用できます。
}
```

### ファイルの変更監視

ワーカーはアプリケーションを一度だけ起動してメモリに保持するため、
PHPファイルに変更を加えても即座には反映されません。

代わりに、`watch`ディレクティブを使用してファイル変更時にワーカーを再起動させることができます。
これは開発環境において有用です。

```caddyfile
{
	frankenphp {
		worker {
			file  /path/to/app/public/worker.php
			watch
		}
	}
}
```

`watch`ディレクトリが指定されていない場合、`./**/*.{php,yaml,yml,twig,env}`にフォールバックします。
これは、FrankenPHPプロセスが開始されたディレクトリおよびそのサブディレクトリ内のすべての`.php`、`.yaml`、`.yml`、`.twig`、`.env`ファイルすべてを監視します。
代わりに、[シェルのファイル名パターン](https://pkg.go.dev/path/filepath#Match)を使用して
1つ以上のディレクトリを指定することもできます：

```caddyfile
{
	frankenphp {
		worker {
			file  /path/to/app/public/worker.php
			watch /path/to/app # /path/to/app以下すべてのサブディレクトリのファイルを監視
			watch /path/to/app/*.php # /path/to/app内の.phpで終わるファイルを監視
			watch /path/to/app/**/*.php # /path/to/appおよびサブディレクトリのPHPファイルを監視
			watch /path/to/app/**/*.{php,twig} # /path/to/appおよびサブディレクトリ内のPHPとTwigファイルを監視
		}
	}
}
```

- `**` パターンは再帰的な監視を意味します
- ディレクトリは相対パス（FrankenPHPプロセスの開始ディレクトリから）にもできます
- 複数のワーカーが定義されている場合、いずれかのファイルが変更されるとすべてのワーカーが再起動されます
- 実行時に生成されるファイル（ログなど）を監視対象に含めると、意図しないワーカーの再起動を引き起こす可能性があるため注意が必要です

ファイルウォッチャーは[e-dant/watcher](https://github.com/e-dant/watcher)に基づいています。

## パスにワーカーをマッチさせる

従来のPHPアプリケーションでは、スクリプトは常にpublicディレクトリに配置されます。
これはワーカースクリプトにも当てはまり、他のPHPスクリプトと同様に扱われます。
ワーカースクリプトをpublicディレクトリの外に配置したい場合は、`match`ディレクティブを使用して実現できます。

`match`ディレクティブは、`try_files`の最適化された代替手段であり、`php_server`および`php`の中でのみ使用できます。
次の例では、public ディレクトリ内にファイルが存在すればそれを配信し、
存在しなければ、パスパターンに一致するワーカーにリクエストを転送します。

```caddyfile
{
	frankenphp {
		php_server {
			worker {
				file /path/to/worker.php # ファイルはpublicパス外でも可
				match /api/* # /api/で始まるすべてのリクエストはこのワーカーで処理される
			}
		}
	}
}
```

### フルデュプレックス（HTTP/1）

HTTP/1.xを使用する場合、全体のボディが読み取られる前にレスポンスを書き込めるようにするため、
フルデュプレックスモードを有効にすることが望ましい場合があります（例：WebSocket、Server-Sent Eventsなど）。

これは明示的に有効化する必要がある設定で、`Caddyfile`のグローバルオプションに追加する必要があります：

```caddyfile
{
  servers {
    enable_full_duplex
  }
}
```

> [!CAUTION]
>
> このオプションを有効にすると、フルデュプレックスをサポートしない古いHTTP/1.xクライアントでデッドロックが発生する可能性があります。
> これは`CADDY_GLOBAL_OPTIONS`環境設定を使用しても設定できます：

```sh
CADDY_GLOBAL_OPTIONS="servers {
  enable_full_duplex
}"
```

この設定の詳細については、[Caddyドキュメント](https://caddyserver.com/docs/caddyfile/options#enable-full-duplex)をご覧ください。

## 環境変数

以下の環境変数を使用することで、`Caddyfile`を直接変更せずにCaddyディレクティブを注入できます：

- `SERVER_NAME`: [待ち受けアドレス](https://caddyserver.com/docs/caddyfile/concepts#addresses)を変更し、指定したホスト名はTLS証明書の生成にも使用されます
- `SERVER_ROOT`: サイトのルートディレクトリを変更します。デフォルトは`public/`
- `CADDY_GLOBAL_OPTIONS`: [グローバルオプション](https://caddyserver.com/docs/caddyfile/options)を注入します
- `FRANKENPHP_CONFIG`: `frankenphp`ディレクティブの下に設定を注入します

FPM や CLI SAPI と同様に、環境変数はデフォルトで`$_SERVER`スーパーグローバルで公開されます。

[`variables_order` PHPディレクティブ](https://www.php.net/manual/en/ini.core.php#ini.variables-order)の`S`値は、このディレクティブ内での`E`の位置にかかわらず常に`ES`と同等です。

## PHP設定

[追加のPHP設定ファイル](https://www.php.net/manual/en/configuration.file.php#configuration.file.scan)を読み込むには、
`PHP_INI_SCAN_DIR`環境変数を使用できます。
設定されると、PHPは指定されたディレクトリに存在する`.ini`拡張子を持つすべてのファイルを読み込みます。

また、`Caddyfile`の`php_ini`ディレクティブを使用してPHP設定を変更することもできます：

```caddyfile
{
    frankenphp {
        php_ini memory_limit 256M

        # または

        php_ini {
            memory_limit 256M
            max_execution_time 15
        }
    }
}
```

## デバッグモードの有効化

Dockerイメージを使用する場合、`CADDY_GLOBAL_OPTIONS`環境変数に`debug`を設定するとデバッグモードが有効になります：

```console
docker run -v $PWD:/app/public \
    -e CADDY_GLOBAL_OPTIONS=debug \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```
