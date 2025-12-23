# Contribuir

## Compilar PHP

### Con Docker (Linux)

Construya la imagen Docker de desarrollo:

```console
docker build -t frankenphp-dev -f dev.Dockerfile .
docker run --cap-add=SYS_PTRACE --security-opt seccomp=unconfined -p 8080:8080 -p 443:443 -p 443:443/udp -v $PWD:/go/src/app -it frankenphp-dev
```

La imagen contiene las herramientas de desarrollo habituales (Go, GDB, Valgrind, Neovim...) y utiliza las siguientes ubicaciones de configuración de PHP:

- php.ini: `/etc/frankenphp/php.ini` Se proporciona un archivo php.ini con ajustes preestablecidos de desarrollo por defecto.
- archivos de configuración adicionales: `/etc/frankenphp/php.d/*.ini`
- extensiones php: `/usr/lib/frankenphp/modules/`

Si su versión de Docker es inferior a 23.0, la construcción fallará debido a un [problema de patrón](https://github.com/moby/moby/pull/42676) en `.dockerignore`. Agregue los directorios a `.dockerignore`:

```patch
 !testdata/*.php
 !testdata/*.txt
+!caddy
+!internal
```

### Sin Docker (Linux y macOS)

[Siga las instrucciones para compilar desde las fuentes](compile.md) y pase la bandera de configuración `--debug`.

## Ejecutar la suite de pruebas

```console
go test -tags watcher -race -v ./...
```

## Módulo Caddy

Construir Caddy con el módulo FrankenPHP:

```console
cd caddy/frankenphp/
go build -tags watcher,brotli,nobadger,nomysql,nopgx
cd ../../
```

Ejecutar Caddy con el módulo FrankenPHP:

```console
cd testdata/
../caddy/frankenphp/frankenphp run
```

El servidor está configurado para escuchar en la dirección `127.0.0.1:80`:

> [!NOTE]
>
> Si está usando Docker, deberá enlazar el puerto 80 del contenedor o ejecutar desde dentro del contenedor.

```console
curl -vk http://127.0.0.1/phpinfo.php
```

## Servidor de prueba mínimo

Construir el servidor de prueba mínimo:

```console
cd internal/testserver/
go build
cd ../../
```

Iniciar el servidor de prueba:

```console
cd testdata/
../internal/testserver/testserver
```

El servidor está configurado para escuchar en la dirección `127.0.0.1:8080`:

```console
curl -v http://127.0.0.1:8080/phpinfo.php
```

## Construir localmente las imágenes Docker

Mostrar el plan de compilación:

```console
docker buildx bake -f docker-bake.hcl --print
```

Construir localmente las imágenes FrankenPHP para amd64:

```console
docker buildx bake -f docker-bake.hcl --pull --load --set "*.platform=linux/amd64"
```

Construir localmente las imágenes FrankenPHP para arm64:

```console
docker buildx bake -f docker-bake.hcl --pull --load --set "*.platform=linux/arm64"
```

Construir desde cero las imágenes FrankenPHP para arm64 y amd64 y subirlas a Docker Hub:

```console
docker buildx bake -f docker-bake.hcl --pull --no-cache --push
```

## Depurar errores de segmentación con las compilaciones estáticas

1. Descargue la versión de depuración del binario FrankenPHP desde GitHub o cree su propia compilación estática incluyendo símbolos de depuración:

   ```console
   docker buildx bake \
       --load \
       --set static-builder.args.DEBUG_SYMBOLS=1 \
       --set "static-builder.platform=linux/amd64" \
       static-builder
   docker cp $(docker create --name static-builder-musl dunglas/frankenphp:static-builder-musl):/go/src/app/dist/frankenphp-linux-$(uname -m) frankenphp
   ```

2. Reemplace su versión actual de `frankenphp` por el ejecutable de depuración de FrankenPHP.
3. Inicie FrankenPHP como de costumbre (alternativamente, puede iniciar FrankenPHP directamente con GDB: `gdb --args frankenphp run`).
4. Adjunte el proceso con GDB:

   ```console
   gdb -p `pidof frankenphp`
   ```

5. Si es necesario, escriba `continue` en el shell de GDB.
6. Haga que FrankenPHP falle.
7. Escriba `bt` en el shell de GDB.
8. Copie la salida.

## Depurar errores de segmentación en GitHub Actions

1. Abrir `.github/workflows/tests.yml`
2. Activar los símbolos de depuración de la biblioteca PHP:

   ```patch
       - uses: shivammathur/setup-php@v2
         # ...
         env:
           phpts: ts
   +       debug: true
   ```

3. Activar `tmate` para conectarse al contenedor:

   ```patch
       - name: Set CGO flags
         run: echo "CGO_CFLAGS=$(php-config --includes)" >> "$GITHUB_ENV"
   +   - run: |
   +       sudo apt install gdb
   +       mkdir -p /home/runner/.config/gdb/
   +       printf "set auto-load safe-path /\nhandle SIG34 nostop noprint pass" > /home/runner/.config/gdb/gdbinit
   +   - uses: mxschmitt/action-tmate@v3
   ```

4. Conectarse al contenedor.
5. Abrir `frankenphp.go`.
6. Activar `cgosymbolizer`:

   ```patch
   - //_ "github.com/ianlancetaylor/cgosymbolizer"
   + _ "github.com/ianlancetaylor/cgosymbolizer"
   ```

7. Descargar el módulo: `go get`.
8. Dentro del contenedor, puede usar GDB y similares:

   ```console
   go test -tags watcher -c -ldflags=-w
   gdb --args frankenphp.test -test.run ^MyTest$
   ```

9. Cuando el error esté corregido, revierta todos los cambios.

## Recursos diversos para el desarrollo

- [Integración de PHP en uWSGI](https://github.com/unbit/uwsgi/blob/master/plugins/php/php_plugin.c)
- [Integración de PHP en NGINX Unit](https://github.com/nginx/unit/blob/master/src/nxt_php_sapi.c)
- [Integración de PHP en Go (go-php)](https://github.com/deuill/go-php)
- [Integración de PHP en Go (GoEmPHP)](https://github.com/mikespook/goemphp)
- [Integración de PHP en C++](https://gist.github.com/paresy/3cbd4c6a469511ac7479aa0e7c42fea7)
- [Extending and Embedding PHP por Sara Golemon](https://books.google.fr/books?id=zMbGvK17_tYC&pg=PA254&lpg=PA254#v=onepage&q&f=false)
- [¿Qué es TSRMLS_CC, exactamente?](http://blog.golemon.com/2006/06/what-heck-is-tsrmlscc-anyway.html)
- [Integración de PHP en Mac](https://gist.github.com/jonnywang/61427ffc0e8dde74fff40f479d147db4)
- [Bindings SDL](https://pkg.go.dev/github.com/veandco/go-sdl2@v0.4.21/sdl#Main)

## Recursos relacionados con Docker

- [Definición del archivo Bake](https://docs.docker.com/build/customize/bake/file-definition/)
- [`docker buildx build`](https://docs.docker.com/engine/reference/commandline/buildx_build/)

## Comando útil

```console
apk add strace util-linux gdb
strace -e 'trace=!futex,epoll_ctl,epoll_pwait,tgkill,rt_sigreturn' -p 1
```

## Traducir la documentación

Para traducir la documentación y el sitio a un nuevo idioma, siga estos pasos:

1. Cree un nuevo directorio con el código ISO de 2 caracteres del idioma en el directorio `docs/` de este repositorio.
2. Copie todos los archivos `.md` de la raíz del directorio `docs/` al nuevo directorio (siempre use la versión en inglés como fuente de traducción, ya que siempre está actualizada).
3. Copie los archivos `README.md` y `CONTRIBUTING.md` del directorio raíz al nuevo directorio.
4. Traduzca el contenido de los archivos, pero no cambie los nombres de los archivos, tampoco traduzca las cadenas que comiencen por `> [!` (es un marcado especial para GitHub).
5. Cree una Pull Request con las traducciones.
6. En el [repositorio del sitio](https://github.com/dunglas/frankenphp-website/tree/main), copie y traduzca los archivos de traducción en los directorios `content/`, `data/` y `i18n/`.
7. Traduzca los valores en el archivo YAML creado.
8. Abra una Pull Request en el repositorio del sitio.
