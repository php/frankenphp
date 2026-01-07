# 使用 GitHub Actions

此存储库会在每次获得批准的拉取请求或在您自己的 fork 配置完成后，构建 Docker 镜像并将其部署到 [Docker Hub](https://hub.docker.com/r/dunglas/frankenphp)。

## 设置 GitHub Actions

在存储库设置的 `Secrets` (密钥) 下，添加以下密钥：

- `REGISTRY_LOGIN_SERVER`: 要使用的 Docker registry（例如 `docker.io`）。
- `REGISTRY_USERNAME`: 用于登录 registry 的用户名（例如 `dunglas`）。
- `REGISTRY_PASSWORD`: 用于登录 registry 的密码（例如，访问密钥）。
- `IMAGE_NAME`: 镜像的名称（例如 `dunglas/frankenphp`）。

## 构建和推送镜像

1. 创建 Pull Request 或推送到您的 Fork 分支。
2. GitHub Actions 将构建镜像并运行所有测试。
3. 如果构建成功，镜像将使用 `pr-x`（其中 `x` 是 PR 编号）作为标签推送到注册表。

## 部署镜像

1. Pull Request 合并后，GitHub Actions 将再次运行测试并构建新的镜像。
2. 如果构建成功，Docker 注册表中的 `main` 标签将被更新。

## 发布

1. 在仓库中创建新标签。
2. GitHub Actions 将构建镜像并运行所有测试。
3. 如果构建成功，镜像将使用标签名称作为标签推送到注册表（例如，将创建 `v1.2.3` 和 `v1.2`）。
4. `latest` 标签也将被更新。
