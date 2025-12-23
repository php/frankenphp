# Crear una Compilación Estática

En lugar de usar una instalación local de la biblioteca PHP,
es posible crear una compilación estática o mayormente estática de FrankenPHP gracias al excelente [proyecto static-php-cli](https://github.com/crazywhalecc/static-php-cli) (a pesar de su nombre, este proyecto soporta todas las SAPI, no solo CLI).

Con este método, un único binario portátil contendrá el intérprete de PHP, el servidor web Caddy y FrankenPHP.

Los ejecutables nativos completamente estáticos no requieren dependencias y pueden ejecutarse incluso en la imagen Docker [`scratch`](https://docs.docker.com/build/building/base-images/#create-a-minimal-base-image-using-scratch).
Sin embargo, no pueden cargar extensiones PHP dinámicas (como Xdebug) y tienen algunas limitaciones porque usan la libc musl.

Los binarios mayormente estáticos solo requieren `glibc` y pueden cargar extensiones dinámicas.

Cuando sea posible, recomendamos usar compilaciones mayormente estáticas basadas en glibc.

FrankenPHP también soporta [incrustar la aplicación PHP en el binario estático](embed.md).

## Linux

Proporcionamos imágenes Docker para compilar binarios Linux estáticos:

### Compilación Completamente Estática Basada en musl

Para un binario completamente estático que se ejecuta en cualquier distribución Linux sin dependencias pero que no soporta carga dinámica de extensiones:

```console
docker buildx bake --load static-builder-musl
docker cp $(docker create --name static-builder-musl dunglas/frankenphp:static-builder-musl):/go/src/app/dist/frankenphp-linux-$(uname -m) frankenphp ; docker rm static-builder-musl
```

Para un mejor rendimiento en escenarios altamente concurrentes, considera usar el asignador [mimalloc](https://github.com/microsoft/mimalloc).

```console
docker buildx bake --load --set static-builder-musl.args.MIMALLOC=1 static-builder-musl
```

### Compilación Mayormente Estática Basada en glibc (Con Soporte para Extensiones Dinámicas)

Para un binario que soporta la carga dinámica de extensiones PHP mientras tiene las extensiones seleccionadas compiladas estáticamente:

```console
docker buildx bake --load static-builder-gnu
docker cp $(docker create --name static-builder-gnu dunglas/frankenphp:static-builder-gnu):/go/src/app/dist/frankenphp-linux-$(uname -m) frankenphp ; docker rm static-builder-gnu
```

Este binario soporta todas las versiones de glibc 2.17 y superiores, pero no se ejecuta en sistemas basados en musl (como Alpine Linux).

El binario resultante (mayormente estático excepto por `glibc`) se llama `frankenphp` y está disponible en el directorio actual.

Si deseas compilar el binario estático sin Docker, consulta las instrucciones para macOS, que también funcionan para Linux.

### Extensiones Personalizadas

Por omisión, se compilan las extensiones PHP más populares.

Para reducir el tamaño del binario y disminuir la superficie de ataque, puedes elegir la lista de extensiones a compilar usando el ARG de Docker `PHP_EXTENSIONS`.

Por ejemplo, ejecuta el siguiente comando para compilar solo la extensión `opcache`:

```console
docker buildx bake --load --set static-builder-musl.args.PHP_EXTENSIONS=opcache,pdo_sqlite static-builder-musl
# ...
```

Para agregar bibliotecas que habiliten funcionalidades adicionales a las extensiones que has habilitado, puedes pasar el ARG de Docker `PHP_EXTENSION_LIBS`:

```console
docker buildx bake \
  --load \
  --set static-builder-musl.args.PHP_EXTENSIONS=gd \
  --set static-builder-musl.args.PHP_EXTENSION_LIBS=libjpeg,libwebp \
  static-builder-musl
```

### Módulos Adicionales de Caddy

Para agregar módulos adicionales de Caddy o pasar otros argumentos a [xcaddy](https://github.com/caddyserver/xcaddy), usa el ARG de Docker `XCADDY_ARGS`:

```console
docker buildx bake \
  --load \
  --set static-builder-musl.args.XCADDY_ARGS="--with github.com/darkweak/souin/plugins/caddy --with github.com/dunglas/caddy-cbrotli --with github.com/dunglas/mercure/caddy --with github.com/dunglas/vulcain/caddy" \
  static-builder-musl
```

En este ejemplo, agregamos el módulo de caché HTTP [Souin](https://souin.io) para Caddy, así como los módulos [cbrotli](https://github.com/dunglas/caddy-cbrotli), [Mercure](https://mercure.rocks) y [Vulcain](https://vulcain.rocks).

> [!TIP]
>
> Los módulos cbrotli, Mercure y Vulcain están incluidos por omisión si `XCADDY_ARGS` está vacío o no está configurado.
> Si personalizas el valor de `XCADDY_ARGS`, debes incluirlos explícitamente si deseas que estén incluidos.

Consulta también cómo [personalizar la compilación](#personalizando-la-compilación).

### Token de GitHub

Si alcanzas el límite de tasa de la API de GitHub, establece un Token de Acceso Personal de GitHub en una variable de entorno llamada `GITHUB_TOKEN`:

```console
GITHUB_TOKEN="xxx" docker --load buildx bake static-builder-musl
# ...
```

## macOS

Ejecuta el siguiente script para crear un binario estático para macOS (debes tener [Homebrew](https://brew.sh/) instalado):

```console
git clone https://github.com/php/frankenphp
cd frankenphp
./build-static.sh
```

Nota: este script también funciona en Linux (y probablemente en otros Unix) y es usado internamente por las imágenes Docker que proporcionamos.

## Personalizando la Compilación

Las siguientes variables de entorno pueden pasarse a `docker build` y al script `build-static.sh` para personalizar la compilación estática:

- `FRANKENPHP_VERSION`: la versión de FrankenPHP a usar
- `PHP_VERSION`: la versión de PHP a usar
- `PHP_EXTENSIONS`: las extensiones PHP a compilar ([lista de extensiones soportadas](https://static-php.dev/en/guide/extensions.html))
- `PHP_EXTENSION_LIBS`: bibliotecas adicionales a compilar que añaden funcionalidades a las extensiones
- `XCADDY_ARGS`: argumentos a pasar a [xcaddy](https://github.com/caddyserver/xcaddy), por ejemplo para agregar módulos adicionales de Caddy
- `EMBED`: ruta de la aplicación PHP a incrustar en el binario
- `CLEAN`: cuando está establecido, libphp y todas sus dependencias se compilan desde cero (sin caché)
- `NO_COMPRESS`: no comprimir el binario resultante usando UPX
- `DEBUG_SYMBOLS`: cuando está establecido, los símbolos de depuración no se eliminarán y se añadirán al binario
- `MIMALLOC`: (experimental, solo Linux) reemplaza mallocng de musl por [mimalloc](https://github.com/microsoft/mimalloc) para mejorar el rendimiento. Solo recomendamos usar esto para compilaciones orientadas a musl; para glibc, preferimos deshabilitar esta opción y usar [`LD_PRELOAD`](https://microsoft.github.io/mimalloc/overrides.html) cuando ejecutes tu binario.
- `RELEASE`: (solo para mantenedores) cuando está establecido, el binario resultante se subirá a GitHub

## Extensiones

Con los binarios basados en glibc o macOS, puedes cargar extensiones PHP dinámicamente. Sin embargo, estas extensiones deberán ser compiladas con soporte ZTS.
Dado que la mayoría de los gestores de paquetes no ofrecen actualmente versiones ZTS de sus extensiones, tendrás que compilarlas tú mismo.

Para esto, puedes compilar y ejecutar el contenedor Docker `static-builder-gnu`, acceder a él y compilar las extensiones con `./configure --with-php-config=/go/src/app/dist/static-php-cli/buildroot/bin/php-config`.

Pasos de ejemplo para [la extensión Xdebug](https://xdebug.org):

```console
docker build -t gnu-ext -f static-builder-gnu.Dockerfile --build-arg FRANKENPHP_VERSION=1.0 .
docker create --name static-builder-gnu -it gnu-ext /bin/sh
docker start static-builder-gnu
docker exec -it static-builder-gnu /bin/sh
cd /go/src/app/dist/static-php-cli/buildroot/bin
git clone https://github.com/xdebug/xdebug.git && cd xdebug
source scl_source enable devtoolset-10
../phpize
./configure --with-php-config=/go/src/app/dist/static-php-cli/buildroot/bin/php-config
make
exit
docker cp static-builder-gnu:/go/src/app/dist/static-php-cli/buildroot/bin/xdebug/modules/xdebug.so xdebug-zts.so
docker cp static-builder-gnu:/go/src/app/dist/frankenphp-linux-$(uname -m) ./frankenphp
docker stop static-builder-gnu
docker rm static-builder-gnu
docker rmi gnu-ext
```

Esto creará `frankenphp` y `xdebug-zts.so` en el directorio actual.
Si mueves `xdebug-zts.so` a tu directorio de extensiones, agrega `zend_extension=xdebug-zts.so` a tu php.ini y ejecuta FrankenPHP, cargará Xdebug.
