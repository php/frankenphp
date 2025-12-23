# Laravel

## Docker

Servir una aplicación web [Laravel](https://laravel.com) con FrankenPHP es tan fácil como montar el proyecto en el directorio `/app` de la imagen Docker oficial.

Ejecuta este comando desde el directorio principal de tu aplicación Laravel:

```console
docker run -p 80:80 -p 443:443 -p 443:443/udp -v $PWD:/app dunglas/frankenphp
```

¡Y listo!

## Instalación Local

Alternativamente, puedes ejecutar tus proyectos Laravel con FrankenPHP desde tu máquina local:

1. [Descarga el binario correspondiente a tu sistema](../#binario-autónomo)
2. Agrega la siguiente configuración a un archivo llamado `Caddyfile` en el directorio raíz de tu proyecto Laravel:

   ```caddyfile
   {
   	frankenphp
   }

   # El nombre de dominio de tu servidor
   localhost {
   	# Establece el directorio web raíz en public/
   	root public/
   	# Habilita la compresión (opcional)
   	encode zstd br gzip
   	# Ejecuta archivos PHP desde el directorio public/ y sirve los assets
   	php_server {
   		try_files {path} index.php
   	}
   }
   ```

3. Inicia FrankenPHP desde el directorio raíz de tu proyecto Laravel: `frankenphp run`

## Laravel Octane

Octane se puede instalar a través del gestor de paquetes Composer:

```console
composer require laravel/octane
```

Después de instalar Octane, puedes ejecutar el comando Artisan `octane:install`, que instalará el archivo de configuración de Octane en tu aplicación:

```console
php artisan octane:install --server=frankenphp
```

El servidor Octane se puede iniciar mediante el comando Artisan `octane:frankenphp`.

```console
php artisan octane:frankenphp
```

El comando `octane:frankenphp` puede tomar las siguientes opciones:

- `--host`: La dirección IP a la que el servidor debe enlazarse (por defecto: `127.0.0.1`)
- `--port`: El puerto en el que el servidor debe estar disponible (por defecto: `8000`)
- `--admin-port`: El puerto en el que el servidor de administración debe estar disponible (por defecto: `2019`)
- `--workers`: El número de workers que deben estar disponibles para manejar solicitudes (por defecto: `auto`)
- `--max-requests`: El número de solicitudes a procesar antes de recargar el servidor (por defecto: `500`)
- `--caddyfile`: La ruta al archivo `Caddyfile` de FrankenPHP (por defecto: [Caddyfile de plantilla en Laravel Octane](https://github.com/laravel/octane/blob/2.x/src/Commands/stubs/Caddyfile))
- `--https`: Habilita HTTPS, HTTP/2 y HTTP/3, y genera y renueva certificados automáticamente
- `--http-redirect`: Habilita la redirección de HTTP a HTTPS (solo se habilita si se pasa --https)
- `--watch`: Recarga automáticamente el servidor cuando se modifica la aplicación
- `--poll`: Usa la sondea del sistema de archivos mientras se observa para vigilar archivos a través de una red
- `--log-level`: Registra mensajes en o por encima del nivel de registro especificado, usando el registrador nativo de Caddy

> [!TIP]
> Para obtener registros JSON estructurados (útil al usar soluciones de análisis de registros), pasa explícitamente la opción `--log-level`.

Consulta también [cómo usar Mercure con Octane](#soporte-para-mercure).

Aprende más sobre [Laravel Octane en su documentación oficial](https://laravel.com/docs/octane).

## Aplicaciones Laravel como Binarios Autónomos

Usando [la característica de incrustación de aplicaciones de FrankenPHP](embed.md), es posible distribuir aplicaciones Laravel
como binarios autónomos.

Sigue estos pasos para empaquetar tu aplicación Laravel como un binario autónomo para Linux:

1. Crea un archivo llamado `static-build.Dockerfile` en el repositorio de tu aplicación:

   ```dockerfile
   FROM --platform=linux/amd64 dunglas/frankenphp:static-builder-gnu
   # Si tienes intención de ejecutar el binario en sistemas musl-libc, usa static-builder-musl en su lugar

   # Copia tu aplicación
   WORKDIR /go/src/app/dist/app
   COPY . .

   # Elimina las pruebas y otros archivos innecesarios para ahorrar espacio
   # Alternativamente, agrega estos archivos a un archivo .dockerignore
   RUN rm -Rf tests/

   # Copia el archivo .env
   RUN cp .env.example .env
   # Cambia APP_ENV y APP_DEBUG para que estén listos para producción
   RUN sed -i'' -e 's/^APP_ENV=.*/APP_ENV=production/' -e 's/^APP_DEBUG=.*/APP_DEBUG=false/' .env

   # Realiza otros cambios en tu archivo .env si es necesario

   # Instala las dependencias
   RUN composer install --ignore-platform-reqs --no-dev -a

   # Compila el binario estático
   WORKDIR /go/src/app/
   RUN EMBED=dist/app/ ./build-static.sh
   ```

   > [!CAUTION]
   >
   > Algunos archivos `.dockerignore`
   > ignorarán el directorio `vendor/` y los archivos `.env`. Asegúrate de ajustar o eliminar el archivo `.dockerignore` antes de la compilación.

2. Compila:

   ```console
   docker build -t static-laravel-app -f static-build.Dockerfile .
   ```

3. Extrae el binario:

   ```console
   docker cp $(docker create --name static-laravel-app-tmp static-laravel-app):/go/src/app/dist/frankenphp-linux-x86_64 frankenphp ; docker rm static-laravel-app-tmp
   ```

4. Rellena las cachés:

   ```console
   frankenphp php-cli artisan optimize
   ```

5. Ejecuta las migraciones de la base de datos (si las hay):

   ```console
   frankenphp php-cli artisan migrate
   ```

6. Genera la clave secreta de la aplicación:

   ```console
   frankenphp php-cli artisan key:generate
   ```

7. Inicia el servidor:

   ```console
   frankenphp php-server
   ```

¡Tu aplicación ya está lista!

Aprende más sobre las opciones disponibles y cómo compilar binarios para otros sistemas operativos en la documentación de [incrustación de aplicaciones](embed.md).

### Cambiar la Ruta de Almacenamiento

Por defecto, Laravel almacena los archivos subidos, cachés, registros, etc. en el directorio `storage/` de la aplicación.
Esto no es adecuado para aplicaciones incrustadas, ya que cada nueva versión se extraerá en un directorio temporal diferente.

Establece la variable de entorno `LARAVEL_STORAGE_PATH` (por ejemplo, en tu archivo `.env`) o llama al método `Illuminate\Foundation\Application::useStoragePath()` para usar un directorio fuera del directorio temporal.

### Soporte para Mercure

[Mercure](https://mercure.rocks) es una excelente manera de agregar capacidades en tiempo real a tus aplicaciones Laravel.
FrankenPHP incluye [soporte para Mercure integrado](mercure.md).

Si no estás usando [Octane](#laravel-octane), consulta [la entrada de documentación de Mercure](mercure.md).

Si estás usando Octane, puedes habilitar el soporte para Mercure agregando las siguientes líneas a tu archivo `config/octane.php`:

```php
// ...

return [
    // ...

    'mercure' => [
        'anonymous' => true,
        'publisher_jwt' => '!CambiaEstaClaveSecretaJWTDelHubMercure!',
        'subscriber_jwt' => '!CambiaEstaClaveSecretaJWTDelHubMercure!',
    ],
];
```

Puedes usar [todas las directivas soportadas por Mercure](https://mercure.rocks/docs/hub/config#directives) en este array.

Para publicar y suscribirte a actualizaciones, recomendamos usar la biblioteca [Laravel Mercure Broadcaster](https://github.com/mvanduijker/laravel-mercure-broadcaster).
Alternativamente, consulta [la documentación de Mercure](mercure.md) para hacerlo en PHP y JavaScript puros.

### Ejecutar Octane con Binarios Autónomos

¡Incluso es posible empaquetar aplicaciones Laravel Octane como binarios autónomos!

Para hacerlo, [instala Octane correctamente](#laravel-octane) y sigue los pasos descritos en [la sección anterior](#laravel-apps-as-standalone-binaries).

Luego, para iniciar FrankenPHP en modo worker a través de Octane, ejecuta:

```console
PATH="$PWD:$PATH" frankenphp php-cli artisan octane:frankenphp
```

> [!CAUTION]
>
> Para que el comando funcione, el binario autónomo **debe** llamarse `frankenphp`
> porque Octane necesita un programa llamado `frankenphp` disponible en la ruta.
