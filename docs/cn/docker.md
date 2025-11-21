# 构建自定义 Docker 镜像

[FrankenPHP Docker 镜像](https://hub.docker.com/r/dunglas/frankenphp) 基于 [官方 PHP 镜像](https://hub.docker.com/_/php/)。提供适用于流行架构的 Debian 和 Alpine Linux 变体。推荐使用 Debian 变体。

提供 PHP 8.2、8.3、8.5 和 8.5 的变体。

标签遵循此模式：`dunglas/frankenphp:<frankenphp-version>-php<php-version>-<os>`

- `<frankenphp-version>` 和 `<php-version>` 分别是 FrankenPHP 和 PHP 的版本号，范围从主版本（例如 `1`）、次版本（例如 `1.2`）到补丁版本（例如 `1.2.3`）。
- `<os>` 要么是 `bookworm`（用于 Debian Bookworm）要么是 `alpine`（用于 Alpine 的最新稳定版本）。

[浏览标签](https://hub.docker.com/r/dunglas/frankenphp/tags)。

## 如何使用镜像

在项目中创建 `Dockerfile`：

```dockerfile
FROM dunglas/frankenphp

COPY . /app/public
```

然后运行以下命令以构建并运行 Docker 镜像：

```console
docker build -t my-php-app .
docker run -it --rm --name my-running-app my-php-app
```

## 如何安装更多 PHP 扩展

[`docker-php-extension-installer`](https://github.com/mlocati/docker-php-extension-installer) 脚本在基础镜像中提供。
添加额外的 PHP 扩展很简单：

```dockerfile
FROM dunglas/frankenphp

# 在此处添加其他扩展：
RUN install-php-extensions \
	pdo_mysql \
	gd \
	intl \
	zip \
	opcache
```

## 如何安装更多 Caddy 模块

FrankenPHP 建立在 Caddy 之上，所有 [Caddy 模块](https://caddyserver.com/docs/modules/) 都可以与 FrankenPHP 一起使用。

安装自定义 Caddy 模块的最简单方法是使用 [xcaddy](https://github.com/caddyserver/xcaddy)：

```dockerfile
FROM dunglas/frankenphp:builder AS builder

# 在构建器镜像中复制 xcaddy
COPY --from=caddy:builder /usr/bin/xcaddy /usr/bin/xcaddy

# 必须启用 CGO 才能构建 FrankenPHP
RUN CGO_ENABLED=1 \
    XCADDY_SETCAP=1 \
    XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
    CGO_CFLAGS=$(php-config --includes) \
    CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
    xcaddy build \
        --output /usr/local/bin/frankenphp \
        --with github.com/dunglas/frankenphp=./ \
        --with github.com/dunglas/frankenphp/caddy=./caddy/ \
        --with github.com/dunglas/caddy-cbrotli \
        # Mercure 和 Vulcain 包含在官方版本中，如果不需要你可以删除它们
        --with github.com/dunglas/mercure/caddy \
        --with github.com/dunglas/vulcain/caddy
        # 在此处添加额外的 Caddy 模块

FROM dunglas/frankenphp AS runner

# 将官方二进制文件替换为包含自定义模块的二进制文件
COPY --from=builder /usr/local/bin/frankenphp /usr/local/bin/frankenphp
```

FrankenPHP 提供的 `builder` 镜像包含 `libphp` 的编译版本。
[用于构建的镜像](https://hub.docker.com/r/dunglas/frankenphp/tags?name=builder) 适用于所有版本的 FrankenPHP 和 PHP，包括 Alpine 和 Debian。

> [!TIP]
>
> 如果你的系统基于 musl libc（Alpine Linux 上默认使用）并搭配 Symfony 使用，
> 你可能需要 [增加默认堆栈大小](compile.md#using-xcaddy)。

## 默认启用 worker 模式

设置 `FRANKENPHP_CONFIG` 环境变量以使用 worker 脚本启动 FrankenPHP：

```dockerfile
FROM dunglas/frankenphp

# ...

ENV FRANKENPHP_CONFIG="worker ./public/index.php"
```

## 开发挂载宿主机目录

要使用 FrankenPHP 轻松开发，请从包含应用程序源代码的主机挂载目录作为 Docker 容器中的 volume：

```console
docker run -v $PWD:/app/public -p 80:80 -p 443:443 -p 443:443/udp --tty my-php-app
```

> [!TIP]
>
> `--tty` 选项允许使用清晰可读的日志，而不是 JSON 日志。

使用 Docker Compose：

```yaml
# compose.yaml

services:
  php:
    image: dunglas/frankenphp
    # 如果要使用自定义 Dockerfile，请取消注释以下行
    #build: .
    # 如果要在生产环境中运行，请取消注释以下行
    # restart: always
    ports:
      - "80:80" # HTTP
      - "443:443" # HTTPS
      - "443:443/udp" # HTTP/3
    volumes:
      - ./:/app/public
      - caddy_data:/data
      - caddy_config:/config
    # 在生产环境中注释以下行，它允许在 dev 中使用清晰可读日志
    tty: true

# Caddy 证书和配置所需的挂载目录
volumes:
  caddy_data:
  caddy_config:
```

## 以非 root 用户身份运行

FrankenPHP 可以在 Docker 中以非 root 用户身份运行。

下面是一个示例 `Dockerfile`：

```dockerfile
FROM dunglas/frankenphp

ARG USER=appuser

RUN \
	# 在基于 alpine 的发行版使用 "adduser -D ${USER}"
	useradd ${USER}; \
	# 需要开放80和443端口的权限
	setcap CAP_NET_BIND_SERVICE=+eip /usr/local/bin/frankenphp; \
	# 需要 /config/caddy 和 /data/caddy 目录的写入权限
	chown -R ${USER}:${USER} /config/caddy /data/caddy

USER ${USER}
```

### 无权限运行

即使在无根运行时，FrankenPHP 也需要 `CAP_NET_BIND_SERVICE` 权限来将
Web 服务器绑定到特权端口（80 和 443）。

如果你在非特权端口（1024 及以上）上公开 FrankenPHP，则可以以非 root 用户身份运行
Web 服务器，并且不需要任何权限：

```dockerfile
FROM dunglas/frankenphp

ARG USER=appuser

RUN \
	# 在基于 alpine 的发行版使用 "adduser -D ${USER}"
	useradd ${USER}; \
	# 移除默认权限
	setcap -r /usr/local/bin/frankenphp; \
	# 给予 /config/caddy 和 /data/caddy 写入权限
	chown -R ${USER}:${USER} /config/caddy /data/caddy

USER ${USER}
```

接下来，设置 `SERVER_NAME` 环境变量以使用非特权端口。
示例：`:8000`

## 更新

Docker 镜像会按照以下条件更新：

- 发布新的版本后
- 每日 4:00（UTC 时间）检查新的 PHP 镜像

## 开发版本

可在此 [`dunglas/frankenphp-dev`](https://hub.docker.com/repository/docker/dunglas/frankenphp-dev) 仓库获取开发版本。
每次在 GitHub 仓库的主分支有新的 commit 都会触发一次新的 build。

`latest*` tag 指向最新的 `main` 分支，且同样支持 `sha-<git-commit-hash>` 的 tag。
