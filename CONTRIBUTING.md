# Contributing

## Compiling PHP

### With Docker (Linux)

Build the dev Docker image:

```console
docker build -t frankenphp-dev -f dev.Dockerfile .
docker run --cap-add=SYS_PTRACE --security-opt seccomp=unconfined -p 8080:8080 -p 443:443 -p 443:443/udp -v $PWD:/go/src/app -it frankenphp-dev
```

The image contains the usual development tools (Go, GDB, Valgrind, Neovim...) and uses the following php setting locations

- php.ini: `/etc/frankenphp/php.ini` A php.ini file with development presets is provided by default.
- additional configuration files: `/etc/frankenphp/php.d/*.ini`
- php extensions: `/usr/lib/frankenphp/modules/`

If your Docker version is lower than 23.0, the build will fail due to dockerignore [pattern issue](https://github.com/moby/moby/pull/42676). Add directories to `.dockerignore`.

```patch
 !testdata/*.php
 !testdata/*.txt
+!caddy
+!internal
```

### Without Docker (Linux and macOS)

[Follow the instructions to compile from sources](https://frankenphp.dev/docs/compile/) and pass the `--debug` configuration flag.

## Running the test suite

```console
go test -tags watcher -race -v ./...
```

## Caddy module

Build Caddy with the FrankenPHP Caddy module:

```console
cd caddy/frankenphp/
go build -tags watcher,brotli,nobadger,nomysql,nopgx
cd ../../
```

Run the Caddy with the FrankenPHP Caddy module:

```console
cd testdata/
../caddy/frankenphp/frankenphp run
```

The server is listening on `127.0.0.1:80`:

> [!NOTE]
> if you are using Docker, you will have to either bind container port 80 or execute from inside the container

```console
curl -vk http://127.0.0.1/phpinfo.php
```

## Minimal test server

Build the minimal test server:

```console
cd internal/testserver/
go build
cd ../../
```

Run the test server:

```console
cd testdata/
../internal/testserver/testserver
```

The server is listening on `127.0.0.1:8080`:

```console
curl -v http://127.0.0.1:8080/phpinfo.php
```

## Building Docker Images Locally

Print bake plan:

```console
docker buildx bake -f docker-bake.hcl --print
```

Build FrankenPHP images for amd64 locally:

```console
docker buildx bake -f docker-bake.hcl --pull --load --set "*.platform=linux/amd64"
```

Build FrankenPHP images for arm64 locally:

```console
docker buildx bake -f docker-bake.hcl --pull --load --set "*.platform=linux/arm64"
```

Build FrankenPHP images from scratch for arm64 & amd64 and push to Docker Hub:

```console
docker buildx bake -f docker-bake.hcl --pull --no-cache --push
```

## Debugging Segmentation Faults With Static Builds

1. Download the debug version of the FrankenPHP binary from GitHub or create your custom static build including debug symbols:

   ```console
   docker buildx bake \
       --load \
       --set static-builder.args.DEBUG_SYMBOLS=1 \
       --set "static-builder.platform=linux/amd64" \
       static-builder
   docker cp $(docker create --name static-builder-musl dunglas/frankenphp:static-builder-musl):/go/src/app/dist/frankenphp-linux-$(uname -m) frankenphp
   ```

2. Replace your current version of `frankenphp` by the debug FrankenPHP executable
3. Start FrankenPHP as usual (alternatively, you can directly start FrankenPHP with GDB: `gdb --args frankenphp run`)
4. Attach to the process with GDB:

   ```console
   gdb -p `pidof frankenphp`
   ```

5. If necessary, type `continue` in the GDB shell
6. Make FrankenPHP crash
7. Type `bt` in the GDB shell
8. Copy the output

## Debugging Segmentation Faults in GitHub Actions

1. Open `.github/workflows/tests.yml`
2. Enable PHP debug symbols

   ```patch
       - uses: shivammathur/setup-php@v2
         # ...
         env:
           phpts: ts
   +       debug: true
   ```

3. Enable `tmate` to connect to the container

   ```patch
       - name: Set CGO flags
         run: echo "CGO_CFLAGS=$(php-config --includes)" >> "$GITHUB_ENV"
   +   - run: |
   +       sudo apt install gdb
   +       mkdir -p /home/runner/.config/gdb/
   +       printf "set auto-load safe-path /\nhandle SIG34 nostop noprint pass" > /home/runner/.config/gdb/gdbinit
   +   - uses: mxschmitt/action-tmate@v3
   ```

4. Connect to the container
5. Open `frankenphp.go`
6. Enable `cgosymbolizer`

   ```patch
   -	//_ "github.com/ianlancetaylor/cgosymbolizer"
   +	_ "github.com/ianlancetaylor/cgosymbolizer"
   ```

7. Download the module: `go get`
8. In the container, you can use GDB and the like:

   ```console
   go test -tags watcher -c -ldflags=-w
   gdb --args frankenphp.test -test.run ^MyTest$
   ```

9. When the bug is fixed, revert all these changes

## Misc Dev Resources

- [PHP embedding in uWSGI](https://github.com/unbit/uwsgi/blob/master/plugins/php/php_plugin.c)
- [PHP embedding in NGINX Unit](https://github.com/nginx/unit/blob/master/src/nxt_php_sapi.c)
- [PHP embedding in Go (go-php)](https://github.com/deuill/go-php)
- [PHP embedding in Go (GoEmPHP)](https://github.com/mikespook/goemphp)
- [PHP embedding in C++](https://gist.github.com/paresy/3cbd4c6a469511ac7479aa0e7c42fea7)
- [Extending and Embedding PHP by Sara Golemon](https://books.google.fr/books?id=zMbGvK17_tYC&pg=PA254&lpg=PA254#v=onepage&q&f=false)
- [What the heck is TSRMLS_CC, anyway?](http://blog.golemon.com/2006/06/what-heck-is-tsrmlscc-anyway.html)
- [SDL bindings](https://pkg.go.dev/github.com/veandco/go-sdl2@v0.4.21/sdl#Main)

## Docker-Related Resources

- [Bake file definition](https://docs.docker.com/build/customize/bake/file-definition/)
- [`docker buildx build`](https://docs.docker.com/engine/reference/commandline/buildx_build/)

## Useful Command

```console
apk add strace util-linux gdb
strace -e 'trace=!futex,epoll_ctl,epoll_pwait,tgkill,rt_sigreturn' -p 1
```

## Translating the Documentation

To translate the documentation and the site in a new language,
follow these steps:

1. Create a new directory named with the language's 2-character ISO code in this repository's `docs/` directory
2. Copy all the `.md` files in the root of the `docs/` directory into the new directory (always use the English version as source for translation, as it's always up to date)
3. Copy the `README.md` and `CONTRIBUTING.md` files from the root directory to the new directory
4. Translate the content of the files, but don't change the filenames, also don't translate strings starting with `> [!` (it's special markup for GitHub)
5. Create a Pull Request with the translations
6. In the [site repository](https://github.com/dunglas/frankenphp-website/tree/main), copy and translate the translation files in the `content/`, `data/` and `i18n/` directories
7. Translate the values in the created YAML file
8. Open a Pull Request on the site repository
