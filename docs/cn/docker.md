# 构建自定义 Docker 镜像

[FrankenPHP Docker 镜像](https://hub.docker.com/r/dunglas/frankenphp) 基于 [官方 PHP 镜像](https://hub.docker.com/_/php/)。
提供适用于流行架构的 Debian 和 Alpine Linux 变体。
推荐使用 Debian 变体。

提供 PHP 8.2、8.3、8.4 和 8.5 的变体。

标签遵循此模式：`dunglas/frankenphp:<frankenphp-version>-php<php-version>-<os>`

- `<frankenphp-version>` 和 `<php-version>` 分别是 FrankenPHP 和 PHP 的版本号，范围从主版本（例如 `1`）、次版本（例如 `1.2`）到补丁版本（例如 `1.2.3`）。
- `<os>` 要么是 `trixie`（用于 Debian Trixie），`bookworm`（用于 Debian Bookworm），要么是 `alpine`（用于 Alpine 的最新稳定版本）。

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

## 如何调整配置

为了方便，镜像中提供了一个包含有用环境变量的[默认 `Caddyfile`](https://github.com/php/frankenphp/blob/main/caddy/frankenphp/Caddyfile)。

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

FrankenPHP 提供的[构建器镜像](https://hub.docker.com/r/dunglas/frankenphp/tags?name=builder)适用于所有版本的 FrankenPHP 和 PHP，同时支持 Debian 和 Alpine。

> [!TIP]
>
> 如果你正在使用 Alpine Linux 和 Symfony，你可能需要[增加默认堆栈大小](compile.md#using-xcaddy)。

## 默认启用 worker 模式

设置 `FRANKENPHP_CONFIG` 环境变量以使用 worker 脚本启动 FrankenPHP：

```dockerfile
FROM dunglas/frankenphp

# ...

ENV FRANKENPHP_CONFIG="worker ./public/index.php"
```

## 在开发中使用卷

要使用 FrankenPHP 轻松开发，请从包含应用程序源代码的主机挂载目录作为 Docker 容器中的卷：

```console
docker run -v $PWD:/app/public -p 80:80 -p 443:443 -p 443:443/udp --tty my-php-app
```

> [!TIP]
>
> `--tty` 选项允许使用易读的日志，而不是 JSON 日志。

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
    # 在生产环境中注释以下行，它允许在开发环境中使用易读日志
    tty: true

# Caddy 证书和配置所需的数据卷
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
	# 在基于 Alpine 的发行版使用 "adduser -D ${USER}"
	useradd ${USER}; \
	# 添加绑定到 80 和 443 端口的额外能力
	setcap CAP_NET_BIND_SERVICE=+eip /usr/local/bin/frankenphp; \
	# 赋予 /config/caddy 和 /data/caddy 目录的写入权限
	chown -R ${USER}:${USER} /config/caddy /data/caddy

USER ${USER}
```

### 在不使用能力的情况下运行

即使在无根运行时，FrankenPHP 也需要 `CAP_NET_BIND_SERVICE` 能力来将
Web 服务器绑定到特权端口（80 和 443）。

如果你在非特权端口（1024 及以上）上公开 FrankenPHP，则可以以非 root 用户身份运行
Web 服务器，并且不需要任何能力：

```dockerfile
FROM dunglas/frankenphp

ARG USER=appuser

RUN \
	# 在基于 Alpine 的发行版使用 "adduser -D ${USER}"
	useradd ${USER}; \
	# 移除默认能力
	setcap -r /usr/local/bin/frankenphp; \
	# 赋予 /config/caddy 和 /data/caddy 目录的写入权限
	chown -R ${USER}:${USER} /config/caddy /data/caddy

USER ${USER}
```

接下来，设置 `SERVER_NAME` 环境变量以使用非特权端口。
示例：`:8000`

## 更新

Docker 镜像会在以下情况下构建：

- 发布新的版本后
- 每日 UTC 时间上午 4 点，如果新的官方 PHP 镜像可用

## 强化镜像

为了进一步减少 FrankenPHP Docker 镜像的攻击面和大小，还可以基于 [Google distroless](https://github.com/GoogleContainerTools/distroless) 或 [Docker hardened](https://www.docker.com/products/hardened-images) 镜像构建它们。

> [!WARNING]
> 这些最小化的基础镜像不包含 shell 或包管理器，这使得调试更加困难。因此，仅在安全性优先级很高的情况下，才推荐将其用于生产环境。

当添加额外的 PHP 扩展时，你需要一个中间构建阶段：

```dockerfile
FROM dunglas/frankenphp AS builder

# 在此处添加额外的 PHP 扩展
RUN install-php-extensions pdo_mysql pdo_pgsql #...

# 将 frankenphp 和所有已安装扩展的共享库复制到临时位置
# 你也可以通过分析 frankenphp 二进制文件和每个扩展 .so 文件的 ldd 输出手动执行此步骤
RUN apt-get update && apt-get install -y libtree && \
    EXT_DIR="$(php -r 'echo ini_get("extension_dir");')" && \
    FRANKENPHP_BIN="$(which frankenphp)"; \
    LIBS_TMP_DIR="/tmp/libs"; \
    mkdir -p "$LIBS_TMP_DIR"; \
    for target in "$FRANKENPHP_BIN" $(find "$EXT_DIR" -maxdepth 2 -type f -name "*.so"); do \
        libtree -pv "$target" | sed 's/.*── \(.*\) \[.*/\1/' | grep -v "^$target" | while IFS= read -r lib; do \
            [ -z "$lib" ] && continue; \
            base=$(basename "$lib"); \
            destfile="$LIBS_TMP_DIR/$base"; \
            if [ ! -f "$destfile" ]; then \
                cp "$lib" "$destfile"; \
            fi; \
        done; \
    done


# Distroless debian 基础镜像，确保它与基础镜像使用相同的 debian 版本
FROM gcr.io/distroless/base-debian13
# Docker hardened 镜像替代方案
# FROM dhi.io/debian:13

# 你的应用程序和 Caddyfile 要复制到容器中的位置
ARG PATH_TO_APP="."
ARG PATH_TO_CADDYFILE="./Caddyfile"

# 将你的应用程序复制到 /app
# 为了进一步强化，请确保只有可写路径由非 root 用户拥有
COPY --chown=nonroot:nonroot "$PATH_TO_APP" /app
COPY "$PATH_TO_CADDYFILE" /etc/caddy/Caddyfile

# 复制 frankenphp 和必要的库
COPY --from=builder /usr/local/bin/frankenphp /usr/local/bin/frankenphp
COPY --from=builder /usr/local/lib/php/extensions /usr/local/lib/php/extensions
COPY --from=builder /tmp/libs /usr/lib

# 复制 php.ini 配置文件
COPY --from=builder /usr/local/etc/php/conf.d /usr/local/etc/php/conf.d
COPY --from=builder /usr/local/etc/php/php.ini-production /usr/local/etc/php/php.ini

# Caddy 数据目录——即使在只读根文件系统上，也必须对非 root 用户可写
ENV XDG_CONFIG_HOME=/config \
    XDG_DATA_HOME=/data
COPY --from=builder --chown=nonroot:nonroot /data/caddy /data/caddy
COPY --from=builder --chown=nonroot:nonroot /config/caddy /config/caddy

USER nonroot

WORKDIR /app

# 运行 frankenphp 并使用提供的 Caddyfile 的入口点
ENTRYPOINT ["/usr/local/bin/frankenphp", "run", "-c", "/etc/caddy/Caddyfile"]
```

## 开发版本

开发版本可在 [`dunglas/frankenphp-dev`](https://hub.docker.com/repository/docker/dunglas/frankenphp-dev) Docker 仓库中获取。
每次将新的提交推送到 GitHub 仓库的主分支时，都会触发一次新的构建。

`latest*` 标签指向 `main` 分支的 HEAD。形式为 `sha-<git-commit-hash>` 的标签也可用。
