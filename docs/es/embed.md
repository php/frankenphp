# Aplicaciones PHP como Binarios Autónomos

FrankenPHP tiene la capacidad de incrustar el código fuente y los activos de aplicaciones PHP en un binario estático y autónomo.

Gracias a esta característica, las aplicaciones PHP pueden distribuirse como binarios autónomos que incluyen la aplicación en sí, el intérprete de PHP y Caddy, un servidor web de nivel de producción.

Obtenga más información sobre esta característica [en la presentación realizada por Kévin en SymfonyCon 2023](https://dunglas.dev/2023/12/php-and-symfony-apps-as-standalone-binaries/).

Para incrustar aplicaciones Laravel, [lea esta entrada específica de documentación](laravel.md#laravel-apps-as-standalone-binaries).

## Preparando su Aplicación

Antes de crear el binario autónomo, asegúrese de que su aplicación esté lista para ser incrustada.

Por ejemplo, probablemente querrá:

- Instalar las dependencias de producción de la aplicación
- Volcar el autoload
- Activar el modo de producción de su aplicación (si lo hay)
- Eliminar archivos innecesarios como `.git` o pruebas para reducir el tamaño de su binario final

Por ejemplo, para una aplicación Symfony, puede usar los siguientes comandos:

```console
# Exportar el proyecto para deshacerse de .git/, etc.
mkdir $TMPDIR/my-prepared-app
git archive HEAD | tar -x -C $TMPDIR/my-prepared-app
cd $TMPDIR/my-prepared-app

# Establecer las variables de entorno adecuadas
echo APP_ENV=prod > .env.local
echo APP_DEBUG=0 >> .env.local

# Eliminar las pruebas y otros archivos innecesarios para ahorrar espacio
# Alternativamente, agregue estos archivos con el atributo export-ignore en su archivo .gitattributes
rm -Rf tests/

# Instalar las dependencias
composer install --ignore-platform-reqs --no-dev -a

# Optimizar .env
composer dump-env prod
```

### Personalizar la Configuración

Para personalizar [la configuración](config.md), puede colocar un archivo `Caddyfile` así como un archivo `php.ini`
en el directorio principal de la aplicación a incrustar (`$TMPDIR/my-prepared-app` en el ejemplo anterior).

## Crear un Binario para Linux

La forma más fácil de crear un binario para Linux es usar el constructor basado en Docker que proporcionamos.

1. Cree un archivo llamado `static-build.Dockerfile` en el repositorio de su aplicación:

   ```dockerfile
   FROM --platform=linux/amd64 dunglas/frankenphp:static-builder-gnu
   # Si tiene la intención de ejecutar el binario en sistemas musl-libc, use static-builder-musl en su lugar

   # Copie su aplicación
   WORKDIR /go/src/app/dist/app
   COPY . .

   # Construya el binario estático
   WORKDIR /go/src/app/
   RUN EMBED=dist/app/ ./build-static.sh
   ```

    > [!CAUTION]
   >
   > Algunos archivos `.dockerignore` (por ejemplo, el [`.dockerignore` predeterminado de Symfony Docker](https://github.com/dunglas/symfony-docker/blob/main/.dockerignore))
   > ignorarán el directorio `vendor/` y los archivos `.env`. Asegúrese de ajustar o eliminar el archivo `.dockerignore` antes de la construcción.

2. Construya:

   ```console
   docker build -t static-app -f static-build.Dockerfile .
   ```

3. Extraiga el binario:

   ```console
   docker cp $(docker create --name static-app-tmp static-app):/go/src/app/dist/frankenphp-linux-x86_64 my-app ; docker rm static-app-tmp
   ```

El binario resultante es el archivo llamado `my-app` en el directorio actual.

## Crear un Binario para Otros Sistemas Operativos

Si no desea usar Docker o desea construir un binario para macOS, use el script de shell que proporcionamos:

```console
git clone https://github.com/php/frankenphp
cd frankenphp
EMBED=/path/to/your/app ./build-static.sh
```

El binario resultante es el archivo llamado `frankenphp-<os>-<arch>` en el directorio `dist/`.

## Usar el Binario

¡Listo! El archivo `my-app` (o `dist/frankenphp-<os>-<arch>` en otros sistemas operativos) contiene su aplicación autónoma.

Para iniciar la aplicación web, ejecute:

```console
./my-app php-server
```

Si su aplicación contiene un [script worker](worker.md), inicie el worker con algo como:

```console
./my-app php-server --worker public/index.php
```

Para habilitar HTTPS (se crea automáticamente un certificado de Let's Encrypt), HTTP/2 y HTTP/3, especifique el nombre de dominio a usar:

```console
./my-app php-server --domain localhost
```

También puede ejecutar los scripts CLI de PHP incrustados en su binario:

```console
./my-app php-cli bin/console
```

## Extensiones de PHP

Por defecto, el script construirá las extensiones requeridas por el archivo `composer.json` de su proyecto, si existe.
Si el archivo `composer.json` no existe, se construirán las extensiones predeterminadas, como se documenta en [la entrada de compilaciones estáticas](static.md).

Para personalizar las extensiones, use la variable de entorno `PHP_EXTENSIONS`.

## Personalizar la Compilación

[Lea la documentación de compilación estática](static.md) para ver cómo personalizar el binario (extensiones, versión de PHP, etc.).

## Distribuir el Binario

En Linux, el binario creado se comprime usando [UPX](https://upx.github.io).

En Mac, para reducir el tamaño del archivo antes de enviarlo, puede comprimirlo.
Recomendamos `xz`.
