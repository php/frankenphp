# 本番環境でのデプロイ

このチュートリアルでは、Docker Composeを使用して単一サーバーにPHPアプリケーションをデプロイする方法を学びます。

Symfonyを使用している場合は、Symfony Dockerプロジェクトの「[本番環境へのデプロイ](https://github.com/dunglas/symfony-docker/blob/main/docs/production.md)」ドキュメントエントリー（FrankenPHPを使用しています）を参照してください。

API Platform（こちらもFrankenPHPを使用しています）を使用している場合は、[フレームワークのデプロイドキュメント](https://api-platform.com/docs/deployment/)を参照してください。

## アプリの準備

まず、PHPプロジェクトのルートディレクトリに`Dockerfile`を作成します：

```dockerfile
FROM dunglas/frankenphp

# "your-domain-name.example.com" を実際のドメイン名に置き換えてください
ENV SERVER_NAME=your-domain-name.example.com
# HTTPSを無効にしたい場合は、次の値を代わりに使用してください：
#ENV SERVER_NAME=:80

# プロジェクトで "public" ディレクトリをWebルートとして使用していない場合、ここで設定できます:
# ENV SERVER_ROOT=web/

# PHPの本番設定を有効化
RUN mv "$PHP_INI_DIR/php.ini-production" "$PHP_INI_DIR/php.ini"

# プロジェクトのPHPファイルをpublicディレクトリにコピー
COPY . /app/public
# Symfony や Laravel を使用している場合は、代わりにプロジェクト全体をコピーする必要があります：
#COPY . /app
```

より詳細な情報やカスタマイズ方法、PHP拡張モジュールやCaddyモジュールのインストール方法については、
「[カスタムDockerイメージのビルド](docker.md)」を参照してください。

プロジェクトでComposerを使用している場合は、
DockerイメージにComposerを含め、依存関係をインストールしてください。

次に、 `compose.yaml` ファイルを追加します：

```yaml
services:
  php:
    image: dunglas/frankenphp
    restart: always
    ports:
      - "80:80" # HTTP
      - "443:443" # HTTPS
      - "443:443/udp" # HTTP/3
    volumes:
      - caddy_data:/data
      - caddy_config:/config

# Caddyの証明書と設定に必要なボリューム
volumes:
  caddy_data:
  caddy_config:
```

> [!NOTE]
>
> 上記の例は本番環境向けです。
> 開発環境では、ボリューム、異なるPHP設定、`SERVER_NAME`環境変数の異なる値を使用したい場合があります。
>
> [Symfony Docker](https://github.com/dunglas/symfony-docker)プロジェクト（FrankenPHPを使用）では、
> マルチステージイメージ、Composer、追加のPHP拡張モジュールなどを活用した、
> より高度な例を見ることができます。

最後に、Gitを使用している場合は、これらのファイルをコミットしてプッシュします。

## サーバーの準備

本番環境にアプリケーションをデプロイするには、サーバーが必要です。
このチュートリアルではDigitalOceanの仮想マシンを使用しますが、他のLinuxサーバーでも同様に動作します。
DockerがインストールされたLinuxサーバーが既にある場合は、[次のセクション](#ドメイン名の設定)に進んでください。

まだサーバーがない場合は、[このアフィリエイトリンク](https://m.do.co/c/5d8aabe3ab80)を使用して$200の無料クレジットを取得し、アカウントを作成してください。その後、「Create a Droplet」をクリックします。
次に、「Choose an image」セクションの下の「Marketplace」タブをクリックし、「Docker」という名前のアプリを検索します。
これにより、DockerとDocker Composeの最新バージョンが既にインストールされたUbuntuサーバーがプロビジョニングされます！

テスト目的であれば、最安のプランで十分です。
実際の本番使用では、おそらくニーズに合わせて「general purpose」セクションのプランを選びたいでしょう。

![FrankenPHPをDockerでDigitalOceanにデプロイ](digitalocean-droplet.png)

他の設定はデフォルトのままにするか、必要に応じて調整も可能です。
SSHキーを追加するかパスワードを作成することを忘れずに行い、「Finalize and create」ボタンをクリックしてください。

次に、Dropletがプロビジョニングされるまで数秒待ちます。
Dropletの準備ができたら、SSHを使用して接続します：

```console
ssh root@<droplet-ip>
```

## ドメイン名の設定

ほとんどの場合、サイトにドメイン名を関連付けたいでしょう。
まだドメイン名を所有していない場合は、レジストラーを通じて購入する必要があります。

次に、サーバーのIPアドレスを指すドメイン名のタイプ`A`のDNSレコードを作成します：

```dns
your-domain-name.example.com.  IN  A     207.154.233.113
```

DigitalOceanのドメインサービス（「Networking」 > 「Domains」）での例：

![DigitalOceanでのDNS設定](digitalocean-dns.png)

> [!NOTE]
>
> FrankenPHPがデフォルトで使用しているTLS証明書の自動生成サービスLet's Encryptは、IPアドレスの直接使用をサポートしていません。Let's Encryptを使用するにはドメイン名の使用が必須です。

## デプロイ

`git clone`や`scp`など、目的に合ったツールを使用してプロジェクトをサーバーにコピーします。
GitHubを使用している場合は、[deploy key](https://docs.github.com/en/free-pro-team@latest/developers/overview/managing-deploy-keys#deploy-keys)の使用を検討してください。
deploy keyは[GitLabでもサポートされています](https://docs.gitlab.com/ee/user/project/deploy_keys/)。

Gitでの例：

```console
git clone git@github.com:<username>/<project-name>.git
```

プロジェクトディレクトリ（`<project-name>`）に移動し、本番モードでアプリを開始します：

```console
docker compose up --wait
```

サーバーが起動し、HTTPS証明書が自動的に生成されます。
`https://your-domain-name.example.com`にアクセスしてお楽しみください！

> [!CAUTION]
>
> Dockerはキャッシュレイヤーを持つ可能性があるため、各デプロイメントで正しいビルドを持っているか確認するか、キャッシュの問題を避けるために`--no-cache`オプションでプロジェクトを再ビルドしてください。

## 複数ノードへのデプロイ

複数のマシンクラスターにアプリをデプロイしたい場合は、提供されるComposeファイルと互換性のある[Docker Swarm](https://docs.docker.com/engine/swarm/stack-deploy/)を
使用できます。
Kubernetesでデプロイするには、FrankenPHPを使用する[API Platformで提供されるHelmチャート](https://api-platform.com/docs/deployment/kubernetes/)をご覧ください。
