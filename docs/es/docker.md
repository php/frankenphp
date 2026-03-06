# Construir una imagen Docker personalizada

Las [imágenes Docker de FrankenPHP](https://hub.docker.com/r/dunglas/frankenphp) están basadas en [imágenes oficiales de PHP](https://hub.docker.com/_/php/).
Se proporcionan variantes para Debian y Alpine Linux en arquitecturas populares.
Se recomiendan las variantes de Debian.

Se proporcionan variantes para PHP 8.2, 8.3, 8.4 y 8.5.

Las etiquetas siguen este patrón: `dunglas/frankenphp:<versión-frankenphp>-php<versión-php>-<sistema-operativo>`

- `<versión-frankenphp>` y `<versión-php>` son los números de versión de FrankenPHP y PHP respectivamente, que van desde versiones principales (ej. `1`), menores (ej. `1.2`) hasta versiones de parche (ej. `1.2.3`).
- `<sistema-operativo>` es `trixie` (para Debian Trixie), `bookworm` (para Debian Bookworm) o `alpine` (para la última versión estable de Alpine).

[Explorar etiquetas](https://hub.docker.com/r/dunglas/frankenphp/tags).

## Cómo usar las imágenes

Cree un archivo `Dockerfile` en su proyecto:

```dockerfile
FROM dunglas/frankenphp

COPY . /app/public
```

Luego, ejecute estos comandos para construir y ejecutar la imagen Docker:

```console
docker build -t my-php-app .
docker run -it --rm --name my-running-app my-php-app
```

## Cómo ajustar la configuración

Para mayor comodidad, se proporciona en la imagen un [archivo `Caddyfile` predeterminado](https://github.com/php/frankenphp/blob/main/caddy/frankenphp/Caddyfile) que contiene variables de entorno útiles.

## Cómo instalar más extensiones de PHP

El script [`docker-php-extension-installer`](https://github.com/mlocati/docker-php-extension-installer) está disponible en la imagen base.
Agregar extensiones adicionales de PHP es sencillo:

```dockerfile
FROM dunglas/frankenphp

# agregue extensiones adicionales aquí:
RUN install-php-extensions \
	pdo_mysql \
	gd \
	intl \
	zip \
	opcache
```

## Cómo instalar más módulos de Caddy

FrankenPHP está construido sobre Caddy, y todos los [módulos de Caddy](https://caddyserver.com/docs/modules/) pueden usarse con FrankenPHP.

La forma más fácil de instalar módulos personalizados de Caddy es usar [xcaddy](https://github.com/caddyserver/xcaddy):

```dockerfile
FROM dunglas/frankenphp:builder AS builder

# Copie xcaddy en la imagen del constructor
COPY --from=caddy:builder /usr/bin/xcaddy /usr/bin/xcaddy

# CGO debe estar habilitado para construir FrankenPHP
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
        # Mercure y Vulcain están incluidos en la compilación oficial, pero puede eliminarlos si lo desea
        --with github.com/dunglas/mercure/caddy \
        --with github.com/dunglas/vulcain/caddy
        # Agregue módulos adicionales de Caddy aquí

FROM dunglas/frankenphp AS runner

# Reemplace el binario oficial por el que contiene sus módulos personalizados
COPY --from=builder /usr/local/bin/frankenphp /usr/local/bin/frankenphp
```

La imagen `builder` proporcionada por FrankenPHP contiene una versión compilada de `libphp`.
Se proporcionan [imágenes de constructor](https://hub.docker.com/r/dunglas/frankenphp/tags?name=builder) para todas las versiones de FrankenPHP y PHP, tanto para Debian como para Alpine.

> [!TIP]
>
> Si está usando Alpine Linux y Symfony,
> es posible que deba [aumentar el tamaño de pila predeterminado](compile.md#using-xcaddy).

## Habilitar el modo Worker por defecto

Establezca la variable de entorno `FRANKENPHP_CONFIG` para iniciar FrankenPHP con un script de worker:

```dockerfile
FROM dunglas/frankenphp

# ...

ENV FRANKENPHP_CONFIG="worker ./public/index.php"
```

## Usar un volumen en desarrollo

Para desarrollar fácilmente con FrankenPHP, monte el directorio de su host que contiene el código fuente de la aplicación como un volumen en el contenedor Docker:

```console
docker run -v $PWD:/app/public -p 80:80 -p 443:443 -p 443:443/udp --tty my-php-app
```

> [!TIP]
>
> La opción `--tty` permite tener logs legibles en lugar de logs en formato JSON.

Con Docker Compose:

```yaml
# compose.yaml

services:
  php:
    image: dunglas/frankenphp
    # descomente la siguiente línea si desea usar un Dockerfile personalizado
    #build: .
    # descomente la siguiente línea si desea ejecutar esto en un entorno de producción
    # restart: always
    ports:
      - "80:80" # HTTP
      - "443:443" # HTTPS
      - "443:443/udp" # HTTP/3
    volumes:
      - ./:/app/public
      - caddy_data:/data
      - caddy_config:/config
    # comente la siguiente línea en producción, permite tener logs legibles en desarrollo
    tty: true

# Volúmenes necesarios para los certificados y configuración de Caddy
volumes:
  caddy_data:
  caddy_config:
```

## Ejecutar como usuario no root

FrankenPHP puede ejecutarse como usuario no root en Docker.

Aquí hay un ejemplo de `Dockerfile` que hace esto:

```dockerfile
FROM dunglas/frankenphp

ARG USER=appuser

RUN \
	# Use "adduser -D ${USER}" para distribuciones basadas en alpine
	useradd ${USER}; \
	# Agregar capacidad adicional para enlazar a los puertos 80 y 443
	setcap CAP_NET_BIND_SERVICE=+eip /usr/local/bin/frankenphp; \
	# Dar acceso de escritura a /config/caddy y /data/caddy
	chown -R ${USER}:${USER} /config/caddy /data/caddy

USER ${USER}
```

### Ejecutar sin capacidades

Incluso cuando se ejecuta sin root, FrankenPHP necesita la capacidad `CAP_NET_BIND_SERVICE` para enlazar el servidor web en puertos privilegiados (80 y 443).

Si expone FrankenPHP en un puerto no privilegiado (1024 y superior), es posible ejecutar el servidor web como usuario no root, y sin necesidad de ninguna capacidad:

```dockerfile
FROM dunglas/frankenphp

ARG USER=appuser

RUN \
	# Use "adduser -D ${USER}" para distribuciones basadas en alpine
	useradd ${USER}; \
	# Eliminar la capacidad predeterminada
	setcap -r /usr/local/bin/frankenphp; \
	# Dar acceso de escritura a /config/caddy y /data/caddy
	chown -R ${USER}:${USER} /config/caddy /data/caddy

USER ${USER}
```

Luego, establezca la variable de entorno `SERVER_NAME` para usar un puerto no privilegiado.
Ejemplo: `:8000`

## Actualizaciones

Las imágenes Docker se construyen:

- Cuando se etiqueta una nueva versión
- Diariamente a las 4 am UTC, si hay nuevas versiones de las imágenes oficiales de PHP disponibles

## Endurecimiento de Imágenes

Para reducir aún más la superficie de ataque y el tamaño de tus imágenes Docker de FrankenPHP, también es posible construirlas sobre una imagen
[Google distroless](https://github.com/GoogleContainerTools/distroless) o
[Docker hardened](https://www.docker.com/products/hardened-images).

> [!WARNING]
> Estas imágenes base mínimas no incluyen un shell ni gestor de paquetes, lo que hace que la depuración sea más difícil.
> Por lo tanto, se recomiendan solo para producción si la seguridad es una alta prioridad.

Cuando agregues extensiones PHP adicionales, necesitarás una etapa de construcción intermedia:

```dockerfile
FROM dunglas/frankenphp AS builder

# Agregar extensiones PHP adicionales aquí
RUN install-php-extensions pdo_mysql pdo_pgsql #...

# Copiar bibliotecas compartidas de frankenphp y todas las extensiones instaladas a una ubicación temporal
# También puedes hacer este paso manualmente analizando la salida de ldd del binario frankenphp y cada archivo .so de extensión
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


# Imagen base distroless de Debian, asegúrate de que sea la misma versión de Debian que la imagen base
FROM gcr.io/distroless/base-debian13
# Alternativa de imagen endurecida de Docker
# FROM dhi.io/debian:13

# Ubicación de tu aplicación y Caddyfile que se copiará al contenedor
ARG PATH_TO_APP="."
ARG PATH_TO_CADDYFILE="./Caddyfile"

# Copiar tu aplicación en /app
# Para mayor endurecimiento asegúrate de que solo las rutas escribibles sean propiedad del usuario nonroot
COPY --chown=nonroot:nonroot "$PATH_TO_APP" /app
COPY "$PATH_TO_CADDYFILE" /etc/caddy/Caddyfile

# Copiar frankenphp y bibliotecas necesarias
COPY --from=builder /usr/local/bin/frankenphp /usr/local/bin/frankenphp
COPY --from=builder /usr/local/lib/php/extensions /usr/local/lib/php/extensions
COPY --from=builder /tmp/libs /usr/lib

# Copiar archivos de configuración php.ini
COPY --from=builder /usr/local/etc/php/conf.d /usr/local/etc/php/conf.d
COPY --from=builder /usr/local/etc/php/php.ini-production /usr/local/etc/php/php.ini

# Directorios de datos de Caddy — deben ser escribibles para nonroot, incluso en un sistema de archivos raíz de solo lectura
ENV XDG_CONFIG_HOME=/config \
    XDG_DATA_HOME=/data
COPY --from=builder --chown=nonroot:nonroot /data/caddy /data/caddy
COPY --from=builder --chown=nonroot:nonroot /config/caddy /config/caddy

USER nonroot

WORKDIR /app

# punto de entrada para ejecutar frankenphp con el Caddyfile proporcionado
ENTRYPOINT ["/usr/local/bin/frankenphp", "run", "-c", "/etc/caddy/Caddyfile"]
```

## Versiones de desarrollo

Las versiones de desarrollo están disponibles en el [repositorio Docker `dunglas/frankenphp-dev`](https://hub.docker.com/repository/docker/dunglas/frankenphp-dev).
Se activa una nueva compilación cada vez que se envía un commit a la rama principal del repositorio de GitHub.

Las etiquetas `latest*` apuntan a la cabeza de la rama `main`.
También están disponibles etiquetas de la forma `sha-<hash-del-commit-git>`.
