# パフォーマンス

デフォルトでは、FrankenPHPはパフォーマンスと使いやすさの良い妥協点を提供するよう設計されています。
ただし、適切な設定により、パフォーマンスを大幅に向上させることが可能です。

## スレッド数とワーカー数

デフォルトでは、FrankenPHPは利用可能なCPU数の2倍のスレッドとワーカー（ワーカーモードで）を開始します。

適切な値は、アプリケーションの書き方、機能、ハードウェアに大きく依存します。
これらの値を調整することを強く推奨します。最適なシステムの安定性のためには、`num_threads` x `memory_limit` < `available_memory`とすることをお勧めします。

適切な値を見つけるには、実際のトラフィックをシミュレートした負荷テストを実行するのが最も効果的です。
[k6](https://k6.io)や[Gatling](https://gatling.io)がそのための有用なツールです。

スレッド数を設定するには、`php_server`や`php`ディレクティブの`num_threads`オプションを使用してください。
ワーカー数を変更するには、`frankenphp`ディレクティブの`worker`セクションにある`num`オプションを使用してください。

### `max_threads`

実際のトラフィックがどのようなものになるかを正確に把握できれば理想ですが、現実のアプリケーションでは
予測困難な挙動が多いものです。`max_threads`[設定](config.md#caddyfile-config) により、FrankenPHPは指定された制限まで実行時に追加スレッドを自動的に生成できます。
`max_threads`はトラフィックを処理するために必要なスレッド数を把握するのに役立ち、レイテンシのスパイクに対してサーバーをより回復力のあるものにできます。
`auto`に設定すると、制限は`php.ini`の`memory_limit`に基づいて推定されます。推定できない場合、
`auto`は代わりに`num_threads`の2倍がデフォルトになります。`auto`は必要なスレッド数を大幅に過小評価する可能性があることに留意してください。
`max_threads`はPHP FPMの[pm.max_children](https://www.php.net/manual/en/install.fpm.configuration.php#pm.max-children)に似ています。主な違いは、FrankenPHPがプロセスではなくスレッドを使用し、
必要に応じて異なるワーカースクリプトと「クラシックモード」間で自動的に委譲することです。

## ワーカーモード

[ワーカーモード](worker.md)を有効にするとパフォーマンスが劇的に向上しますが、
アプリがこのモードと互換性があるように適応する必要があります：
ワーカースクリプトを作成し、アプリがメモリリークしていないことを確認してください。

## muslを使用しない

公式Dockerイメージと私たちが提供するデフォルトバイナリのAlpine Linuxバリアントは、[musl libc](https://musl.libc.org)を使用しています。

PHPは、従来のGNUライブラリの代わりにこの代替Cライブラリを使用すると[遅くなる](https://gitlab.alpinelinux.org/alpine/aports/-/issues/14381)ことが知られており、
特にFrankenPHPに必要なZTSモード（スレッドセーフ）でコンパイルされた場合です。高度にスレッド化された環境では、差が大きくなる可能性があります。

また、[一部のバグはmuslを使用した場合にのみ発生します](https://github.com/php/php-src/issues?q=sort%3Aupdated-desc+is%3Aissue+is%3Aopen+label%3ABug+musl)。

本番環境では、glibcにリンクされ、適切な最適化レベルでコンパイルされたFrankenPHPを使用することをお勧めします。

これは、Debian Dockerイメージを使用するか、メンテナーの[.deb](https://debs.henderkes.com)または[.rpm](https://rpms.henderkes.com)パッケージを使用するか、または[FrankenPHPをソースからコンパイル](compile.md)することで実現できます。

## Go Runtime設定

FrankenPHPはGoで書かれています。

一般的に、Go runtimeは特別な設定を必要としませんが、特定の状況では、
特定の設定でパフォーマンスが向上する場合があります。

おそらく`GODEBUG`環境変数を`cgocheck=0`に設定したいでしょう（FrankenPHP Dockerイメージのデフォルト）。

FrankenPHPをコンテナ（Docker、Kubernetes、LXC...）で実行しており、コンテナで利用可能なメモリを制限している場合は、
`GOMEMLIMIT`環境変数に利用可能なメモリ量を設定してください。

詳細については、Go ランタイムを最大限に活用するために、[この主題に特化したGoドキュメントページ](https://pkg.go.dev/runtime#hdr-Environment_Variables)を読むことを強く推奨します。

## `file_server`

デフォルトでは、`php_server`ディレクティブは自動的にファイルサーバーを設定して
ルートディレクトリに保存された静的ファイル（アセット）を配信します。

この機能は便利ですが、コストがかかります。
無効にするには、以下の設定を使用してください：

```caddyfile
php_server {
    file_server off
}
```

## `try_files`

`php_server`は、静的ファイルとPHPファイルに加えて、アプリケーションのインデックスファイル
およびディレクトリインデックスファイル（`/path/` -> `/path/index.php`）も試行します。ディレクトリインデックスが不要な場合、
次のように`try_files`を明示的に定義して無効にできます：

```caddyfile
php_server {
    try_files {path} index.php
    root /root/to/your/app # ここで root を明示的に追加すると、キャッシュの効率が向上します
}
```

これにより、不要なファイルの操作の回数を大幅に削減できます。

ファイルシステムへの不要な操作を完全にゼロにする代替アプローチとして、`php`ディレクティブを使用し、
パスによってPHPファイルとそれ以外を分ける方法があります。アプリケーション全体が1つのエントリーファイルで提供される場合、この方法は有効です。
たとえば`/assets`フォルダの背後で静的ファイルを提供する[設定](config.md#caddyfile-config)は次のようになります：

```caddyfile
route {
    @assets {
        path /assets/*
    }

    # /assets 以下のリクエストはファイルサーバーが処理する
    file_server @assets {
        root /root/to/your/app
    }

    # /assets 以外のすべてのリクエストは index または worker の PHP ファイルで処理する
    rewrite index.php
    php {
        root /root/to/your/app # ここで root を明示的に追加すると、キャッシュの効率が向上します
    }
}
```

## プレースホルダー

`root`および`env`ディレクティブ内では、[プレースホルダー](https://caddyserver.com/docs/conventions#placeholders)を使用できます。
ただし、これによりこれらの値をキャッシュすることができなくなり、大幅なパフォーマンスコストが発生します。

可能であれば、これらのディレクティブではプレースホルダーの使用を避けてください。

## `resolve_root_symlink`

デフォルトでは、ドキュメントルートがシンボリックリンクである場合、FrankenPHP はそれを自動的に解決します（これは PHP が正しく動作するために必要です）。
ドキュメントルートがシンボリックリンクでない場合、この機能を無効にできます。

```caddyfile
php_server {
    resolve_root_symlink false
}
```

この設定は、`root`ディレクティブに[プレースホルダー](https://caddyserver.com/docs/conventions#placeholders)が含まれている場合にパフォーマンスを向上させます。
それ以外の場合の効果はごくわずかです。

## ログ

ログ出力は当然ながら非常に有用ですが、その性質上、
I/O操作およびメモリ確保が必要となり、パフォーマンスを大幅に低下させます。
[ログレベルを正しく設定](https://caddyserver.com/docs/caddyfile/options#log)し、
必要なもののみをログに記録するようにしてください。

## PHPパフォーマンス

FrankenPHPは公式のPHPインタープリターを使用しています。
通常のPHPに関するパフォーマンス最適化はすべてFrankenPHPでも有効です。

特に以下の点を確認してください：

- [OPcache](https://www.php.net/manual/en/book.opcache.php)がインストールされ、有効化され、適切に設定されていること
- [Composer autoloader optimizations](https://getcomposer.org/doc/articles/autoloader-optimization.md)を有効にすること
- `realpath`キャッシュがアプリケーションのニーズに合わせて十分な大きさであること
- [preloading](https://www.php.net/manual/en/opcache.preloading.php)を使用すること

詳細については、[Symfonyの専用ドキュメントエントリ](https://symfony.com/doc/current/performance.html)をお読みください
（Symfonyを使用していなくても、多くのヒントが役立ちます）。

## スレッドプールの分割

アプリケーションが、高負荷時に不安定になったり、常に10秒以上応答に時間がかかったりするAPIのような、低速な外部サービスとやり取りすることはよくあります。このような場合、専用の「低速」プールを持つようにスレッドプールを分割すると有益です。これにより、低速なエンドポイントがサーバーのリソース/スレッドをすべて消費してしまうのを防ぎ、コネクションプールのように低速なエンドポイントへのリクエストの同時実行数を制限できます。

```caddyfile
{
    frankenphp {
        max_threads 100 # すべてのワーカーで共有される最大100スレッド
    }
}

example.com {
    php_server {
        root /app/public # アプリケーションのルート
        worker index.php {
            match /slow-endpoint/* # パスが /slow-endpoint/* のすべてリクエストはこのスレッドプールによって処理される
            num 10 # /slow-endpoint/* に一致するリクエストの最小スレッド数10
        }
        worker index.php {
            match * # その他すべてのリクエストは個別に処理される
            num 20 # 低速なエンドポイントがハングし始めた場合でも、その他のリクエストに最小20スレッド
        }
    }
}
```

一般的に、メッセージキューなどの適切なメカニズムを使用して、非常に低速なエンドポイントを非同期に処理することも推奨されます。
