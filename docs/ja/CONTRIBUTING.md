# コントリビューション

## PHPのコンパイル

### Dockerを使用する場合（Linux）

開発用Dockerイメージをビルドします：

```console
docker build -t frankenphp-dev -f dev.Dockerfile .
docker run --cap-add=SYS_PTRACE --security-opt seccomp=unconfined -p 8080:8080 -p 443:443 -p 443:443/udp -v $PWD:/go/src/app -it frankenphp-dev
```

このイメージには通常の開発ツール（Go、GDB、Valgrind、Neovimなど）が含まれており、PHP設定ファイルは以下の場所に配置されます。

- php.ini: `/etc/frankenphp/php.ini` 開発用のプリセットが適用されたphp.iniファイルがデフォルトで提供されます。
- 追加の設定ファイル: `/etc/frankenphp/php.d/*.ini`
- PHP拡張モジュール: `/usr/lib/frankenphp/modules/`

お使いのDockerのバージョンが23.0未満の場合、dockerignore[パターンの問題](https://github.com/moby/moby/pull/42676)によりビルドに失敗する可能性があります。以下のように`.dockerignore`にディレクトリを追加してください。

```patch
 !testdata/*.php
 !testdata/*.txt
+!caddy
+!internal
```

### Dockerを使用しない場合（LinuxおよびmacOS）

[ソースからのコンパイル手順](https://frankenphp.dev/docs/compile/)に従い、`--debug`設定フラグを渡してください。

## テストスイートの実行

```console
go test -tags watcher -race -v ./...
```

## Caddyモジュール

FrankenPHPのCaddyモジュール付きでCaddyをビルドします：

```console
cd caddy/frankenphp/
go build -tags watcher,brotli,nobadger,nomysql,nopgx
cd ../../
```

FrankenPHPのCaddyモジュール付きでCaddyを実行します：

```console
cd testdata/
../caddy/frankenphp/frankenphp run
```

サーバーは`127.0.0.1:80`で待ち受けています：

> [!NOTE]
> Dockerを使用している場合は、コンテナのポート80をバインドするか、コンテナ内で実行する必要があります。

```console
curl -vk http://127.0.0.1/phpinfo.php
```

## 最小構成のテストサーバー

最小構成のテストサーバーをビルドします：

```console
cd internal/testserver/
go build
cd ../../
```

テストサーバーを実行します：

```console
cd testdata/
../internal/testserver/testserver
```

サーバーは`127.0.0.1:8080`で待ち受けています：

```console
curl -v http://127.0.0.1:8080/phpinfo.php
```

## Dockerイメージをローカルでビルドする

bakeプランを出力します：

```console
docker buildx bake -f docker-bake.hcl --print
```

amd64用のFrankenPHPイメージをローカルでビルドします：

```console
docker buildx bake -f docker-bake.hcl --pull --load --set "*.platform=linux/amd64"
```

arm64用のFrankenPHPイメージをローカルでビルドします：

```console
docker buildx bake -f docker-bake.hcl --pull --load --set "*.platform=linux/arm64"
```

arm64とamd64用のFrankenPHPイメージをスクラッチからビルドしてDocker Hubにプッシュします：

```console
docker buildx bake -f docker-bake.hcl --pull --no-cache --push
```

## 静的ビルドでのセグメンテーション違反のデバッグ

1. GitHubからFrankenPHPバイナリのデバッグ版をダウンロードするか、デバッグシンボルを含む独自の静的ビルドを作成します：

   ```console
   docker buildx bake \
       --load \
       --set static-builder.args.DEBUG_SYMBOLS=1 \
       --set "static-builder.platform=linux/amd64" \
       static-builder
   docker cp $(docker create --name static-builder-musl dunglas/frankenphp:static-builder-musl):/go/src/app/dist/frankenphp-linux-$(uname -m) frankenphp
   ```

2. 現在使用している`frankenphp`を、デバッグ版のFrankenPHP実行ファイルに置き換えます
3. 通常通りFrankenPHPを起動します（あるいは、GDBで直接FrankenPHPを開始することもできます：`gdb --args frankenphp run`）
4. GDBでプロセスにアタッチします：

   ```console
   gdb -p `pidof frankenphp`
   ```

5. 必要に応じて、GDBシェルで`continue`と入力します
6. FrankenPHPをクラッシュさせます
7. GDBシェルで`bt`と入力します
8. 出力結果をコピーします

## GitHub Actionsでのセグメンテーション違反のデバッグ

1. `.github/workflows/tests.yml`を開きます
2. PHPデバッグシンボルを有効にします

   ```patch
       - uses: shivammathur/setup-php@v2
         # ...
         env:
           phpts: ts
   +       debug: true
   ```

3. `tmate`を有効にしてコンテナに接続できるようにします

   ```patch
       - name: Set CGO flags
         run: echo "CGO_CFLAGS=$(php-config --includes)" >> "$GITHUB_ENV"
   +   - run: |
   +       sudo apt install gdb
   +       mkdir -p /home/runner/.config/gdb/
   +       printf "set auto-load safe-path /\nhandle SIG34 nostop noprint pass" > /home/runner/.config/gdb/gdbinit
   +   - uses: mxschmitt/action-tmate@v3
   ```

4. コンテナに接続します
5. `frankenphp.go`を開きます
6. `cgosymbolizer`を有効にします

   ```patch
   -	//_ "github.com/ianlancetaylor/cgosymbolizer"
   +	_ "github.com/ianlancetaylor/cgosymbolizer"
   ```

7. モジュールを取得します：`go get`
8. コンテナ内で、GDBなどを使用できます：

   ```console
   go test -tags watcher -c -ldflags=-w
   gdb --args frankenphp.test -test.run ^MyTest$
   ```

9. バグが修正されたら、これらの変更をすべて元に戻します

## その他の開発リソース

- [uWSGIでのPHP埋め込み](https://github.com/unbit/uwsgi/blob/master/plugins/php/php_plugin.c)
- [NGINX UnitでのPHP埋め込み](https://github.com/nginx/unit/blob/master/src/nxt_php_sapi.c)
- [Go言語でのPHP埋め込み (go-php)](https://github.com/deuill/go-php)
- [Go言語でのPHP埋め込み (GoEmPHP)](https://github.com/mikespook/goemphp)
- [C++でのPHP埋め込み](https://gist.github.com/paresy/3cbd4c6a469511ac7479aa0e7c42fea7)
- [Sara Golemon 著『Extending and Embedding PHP』](https://books.google.fr/books?id=zMbGvK17_tYC&pg=PA254&lpg=PA254#v=onepage&q&f=false)
- [TSRMLS_CCとは何か？](http://blog.golemon.com/2006/06/what-heck-is-tsrmlscc-anyway.html)
- [SDL バインディング](https://pkg.go.dev/github.com/veandco/go-sdl2@v0.4.21/sdl#Main)

## Docker関連リソース

- [Bakeファイル定義](https://docs.docker.com/build/customize/bake/file-definition/)
- [`docker buildx build`](https://docs.docker.com/engine/reference/commandline/buildx_build/)

## 便利なコマンド

```console
apk add strace util-linux gdb
strace -e 'trace=!futex,epoll_ctl,epoll_pwait,tgkill,rt_sigreturn' -p 1
```

## ドキュメントの翻訳

新しい言語でドキュメントとサイトを翻訳するには、
以下の手順で行ってください。

1. このリポジトリの`docs/`ディレクトリに、言語の2文字ISOコードを名前にした新しいディレクトリを作成します
2. `docs/`ディレクトリのルートにある全ての`.md`ファイルを新しいディレクトリにコピーします（翻訳のソースとして常に英語版を使用してください。英語版が最新版だからです）
3. ルートディレクトリから`README.md`と`CONTRIBUTING.md`ファイルを新しいディレクトリにコピーします
4. ファイルの内容を翻訳しますが、ファイル名は変更せず、`> [!`で始まる文字列も翻訳しないでください（これはGitHub用の特別なマークアップです）
5. 翻訳でプルリクエストを作成します
6. [サイトリポジトリ](https://github.com/dunglas/frankenphp-website/tree/main)で、`content/`、`data/`、`i18n/`ディレクトリの翻訳ファイルをコピーして翻訳します
7. 作成されたYAMLファイルの値を翻訳します
8. サイトリポジトリでプルリクエストを開きます
