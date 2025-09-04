# 使用 GitHub Actions

此存储库构建 Docker 镜像并将其部署到 [Docker Hub](https://hub.docker.com/r/dunglas/frankenphp) 上
每个批准的拉取请求或设置后在你自己的分支上。

## 设置 GitHub Actions

在存储库设置中的 `secrets` 下，添加以下字段：

- `REGISTRY_LOGIN_SERVER`: 要使用的 Docker registry（如 `docker.io`）。
- `REGISTRY_USERNAME`: 用于登录 registry 的用户名（如 `dunglas`）。
- `REGISTRY_PASSWORD`: 用于登录 registry 的密码（如 `access key`）。
- `IMAGE_NAME`: 镜像的名称（如 `dunglas/frankenphp`）。

## 构建和推送镜像

1. 创建 Pull Request 或推送到你的 Fork 分支。
2. GitHub Actions 将生成镜像并运行每项测试。
3. 如果生成成功，则将使用 `pr-x` 推送 registry，其中 `x` 是 PR 编号，作为标记将镜像推送到注册表。

## 部署镜像

1. 合并 Pull Request 后，GitHub Actions 将再次运行测试并生成新镜像。
2. 如果构建成功，则 Docker 注册表中的 `main` tag 将更新。

## 发布

1. 在项目仓库中创建新 Tag。
2. GitHub Actions 将生成镜像并运行每项测试。
3. 如果构建成功，镜像将使用标记名称作为标记推送到 registry（例如，将创建 `v1.2.3` 和 `v1.2`）。
4. `latest` 标签也将更新。
