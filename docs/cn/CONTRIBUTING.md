# 贡献

## 编译 PHP

### 使用 Docker (Linux)

构建开发环境 Docker 镜像：

```console
docker build -t frankenphp-dev -f dev.Dockerfile .
docker run --cap-add=SYS_PTRACE --security-opt seccomp=unconfined -p 8080:8080 -p 443:443 -p 443:443/udp -v $PWD:/go/src/app -it frankenphp-dev
```

该镜像包含常用的开发工具（Go、GDB、Valgrind、Neovim等）并使用以下 php 设置位置

- php.ini: `/etc/frankenphp/php.ini` 默认提供了一个带有开发预设的 php.ini 文件。
- 附加配置文件: `/etc/frankenphp/php.d/*.ini`
- php 扩展: `/usr/lib/frankenphp/modules/`

如果你的 Docker 版本低于 23.0，则会因为 dockerignore [pattern issue](https://github.com/moby/moby/pull/42676) 而导致构建失败。将目录添加到 `.dockerignore`。

```patch
 !testdata/*.php
 !testdata/*.txt
+!caddy
+!internal
```

### 不使用 Docker (Linux 和 macOS)

[按照说明从源代码编译](https://frankenphp.dev/docs/compile/) 并传递 `--debug` 配置标志。

## 运行测试套件

```console
go test -tags watcher -race -v ./...
```

## Caddy 模块

使用 FrankenPHP Caddy 模块构建 Caddy：

```console
cd caddy/frankenphp/
go build -tags watcher,brotli,nobadger,nomysql,nopgx
cd ../../
```

使用 FrankenPHP Caddy 模块运行 Caddy：

```console
cd testdata/
../caddy/frankenphp/frankenphp run
```

服务器正在监听 `127.0.0.1:80`：

> [!NOTE]
> 如果您正在使用 Docker，您必须绑定容器的 80 端口或者在容器内部执行命令。

```console
curl -vk http://127.0.0.1/phpinfo.php
```

## 最小测试服务器

构建最小测试服务器：

```console
cd internal/testserver/
go build
cd ../../
```

运行测试服务器：

```console
cd testdata/
../internal/testserver/testserver
```

服务器正在监听 `127.0.0.1:8080`：

```console
curl -v http://127.0.0.1:8080/phpinfo.php
```

## 本地构建 Docker 镜像

打印 bake 计划:

```console
docker buildx bake -f docker-bake.hcl --print
```

本地构建 amd64 的 FrankenPHP 镜像：

```console
docker buildx bake -f docker-bake.hcl --pull --load --set "*.platform=linux/amd64"
```

本地构建 arm64 的 FrankenPHP 镜像：

```console
docker buildx bake -f docker-bake.hcl --pull --load --set "*.platform=linux/arm64"
```

从头开始为 arm64 和 amd64 构建 FrankenPHP 镜像并推送到 Docker Hub：

```console
docker buildx bake -f docker-bake.hcl --pull --no-cache --push
```

## 使用静态构建调试分段错误

1. 从 GitHub 下载 FrankenPHP 二进制文件的调试版本或创建包含调试符号的自定义静态构建：

   ```console
   docker buildx bake \
       --load \
       --set static-builder.args.DEBUG_SYMBOLS=1 \
       --set "static-builder.platform=linux/amd64" \
       static-builder
   docker cp $(docker create --name static-builder-musl dunglas/frankenphp:static-builder-musl):/go/src/app/dist/frankenphp-linux-$(uname -m) frankenphp
   ```

2. 将当前版本的 `frankenphp` 替换为 debug FrankenPHP 可执行文件
3. 照常启动 FrankenPHP（或者，你可以直接使用 GDB 启动 FrankenPHP： `gdb --args frankenphp run`）
4. 使用 GDB 附加到进程：

   ```console
   gdb -p `pidof frankenphp`
   ```

5. 如有必要，请在 GDB shell 中输入 `continue`
6. 使 FrankenPHP 崩溃
7. 在 GDB shell 中输入 `bt`
8. 复制输出

## 在 GitHub Actions 中调试分段错误

1. 打开 `.github/workflows/tests.yml`
2. 启用 PHP 调试符号

   ```patch
       - uses: shivammathur/setup-php@v2
         # ...
         env:
           phpts: ts
   +       debug: true
   ```

3. 启用 `tmate` 以连接到容器

   ```patch
       - name: Set CGO flags
         run: echo "CGO_CFLAGS=$(php-config --includes)" >> "$GITHUB_ENV"
   +   - run: |
   +       sudo apt install gdb
   +       mkdir -p /home/runner/.config/gdb/
   +       printf "set auto-load safe-path /\nhandle SIG34 nostop noprint pass" > /home/runner/.config/gdb/gdbinit
   +   - uses: mxschmitt/action-tmate@v3
   ```

4. 连接到容器
5. 打开 `frankenphp.go`
6. 启用 `cgosymbolizer`

   ```patch
   -	//_ "github.com/ianlancetaylor/cgosymbolizer"
   +	_ "github.com/ianlancetaylor/cgosymbolizer"
   ```

7. 下载模块： `go get`
8. 在容器中，可以使用 GDB 和以下：

   ```console
   go test -tags watcher -c -ldflags=-w
   gdb --args frankenphp.test -test.run ^MyTest$
   ```

9. 当错误修复后，恢复所有这些更改

## 其他开发资源

- [PHP 嵌入 uWSGI](https://github.com/unbit/uwsgi/blob/master/plugins/php/php_plugin.c)
- [PHP 嵌入 NGINX Unit](https://github.com/nginx/unit/blob/master/src/nxt_php_sapi.c)
- [PHP 嵌入 Go (go-php)](https://github.com/deuill/go-php)
- [PHP 嵌入 Go (GoEmPHP)](https://github.com/mikespook/goemphp)
- [PHP 嵌入 C++](https://gist.github.com/paresy/3cbd4c6a469511ac7479aa0e7c42fea7)
- [扩展和嵌入 PHP 作者：Sara Golemon](https://books.google.fr/books?id=zMbGvK17_tYC&pg=PA254&lpg=PA254#v=onepage&q&f=false)
- [TSRMLS_CC到底是什么？](http://blog.golemon.com/2006/06/what-heck-is-tsrmlscc-anyway.html)
- [SDL 绑定](https://pkg.go.dev/github.com/veandco/go-sdl2@v0.4.21/sdl#Main)

## Docker 相关资源

- [Bake 文件定义](https://docs.docker.com/build/customize/bake/file-definition/)
- [`docker buildx build`](https://docs.docker.com/engine/reference/commandline/buildx_build/)

## 有用的命令

```console
apk add strace util-linux gdb
strace -e 'trace=!futex,epoll_ctl,epoll_pwait,tgkill,rt_sigreturn' -p 1
```

## 翻译文档

要将文档和网站翻译成新语言，请按照下列步骤操作：

1. 在此存储库的 `docs/` 目录中创建一个以语言的 2 个字符的 ISO 代码命名的新目录
2. 将 `docs/` 目录根目录中的所有 `.md` 文件复制到新目录中（始终使用英文版本作为翻译源，因为它始终是最新的）
3. 将 `README.md` 和 `CONTRIBUTING.md` 文件从根目录复制到新目录
4. 翻译文件的内容，但不要更改文件名，也不要翻译以 `> [!` 开头的字符串（这是 GitHub 的特殊标记）
5. 创建翻译的拉取请求
6. 在 [站点存储库](https://github.com/dunglas/frankenphp-website/tree/main) 中，复制并翻译 `content/`、`data/` 和 `i18n/` 目录中的翻译文件
7. 转换创建的 YAML 文件中的值
8. 在站点存储库上打开拉取请求
