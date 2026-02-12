# Compilar desde fuentes

Este documento explica cómo crear un binario de FrankenPHP que cargará PHP como una biblioteca dinámica.
Esta es la forma recomendada.

Alternativamente, también se pueden crear [compilaciones estáticas y mayormente estáticas](static.md).

## Instalar PHP

FrankenPHP es compatible con PHP 8.2 y versiones superiores.

### Con Homebrew (Linux y Mac)

La forma más sencilla de instalar una versión de libphp compatible con FrankenPHP es usar los paquetes ZTS proporcionados por [Homebrew PHP](https://github.com/shivammathur/homebrew-php).

Primero, si no lo ha hecho ya, instale [Homebrew](https://brew.sh).

Luego, instale la variante ZTS de PHP, Brotli (opcional, para soporte de compresión) y watcher (opcional, para detección de cambios en archivos):

```console
brew install shivammathur/php/php-zts brotli watcher
brew link --overwrite --force shivammathur/php/php-zts
```

### Compilando PHP

Alternativamente, puede compilar PHP desde las fuentes con las opciones necesarias para FrankenPHP siguiendo estos pasos.

Primero, [obtenga las fuentes de PHP](https://www.php.net/downloads.php) y extráigalas:

```console
tar xf php-*
cd php-*/
```

Luego, ejecute el script `configure` con las opciones necesarias para su plataforma.
Las siguientes banderas de `./configure` son obligatorias, pero puede agregar otras, por ejemplo, para compilar extensiones o características adicionales.

#### Linux

```console
./configure \
    --enable-embed \
    --enable-zts \
    --disable-zend-signals \
    --enable-zend-max-execution-timers
```

#### Mac

Use el gestor de paquetes [Homebrew](https://brew.sh/) para instalar las dependencias requeridas y opcionales:

```console
brew install libiconv bison brotli re2c pkg-config watcher
echo 'export PATH="/opt/homebrew/opt/bison/bin:$PATH"' >> ~/.zshrc
```

Luego ejecute el script de configuración:

```console
./configure \
    --enable-embed \
    --enable-zts \
    --disable-zend-signals \
    --with-iconv=/opt/homebrew/opt/libiconv/
```

#### Compilar PHP

Finalmente, compile e instale PHP:

```console
make -j"$(getconf _NPROCESSORS_ONLN)"
sudo make install
```

## Instalar dependencias opcionales

Algunas características de FrankenPHP dependen de dependencias opcionales del sistema que deben instalarse.
Alternativamente, estas características pueden deshabilitarse pasando etiquetas de compilación al compilador Go.

| Característica                     | Dependencia                                                                                                   | Etiqueta de compilación para deshabilitarla |
| ----------------------------------- | ------------------------------------------------------------------------------------------------------------- | -------------------------------------------- |
| Compresión Brotli                  | [Brotli](https://github.com/google/brotli)                                                                   | nobrotli                                    |
| Reiniciar workers al cambiar archivos | [Watcher C](https://github.com/e-dant/watcher/tree/release/watcher-c)                                        | nowatcher                                   |
| [Mercure](mercure.md)              | [Biblioteca Mercure Go](https://pkg.go.dev/github.com/dunglas/mercure) (instalada automáticamente, licencia AGPL) | nomercure                                   |

## Compilar la aplicación Go

Ahora puede construir el binario final.

### Usando xcaddy

La forma recomendada es usar [xcaddy](https://github.com/caddyserver/xcaddy) para compilar FrankenPHP.
`xcaddy` también permite agregar fácilmente [módulos personalizados de Caddy](https://caddyserver.com/docs/modules/) y extensiones de FrankenPHP:

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/dunglas/frankenphp/caddy \
    --with github.com/dunglas/mercure/caddy \
    --with github.com/dunglas/vulcain/caddy \
    --with github.com/dunglas/caddy-cbrotli
    # Agregue módulos adicionales de Caddy y extensiones de FrankenPHP aquí
    # opcionalmente, si desea compilar desde sus fuentes de frankenphp:
    # --with github.com/dunglas/frankenphp=$(pwd) \
    # --with github.com/dunglas/frankenphp/caddy=$(pwd)/caddy

```

> [!TIP]
>
> Si está usando musl libc (predeterminado en Alpine Linux) y Symfony,
> es posible que deba aumentar el tamaño de pila predeterminado.
> De lo contrario, podría obtener errores como `PHP Fatal error: Maximum call stack size of 83360 bytes reached during compilation. Try splitting expression`
>
> Para hacerlo, cambie la variable de entorno `XCADDY_GO_BUILD_FLAGS` a algo como:
> `XCADDY_GO_BUILD_FLAGS=$'-ldflags "-w -s -extldflags \'-Wl,-z,stack-size=0x80000\'"'`
> (cambie el valor del tamaño de pila según las necesidades de su aplicación).

### Sin xcaddy

Alternativamente, es posible compilar FrankenPHP sin `xcaddy` usando directamente el comando `go`:

```console
curl -L https://github.com/php/frankenphp/archive/refs/heads/main.tar.gz | tar xz
cd frankenphp-main/caddy/frankenphp
CGO_CFLAGS=$(php-config --includes) CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" go build -tags=nobadger,nomysql,nopgx
```
