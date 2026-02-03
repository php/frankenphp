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
export CGO_CFLAGS=$(php-config --includes) CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)"
go test -race -v ./...
```

## Caddy module

Build Caddy with the FrankenPHP Caddy module:

```console
cd caddy/frankenphp/
go build -tags nobadger,nomysql,nopgx
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
   go test -tags -c -ldflags=-w
   gdb --args frankenphp.test -test.run ^MyTest$
   ```

9. When the bug is fixed, revert all these changes

## Development Environment Setup

---

### For Windows: WSL2 Setup

FrankenPHP cannot be compiled natively on Windows, so to build and debug code, you need to run your IDE in WSL.

1. Install WSL2:

   ```powershell
   wsl --install
   ```

---

### Initial setup

Follow the instructions in [compiling from sources](https://frankenphp.dev/docs/compile/).
The steps assume the following environment:

- Go installed at `/usr/local/go`
- PHP source cloned to `~/php-src`
- PHP built at: `/usr/local/bin/php`
- FrankenPHP source cloned to `~/frankenphp`

### CLion Setup for CGO glue/PHP Source Development

1. Install CLion (on your host OS)

   - Download from [JetBrains](https://www.jetbrains.com/clion/download/)
   - Launch (if on Windows, in WSL):

     ```bash
     clion &>/dev/null
     ```

2. Open Project in CLion

   - Open CLion → Open → Select the `~/frankenphp` directory
   - Add a build chain: Settings → Build, Execution, Deployment → Custom Build Targets
   - Select any Build Target, under `Build` set up an External Tool (call it e.g. go build)
   - Set up a wrapper script that builds frankenphp for you, called `go_compile_frankenphp.sh`

   ```bash
   export CGO_CFLAGS="-O0 -g $(php-config --includes)"
   export CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)"
   go build -tags=nobadger,nomysql,nopgx
   ```

   - Under Program, select `go_compile_frankenphp.sh`
   - Leave Arguments blank
   - Working Directory: `~/frankenphp/caddy/frankenphp`

3. Configure Run Targets

   - Go to Run → Edit Configurations
   - Create:
     - frankenphp:
       - Type: Native Application
       - Target: select the `go build` target you created
       - Executable: `~/frankenphp/caddy/frankenphp/frankenphp`
       - Arguments: the arguments you want to start frankenphp with, e.g. `php-cli test.php`

4. Debug Go files from CLion

   - Right click on a *.go file in the Project view on the left
   - Override file type → C/C++

   Now you can place breakpoints in C, C++ and Go files.
   To get syntax highlighting for imports from php-src, you may need to tell CLion about the include paths. Create a
   `compile_flags.txt` file in `~/frankenphp` with the following contents:

   ```gcc
   -I/usr/local/include/php
   -I/usr/local/include/php/Zend
   -I/usr/local/include/php/main
   -I/usr/local/include/php/TSRM
   ```

---

### GoLand Setup for FrankenPHP Development

Use GoLand for primary Go development, but the debugger cannot debug C code.

1. Install GoLand (on your host OS)

   - Download from [JetBrains](https://www.jetbrains.com/go/download/)
   - Launch (if on Windows, in WSL):

     ```bash
     goland &>/dev/null
     ```

2. Open in GoLand

   - Launch GoLand → Open → Select the `~/frankenphp` directory

---

### Go Configuration

- Select Go Build
  - Name `frankenphp`
  - Run kind: Directory
- Directory: `~/frankenphp/caddy/frankenphp`
- Output directory: `~/frankenphp/caddy/frankenphp`
- Working directory: `~/frankenphp/caddy/frankenphp`
- Environment (adjust for your $(php-config ...) output):
  `CGO_CFLAGS=-O0 -g -I/usr/local/include/php -I/usr/local/include/php/main -I/usr/local/include/php/TSRM -I/usr/local/include/php/Zend -I/usr/local/include/php/ext -I/usr/local/include/php/ext/date/lib;CGO_LDFLAGS=-lm -lpthread -lsqlite3 -lxml2 -lbrotlienc -lbrotlidec -lbrotlicommon -lwatcher`
- Go tool arguments: `-tags=nobadger,nomysql,nopgx`
- Program arguments: e.g. `php-cli -i`

You can now place breakpoints and debug through Go code when you debug the `frankenphp` configuration, but breakpoints
in C code will not work.

---

### Debugging and Integration Notes

- Use CLion for debugging PHP internals and `cgo` glue code
- Use GoLand for primary Go development and debugging
- FrankenPHP can be added as a run configuration in CLion for unified C/Go debugging if needed, but syntax highlighting won't work in Go files

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
