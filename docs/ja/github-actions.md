# GitHub Actionsの使用

このリポジトリでは、承認されたプルリクエストごと、またはセットアップ後のあなた自身のフォークで、
Dockerイメージをビルドして[Docker Hub](https://hub.docker.com/r/dunglas/frankenphp)にデプロイします。

## GitHub Actionsのセットアップ

リポジトリ設定のシークレットで、以下のシークレットを追加してください：

- `REGISTRY_LOGIN_SERVER`: 使用するDockerレジストリ（例：`docker.io`）
- `REGISTRY_USERNAME`: レジストリログイン用のユーザー名（例：`dunglas`）
- `REGISTRY_PASSWORD`: レジストリログイン用のパスワード（例：アクセスキー）
- `IMAGE_NAME`: イメージの名前（例：`dunglas/frankenphp`）

## イメージのビルドとプッシュ

1. プルリクエストを作成するか、フォークにプッシュします
2. GitHub Actionsがイメージをビルドし、テストを実行します
3. ビルドが成功した場合、イメージは`pr-x`（`x`はPR番号）をタグとしてレジストリにプッシュされます

## イメージのデプロイ

1. プルリクエストがマージされると、GitHub Actionsが再度テストを実行し、新しいイメージをビルドします
2. ビルドが成功した場合、Dockerレジストリの`main`タグが更新されます

## リリース

1. リポジトリで新しいタグを作成します
2. GitHub Actionsがイメージをビルドし、テストを実行します
3. ビルドが成功した場合、イメージはタグ名をタグとしてレジストリにプッシュされます（例：`v1.2.3`と`v1.2`が作成されます）
4. `latest`タグも更新されます
