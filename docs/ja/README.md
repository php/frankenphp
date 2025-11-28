# FrankenPHP: PHPのためのモダンなアプリケーションサーバー

<h1 align="center"><a href="https://frankenphp.dev"><img src="frankenphp.png" alt="FrankenPHP" width="600"></a></h1>

FrankenPHPは、[Caddy](https://caddyserver.com/) Webサーバーをベースに構築された、PHPのためのモダンなアプリケーションサーバーです。

FrankenPHPは、[_Early Hints_](https://frankenphp.dev/docs/early-hints/)、[ワーカーモード](https://frankenphp.dev/docs/worker/)、[リアルタイム機能](https://frankenphp.dev/docs/mercure/)、自動HTTPS、HTTP/2、HTTP/3などの驚異的な機能により、あなたのPHPアプリに強力な力を与えます。

FrankenPHPはあらゆるPHPアプリと連携し、ワーカーモードの公式統合によってLaravelやSymfonyプロジェクトをこれまで以上に高速化します。

また、FrankenPHPはスタンドアロンのGoライブラリとしても利用可能で、`net/http`を使って任意のアプリにPHPを埋め込むことができます。

[**詳しくは** _frankenphp.dev_](https://frankenphp.dev)と、このスライド資料もご参照ください：

<a href="https://dunglas.dev/2022/10/frankenphp-the-modern-php-app-server-written-in-go/"><img src="https://dunglas.dev/wp-content/uploads/2022/10/frankenphp.png" alt="Slides" width="600"></a>

## はじめに

Windowsの場合、FrankenPHPを動かすために[WSL](https://learn.microsoft.com/windows/wsl/)を使ってください。

### インストール
プラットフォームにあった適切なバージョンを自動インストールするために以下の行をターミナルにコピーしてください。

```console
curl https://frankenphp.dev/install.sh | sh
```

### スタンドアロンバイナリ

LinuxとmacOS向けに、[PHP 8.4](https://www.php.net/releases/8.4/en.php)と人気のPHP拡張モジュールを含む静的な
FrankenPHPバイナリを提供しています。

[FrankenPHPをダウンロード](https://github.com/php/frankenphp/releases)

**拡張機能のインストール**: 一般的な拡張機能はバンドルされています。追加の拡張インストールはできません。

### rpm パッケージ
メンテナーは```dnf```を使った全てのシステム向けにrpmパッケージ提供しています。インストールと起動方法:

```console
sudo dnf install https://rpm.henderkes.com/static-php-1-0.noarch.rpm
sudo dnf module enable php-zts:static-8.4 # 8.2-8.5 available
sudo dnf install frankenphp
```

**拡張機能のインストール:** `sudo dnf install php-zts-<extension>`

デフォルトで利用できない拡張機能は[PIE](https://github.com/php/pie)を使ってください:

```console
sudo dnf install php-zts-devel
sudo pie install asgrim/example-pie-extension --with-php-config=php-config-zts
```

### deb パッケージ
メンテナーは```apt```を使った全てのシステム向けにdebパッケージ提供しています。インストールと起動方法:

```console
sudo curl -fsSL https://key.henderkes.com/static-php.gpg -o /usr/share/keyrings/static-php.gpg && \
echo "deb [signed-by=/usr/share/keyrings/static-php.gpg] https://deb.henderkes.com/ stable main" | sudo tee /etc/apt/sources.list.d/static-php.list && \
sudo apt update
sudo apt install frankenphp
```

**拡張機能のインストール:** `sudo apt install php-zts-<extension>`

デフォルトで利用できない拡張機能は[PIE](https://github.com/php/pie)を使ってください:

```console
sudo apt install php-zts-devel
sudo pie install asgrim/example-pie-extension --with-php-config=php-config-zts
```

### Homebrew

FrankenPHPはmacOSとLinuxで[Homebrew](https://brew.sh)として利用も可能です。

```console
brew install dunglas/frankenphp/frankenphp
```

**拡張機能のインストール:** [PIE](https://github.com/php/pie)を使用する。

### 使い方

カレントディレクトリの内容を起動する:

```console
frankenphp php-server
```

コマンドラインスクリプトで起動することも可能

```console
frankenphp php-cli /path/to/your/script.php
```

deb, rpmパッケージでsystemdで実行することも可能:

```console
sudo systemctl start frankenphp
```

### Docker

また、[Dockerイメージ](https://frankenphp.dev/docs/docker/)も利用可能です：

```console
docker run -v .:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

ブラウザで`https://localhost`にアクセスして、FrankenPHPをお楽しみください！

> [!TIP]
>
> `https://127.0.0.1`ではなく、`https://localhost`を使用して、自己署名証明書を受け入れてください。
> 使用するドメインを変更したい場合は、[`SERVER_NAME` 環境変数](docs/config.md#environment-variables)を設定してください。

### Homebrew

FrankenPHPはmacOSおよびLinux向けに[Homebrew](https://brew.sh)パッケージとしても利用可能です。

インストール方法：

```console
brew install dunglas/frankenphp/frankenphp
```

現在のディレクトリのコンテンツを配信するには、以下を実行してください：

```console
frankenphp php-server
```

## ドキュメント

- [クラシックモード](https://frankenphp.dev/docs/classic/)
- [ワーカーモード](https://frankenphp.dev/docs/worker/)
- [Early Hintsサポート（103 HTTPステータスコード）](https://frankenphp.dev/docs/early-hints/)
- [リアルタイム](https://frankenphp.dev/docs/mercure/)
- [大きな静的ファイルの効率的な提供](https://frankenphp.dev/docs/x-sendfile/)
- [設定](https://frankenphp.dev/docs/config/)
- [Dockerイメージ](https://frankenphp.dev/docs/docker/)
- [本番環境でのデプロイ](https://frankenphp.dev/docs/production/)
- [パフォーマンス最適化](https://frankenphp.dev/docs/performance/)
- [**スタンドアロン**、自己実行可能なPHPアプリの作成](https://frankenphp.dev/docs/embed/)
- [静的バイナリの作成](https://frankenphp.dev/docs/static/)
- [ソースからのコンパイル](https://frankenphp.dev/docs/compile/)
- [FrankenPHPの監視](https://frankenphp.dev/docs/metrics/)
- [Laravel統合](https://frankenphp.dev/docs/laravel/)
- [既知の問題](https://frankenphp.dev/docs/known-issues/)
- [デモアプリ（Symfony）とベンチマーク](https://github.com/dunglas/frankenphp-demo)
- [Goライブラリドキュメント](https://pkg.go.dev/github.com/dunglas/frankenphp)
- [コントリビューションとデバッグ](https://frankenphp.dev/docs/contributing/)

## 例とスケルトン

- [Symfony](https://github.com/dunglas/symfony-docker)
- [API Platform](https://api-platform.com/docs/symfony)
- [Laravel](https://frankenphp.dev/docs/laravel/)
- [Sulu](https://sulu.io/blog/running-sulu-with-frankenphp)
- [WordPress](https://github.com/StephenMiracle/frankenwp)
- [Drupal](https://github.com/dunglas/frankenphp-drupal)
- [Joomla](https://github.com/alexandreelise/frankenphp-joomla)
- [TYPO3](https://github.com/ochorocho/franken-typo3)
- [Magento2](https://github.com/ekino/frankenphp-magento2)
