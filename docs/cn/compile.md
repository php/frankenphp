# 从源代码编译

本文档解释了如何创建一个 FrankenPHP 构建，它将 PHP 加载为一个动态库。
这是推荐的方法。

或者，你也可以 [编译静态版本](static.md)。

## 安装 PHP

FrankenPHP 支持 PHP 8.2 及更高版本。

### 使用 Homebrew (Linux 和 Mac)

安装与 FrankenPHP 兼容的 libphp 版本的最简单方法是使用 [Homebrew PHP](https://github.com/shivammathur/homebrew-php) 提供的 ZTS 包。

首先，如果尚未安装，请安装 [Homebrew](https://brew.sh)。

然后，安装 PHP 的 ZTS 变体、Brotli（可选，用于压缩支持）和 watcher（可选，用于文件更改检测）：

```console
brew install shivammathur/php/php-zts brotli watcher
brew link --overwrite --force shivammathur/php/php-zts
```

### 通过编译 PHP

或者，你可以按照以下步骤，使用 FrankenPHP 所需的选项从源代码编译 PHP。

首先，[获取 PHP 源代码](https://www.php.net/downloads.php) 并提取它们：

```console
tar xf php-*
cd php-*/
```

然后，运行适用于你平台的 `configure` 脚本。
以下 `./configure` 标志是必需的，但你可以添加其他标志，例如编译扩展或附加功能。

#### Linux

```console
./configure \
    --enable-embed \
    --enable-zts \
    --disable-zend-signals \
    --enable-zend-max-execution-timers
```

#### Mac

使用 [Homebrew](https://brew.sh/) 包管理器安装所需的和可选的依赖项：

```console
brew install libiconv bison brotli re2c pkg-config watcher
echo 'export PATH="/opt/homebrew/opt/bison/bin:$PATH"' >> ~/.zshrc
```

然后运行 `./configure` 脚本：

```console
./configure \
    --enable-embed \
    --enable-zts \
    --disable-zend-signals \
    --with-iconv=/opt/homebrew/opt/libiconv/
```

#### 编译 PHP

最后，编译并安装 PHP：

```console
make -j"$(getconf _NPROCESSORS_ONLN)"
sudo make install
```

## 安装可选依赖项

某些 FrankenPHP 功能依赖于必须安装的可选系统依赖项。
或者，可以通过向 Go 编译器传递构建标签来禁用这些功能。

| 功能                     | 依赖项                                                                   | 用于禁用的构建标签 |
|--------------------------|------------------------------------------------------------------------|-------------------|
| Brotli 压缩              | [Brotli](https://github.com/google/brotli)                            | nobrotli          |
| 文件更改时重启 worker     | [Watcher C](https://github.com/e-dant/watcher/tree/release/watcher-c) | nowatcher         |

## 编译 Go 应用

你现在可以构建最终的二进制文件。

### 使用 xcaddy

推荐的方法是使用 [xcaddy](https://github.com/caddyserver/xcaddy) 来编译 FrankenPHP。
`xcaddy` 还允许轻松添加 [自定义 Caddy 模块](https://caddyserver.com/docs/modules/) 和 FrankenPHP 扩展：

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/dunglas/frankenphp/caddy \
    --with github.com/dunglas/mercure/caddy \
    --with github.com/dunglas/vulcain/caddy
    # 在这里添加额外的 Caddy 模块和 FrankenPHP 扩展
```

> [!TIP]
>
> 如果你的系统基于 musl libc（Alpine Linux 上默认使用）并搭配 Symfony 使用，
> 你可能需要增加默认堆栈大小。
> 否则，你可能会收到如下错误 `PHP Fatal error: Maximum call stack size of 83360 bytes reached during compilation. Try splitting expression`
>
> 请将 `XCADDY_GO_BUILD_FLAGS` 环境变量更改为如下类似的值
> `XCADDY_GO_BUILD_FLAGS=$'-ldflags "-w -s -extldflags \'-Wl,-z,stack-size=0x80000\'"'`
> （根据你的应用需求更改堆栈大小）。

### 不使用 xcaddy

或者，可以通过直接使用 `go` 命令来编译 FrankenPHP 而不使用 `xcaddy`：

```console
curl -L https://github.com/php/frankenphp/archive/refs/heads/main.tar.gz | tar xz
cd frankenphp-main/caddy/frankenphp
CGO_CFLAGS=$(php-config --includes) CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" go build -tags=nobadger,nomysql,nopgx
```
