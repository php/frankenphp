# 在生产环境中部署

在本教程中，我们将学习如何使用 Docker Compose 在单个服务器上部署 PHP 应用程序。

如果你使用的是 Symfony，请阅读 Symfony Docker 项目（使用 FrankenPHP）的 [在生产环境中部署](https://github.com/dunglas/symfony-docker/blob/main/docs/production.md) 文档条目。

如果你使用的是 API Platform（同样使用 FrankenPHP），请参阅 [框架的部署文档](https://api-platform.com/docs/deployment/)。

## 准备应用

首先，在 PHP 项目的根目录中创建一个 `Dockerfile`：

```dockerfile
FROM dunglas/frankenphp

# 请将 "your-domain-name.example.com" 替换为你的域名
ENV SERVER_NAME=your-domain-name.example.com
# 如果要禁用 HTTPS，请改用以下值：
#ENV SERVER_NAME=:80

# 如果你的项目不使用 "public" 目录作为 web 根目录，你可以在这里设置：
# ENV SERVER_ROOT=web/

# 启用 PHP 生产配置
RUN mv "$PHP_INI_DIR/php.ini-production" "$PHP_INI_DIR/php.ini"

# 将项目的 PHP 文件复制到 public 目录中
COPY . /app/public
# 如果你使用 Symfony 或 Laravel，你需要复制整个项目：
#COPY . /app
```

有关更多详细信息和选项，请参阅 [构建自定义 Docker 镜像](docker.md)。
要了解如何自定义配置，请安装 PHP 扩展和 Caddy 模块。

如果你的项目使用 Composer，
请务必将其包含在 Docker 镜像中并安装你的依赖。

然后，添加一个 `compose.yaml` 文件：

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

# Caddy 证书和配置所需的挂载目录
volumes:
  caddy_data:
  caddy_config:
```

> [!NOTE]
>
> 前面的示例适用于生产用途。
> 在开发中，你可能希望使用挂载目录，不同的 PHP 配置和不同的 `SERVER_NAME` 环境变量值。
>
> 见 [Symfony Docker](https://github.com/dunglas/symfony-docker) 项目
> （使用 FrankenPHP）作为使用多阶段镜像的更高级示例，
> Composer、额外的 PHP 扩展等。

最后，如果你使用 Git，请提交这些文件并推送。

## 准备服务器

若要在生产环境中部署应用程序，需要一台服务器。
在本教程中，我们将使用 DigitalOcean 提供的虚拟机，但任何 Linux 服务器都可以工作。
如果你已经有安装了 Docker 的 Linux 服务器，你可以直接跳到 [下一节](#配置域名)。

否则，请使用 [此会员链接](https://m.do.co/c/5d8aabe3ab80) 获得 200 美元的免费信用额度，创建一个帐户，然后单击“Create a Droplet”。
然后，单击“Choose an image”部分下的“Marketplace”选项卡，然后搜索名为“Docker”的应用程序。
这将配置已安装最新版本的 Docker 和 Docker Compose 的 Ubuntu 服务器！

出于测试目的，最便宜的就足够了。
对于实际的生产用途，你可能需要在“general purpose”部分中选择一个计划来满足你的需求。

![使用 Docker 在 DigitalOcean 上部署 FrankenPHP](../digitalocean-droplet.png)

你可以保留其他设置的默认值，也可以根据需要进行调整。
不要忘记添加你的 SSH 密钥或创建密码，然后点击“完成并创建”按钮。

然后，在 Droplet 预配时等待几秒钟。
Droplet 准备就绪后，使用 SSH 进行连接：

```console
ssh root@<droplet-ip>
```

## 配置域名

在大多数情况下，你需要将域名与你的网站相关联。
如果你还没有域名，则必须通过注册商购买。

然后为你的域名创建类型为 `A` 的 DNS 记录，指向服务器的 IP 地址：

```dns
your-domain-name.example.com.  IN  A     207.154.233.113
```

DigitalOcean 域服务示例（“Networking” > “Domains”）：

![在 DigitalOcean 上配置 DNS](../digitalocean-dns.png)

> [!NOTE]
>
> Let's Encrypt 是 FrankenPHP 默认用于自动生成 TLS 证书的服务，不支持使用裸 IP 地址。使用域名是使用 Let's Encrypt 的必要条件。

## 部署

使用 `git clone`、`scp` 或任何其他可能适合你需要的工具在服务器上复制你的项目。
如果使用 GitHub，则可能需要使用 [部署密钥](https://docs.github.com/en/free-pro-team@latest/developers/overview/managing-deploy-keys#deploy-keys)。
部署密钥也 [由 GitLab 支持](https://docs.gitlab.com/ee/user/project/deploy_keys/)。

Git 示例：

```console
git clone git@github.com:<username>/<project-name>.git
```

进入包含项目 (`<project-name>`) 的目录，并在生产模式下启动应用：

```console
docker compose up --wait
```

你的服务器已启动并运行，并且已自动为你生成 HTTPS 证书。
去 `https://your-domain-name.example.com` 享受吧！

> [!CAUTION]
>
> Docker 有一个缓存层，请确保每个部署都有正确的构建，或者使用 `--no-cache` 选项重新构建项目以避免缓存问题。

## 在多个节点上部署

如果要在计算机集群上部署应用程序，可以使用 [Docker Swarm](https://docs.docker.com/engine/swarm/stack-deploy/)，
它与提供的 Compose 文件兼容。
要在 Kubernetes 上部署，请查看 [API 平台提供的 Helm 图表](https://api-platform.com/docs/deployment/kubernetes/)，同样也使用 FrankenPHP。
