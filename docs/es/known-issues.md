# Problemas Conocidos

## Extensiones PHP no Soportadas

Las siguientes extensiones se sabe que no son compatibles con FrankenPHP:

| Nombre                                                                                                        | Razón               | Alternativas                                                                                                         |
| ----------------------------------------------------------------------------------------------------------- | ------------------- | -------------------------------------------------------------------------------------------------------------------- |
| [imap](https://www.php.net/manual/es/imap.installation.php)                                                 | No es thread-safe   | [javanile/php-imap2](https://github.com/javanile/php-imap2), [webklex/php-imap](https://github.com/Webklex/php-imap) |
| [newrelic](https://docs.newrelic.com/docs/apm/agents/php-agent/getting-started/introduction-new-relic-php/) | No es thread-safe   | -                                                                                                                    |

## Extensiones PHP con Errores

Las siguientes extensiones tienen errores conocidos y comportamientos inesperados cuando se usan con FrankenPHP:

| Nombre                                                          | Problema                                                                                                                                                                                                                   |
| ------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [ext-openssl](https://www.php.net/manual/es/book.openssl.php) | Cuando se usa musl libc, la extensión OpenSSL puede fallar bajo cargas pesadas. El problema no ocurre cuando se usa la más popular GNU libc. Este error está [siendo rastreado por PHP](https://github.com/php/php-src/issues/13648). |

## get_browser

La función [get_browser()](https://www.php.net/manual/es/function.get-browser.php) parece funcionar mal después de un tiempo. Una solución es almacenar en caché (por ejemplo, con [APCu](https://www.php.net/manual/es/book.apcu.php)) los resultados por User Agent, ya que son estáticos.

## Binario Autónomo e Imágenes Docker Basadas en Alpine

Los binarios completamente estáticos y las imágenes Docker basadas en Alpine (`dunglas/frankenphp:*-alpine`) usan [musl libc](https://musl.libc.org/) en lugar de [glibc](https://www.etalabs.net/compare_libcs.html), para mantener un tamaño de binario más pequeño.
Esto puede llevar a algunos problemas de compatibilidad.
En particular, la bandera glob `GLOB_BRACE` [no está disponible](https://www.php.net/manual/es/function.glob.php).

Se recomienda usar la variante GNU del binario estático y las imágenes Docker basadas en Debian si encuentras problemas.

## Usar `https://127.0.0.1` con Docker

Por defecto, FrankenPHP genera un certificado TLS para `localhost`.
Es la opción más fácil y recomendada para el desarrollo local.

Si realmente deseas usar `127.0.0.1` como host en su lugar, es posible configurarlo para generar un certificado para él estableciendo el nombre del servidor en `127.0.0.1`.

Desafortunadamente, esto no es suficiente al usar Docker debido a [su sistema de red](https://docs.docker.com/network/).
Obtendrás un error TLS similar a `curl: (35) LibreSSL/3.3.6: error:1404B438:SSL routines:ST_CONNECT:tlsv1 alert internal error`.

Si estás usando Linux, una solución es usar [el controlador de red host](https://docs.docker.com/network/network-tutorial-host/):

```console
docker run \
    -e SERVER_NAME="127.0.0.1" \
    -v $PWD:/app/public \
    --network host \
    dunglas/frankenphp
```

El controlador de red host no está soportado en Mac y Windows. En estas plataformas, tendrás que adivinar la dirección IP del contenedor e incluirla en los nombres del servidor.

Ejecuta `docker network inspect bridge` y busca la clave `Containers` para identificar la última dirección IP actualmente asignada bajo la clave `IPv4Address`, y incrementa en uno. Si no hay contenedores en ejecución, la primera dirección IP asignada suele ser `172.17.0.2`.

Luego, incluye esto en la variable de entorno `SERVER_NAME`:

```console
docker run \
    -e SERVER_NAME="127.0.0.1, 172.17.0.3" \
    -v $PWD:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

> [!CAUTION]
>
> Asegúrate de reemplazar `172.17.0.3` con la IP que se asignará a tu contenedor.

Ahora deberías poder acceder a `https://127.0.0.1` desde la máquina host.

Si no es así, inicia FrankenPHP en modo depuración para intentar identificar el problema:

```console
docker run \
    -e CADDY_GLOBAL_OPTIONS="debug" \
    -e SERVER_NAME="127.0.0.1" \
    -v $PWD:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

## Scripts de Composer que Referencian `@php`

Los [scripts de Composer](https://getcomposer.org/doc/articles/scripts.md) pueden querer ejecutar un binario PHP para algunas tareas, por ejemplo, en [un proyecto Laravel](laravel.md) para ejecutar `@php artisan package:discover --ansi`. Esto [actualmente falla](https://github.com/php/frankenphp/issues/483#issuecomment-1899890915) por dos razones:

- Composer no sabe cómo llamar al binario de FrankenPHP;
- Composer puede agregar configuraciones de PHP usando la bandera `-d` en el comando, que FrankenPHP aún no soporta.

Como solución alternativa, podemos crear un script de shell en `/usr/local/bin/php` que elimine los parámetros no soportados y luego llame a FrankenPHP:

```bash
#!/usr/bin/env bash
args=("$@")
index=0
for i in "$@"
do
    if [ "$i" == "-d" ]; then
        unset 'args[$index]'
        unset 'args[$index+1]'
    fi
    index=$((index+1))
done

/usr/local/bin/frankenphp php-cli ${args[@]}
```

Luego, establece la variable de entorno `PHP_BINARY` a la ruta de nuestro script `php` y ejecuta Composer:

```console
export PHP_BINARY=/usr/local/bin/php
composer install
```

## Solución de Problemas de TLS/SSL con Binarios Estáticos

Al usar los binarios estáticos, puedes encontrar los siguientes errores relacionados con TLS, por ejemplo, al enviar correos electrónicos usando STARTTLS:

```text
No se puede conectar con STARTTLS: stream_socket_enable_crypto(): La operación SSL falló con el código 5. Mensajes de error de OpenSSL:
error:80000002:librería del sistema::No existe el archivo o el directorio
error:80000002:librería del sistema::No existe el archivo o el directorio
error:80000002:librería del sistema::No existe el archivo o el directorio
error:0A000086:rutinas de SSL::falló la verificación del certificado
```

Dado que el binario estático no incluye certificados TLS, necesitas indicar a OpenSSL la ubicación de tu instalación local de certificados CA.

Inspecciona la salida de [`openssl_get_cert_locations()`](https://www.php.net/manual/es/function.openssl-get-cert-locations.php),
para encontrar dónde deben instalarse los certificados CA y guárdalos en esa ubicación.

> [!CAUTION]
>
> Los contextos web y CLI pueden tener configuraciones diferentes.
> Asegúrate de ejecutar `openssl_get_cert_locations()` en el contexto adecuado.

[Los certificados CA extraídos de Mozilla pueden descargarse del sitio de cURL](https://curl.se/docs/caextract.html).

Alternativamente, muchas distribuciones, incluyendo Debian, Ubuntu y Alpine, proporcionan paquetes llamados `ca-certificates` que contienen estos certificados.

También es posible usar `SSL_CERT_FILE` y `SSL_CERT_DIR` para indicar a OpenSSL dónde buscar los certificados CA:

```console
# Establecer variables de entorno de certificados TLS
export SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
export SSL_CERT_DIR=/etc/ssl/certs
```
