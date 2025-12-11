# PHP 应用程序作为独立二进制文件

FrankenPHP 能够将 PHP 应用程序的源代码和资源文件嵌入到静态的、独立的二进制文件中。

由于这个特性，PHP 应用程序可以作为独立的二进制文件分发，包括应用程序本身、PHP 解释器和生产级 Web 服务器 Caddy。

了解有关此功能的更多信息 [Kévin 在 SymfonyCon 上的演讲](https://dunglas.dev/2023/12/php-and-symfony-apps-as-standalone-binaries/)。

有关嵌入 Laravel 应用程序，请[阅读此特定文档条目](laravel.md#laravel-apps-as-standalone-binaries)。

## 准备你的应用

在创建独立二进制文件之前，请确保应用已准备好进行打包。

例如，你可能希望：

- 给应用安装生产环境的依赖
- 导出 autoloader
- 如果可能，为应用启用生产模式
- 丢弃不需要的文件，例如 `.git` 或测试文件，以减小最终二进制文件的大小

例如，对于 Symfony 应用程序，你可以使用以下命令：

```console
# 导出项目以避免 .git/ 等目录
mkdir $TMPDIR/my-prepared-app
git archive HEAD | tar -x -C $TMPDIR/my-prepared-app
cd $TMPDIR/my-prepared-app

# 设置适当的环境变量
echo APP_ENV=prod > .env.local
echo APP_DEBUG=0 >> .env.local

# 删除测试和其他不需要的文件以节省空间
# 或者，将这些文件添加到您的 .gitattributes 文件中，并设置 export-ignore 属性
rm -Rf tests/

# 安装依赖项
composer install --ignore-platform-reqs --no-dev -a

# 优化 .env
composer dump-env prod
```

### 自定义配置

要自定义[配置](config.md)，您可以放置一个 `Caddyfile` 以及一个 `php.ini` 文件
在应用程序的主目录中嵌入（在之前的示例中是`$TMPDIR/my-prepared-app`）。

## 创建 Linux 二进制文件

创建 Linux 二进制文件的最简单方法是使用我们提供的基于 Docker 的构建器。

1. 在准备好的应用的存储库中创建一个名为 `static-build.Dockerfile` 的文件。

   ```dockerfile
   FROM --platform=linux/amd64 dunglas/frankenphp:static-builder-gnu
   # 如果你打算在 glibc 系统上运行该二进制文件，请使用 static-builder-gnu

   # 复制应用代码
   WORKDIR /go/src/app/dist/app
   COPY . .

   # 构建静态二进制文件
   WORKDIR /go/src/app/
   RUN EMBED=/go/src/app/dist/app/ ./build-static.sh
   ```

   > [!CAUTION]
   >
   > 某些 `.dockerignore` 文件（例如默认的 [Symfony Docker `.dockerignore`](https://github.com/dunglas/symfony-docker/blob/main/.dockerignore)）
   > 会忽略 `vendor/` 文件夹和 `.env` 文件。在构建之前，请务必调整或删除 `.dockerignore` 文件。

2. 构建:

   ```console
   docker build -t static-app -f static-build.Dockerfile .
   ```

3. 提取二进制文件

   ```console
   docker cp $(docker create --name static-app-tmp static-app):/go/src/app/dist/frankenphp-linux-x86_64 my-app ; docker rm static-app-tmp
   ```

生成的二进制文件是当前目录中名为 `my-app` 的文件。

## 为其他操作系统创建二进制文件

如果你不想使用 Docker，或者想要构建 macOS 二进制文件，你可以使用我们提供的 shell 脚本：

```console
git clone https://github.com/php/frankenphp
cd frankenphp
EMBED=/path/to/your/app ./build-static.sh
```

在 `dist/` 目录中生成的二进制文件名称为 `frankenphp-<os>-<arch>`。

## 使用二进制文件

就是这样！`my-app` 文件（或其他操作系统上的 `dist/frankenphp-<os>-<arch>`）包含你的独立应用程序！

若要启动 Web 应用，请执行：

```console
./my-app php-server
```

如果你的应用包含 [worker 脚本](worker.md)，请使用如下命令启动 worker：

```console
./my-app php-server --worker public/index.php
```

要启用 HTTPS（自动创建 Let's Encrypt 证书）、HTTP/2 和 HTTP/3，请指定要使用的域名：

```console
./my-app php-server --domain localhost
```

你还可以运行二进制文件中嵌入的 PHP CLI 脚本：

```console
./my-app php-cli bin/console
```

## PHP Extensions

默认情况下，脚本将构建您项目的 `composer.json` 文件中所需的扩展（如果有的话）。
如果 `composer.json` 文件不存在，将构建默认扩展，如 [静态构建条目](static.md) 中所述。

要自定义扩展，请使用 `PHP_EXTENSIONS` 环境变量。

## 自定义构建

[阅读静态构建文档](static.md) 查看如何自定义二进制文件（扩展、PHP 版本等）。

## 分发二进制文件

在Linux上，创建的二进制文件使用[UPX](https://upx.github.io)进行压缩。

在Mac上，您可以在发送文件之前压缩它以减小文件大小。
我们推荐使用 `xz`。
