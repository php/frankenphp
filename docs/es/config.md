# Configuración

FrankenPHP, Caddy así como los módulos [Mercure](mercure.md) y [Vulcain](https://vulcain.rocks) pueden configurarse usando [los formatos soportados por Caddy](https://caddyserver.com/docs/getting-started#your-first-config).

El formato más común es el `Caddyfile`, que es un formato de texto simple y legible.
Por defecto, FrankenPHP buscará un `Caddyfile` en el directorio actual.
Puede especificar una ruta personalizada con la opción `-c` o `--config`.

Un `Caddyfile` mínimo para servir una aplicación PHP se muestra a continuación:

```caddyfile
# El nombre de host al que responder
localhost

# Opcionalmente, el directorio desde el que servir archivos, por defecto es el directorio actual
#root public/
php_server
```

Un `Caddyfile` más avanzado que habilita más características y proporciona variables de entorno convenientes está disponible [en el repositorio de FrankenPHP](https://github.com/php/frankenphp/blob/main/caddy/frankenphp/Caddyfile),
y con las imágenes de Docker.

PHP en sí puede configurarse [usando un archivo `php.ini`](https://www.php.net/manual/es/configuration.file.php).

Dependiendo de su método de instalación, FrankenPHP y el intérprete de PHP buscarán archivos de configuración en las ubicaciones descritas a continuación.

## Docker

FrankenPHP:

- `/etc/frankenphp/Caddyfile`: el archivo de configuración principal
- `/etc/frankenphp/Caddyfile.d/*.caddyfile`: archivos de configuración adicionales que se cargan automáticamente

PHP:

- `php.ini`: `/usr/local/etc/php/php.ini` (no se proporciona ningún `php.ini` por defecto)
- archivos de configuración adicionales: `/usr/local/etc/php/conf.d/*.ini`
- extensiones de PHP: `/usr/local/lib/php/extensions/no-debug-zts-<YYYYMMDD>/`
- Debe copiar una plantilla oficial proporcionada por el proyecto PHP:

```dockerfile
FROM dunglas/frankenphp

# Producción:
RUN cp $PHP_INI_DIR/php.ini-production $PHP_INI_DIR/php.ini

# O desarrollo:
RUN cp $PHP_INI_DIR/php.ini-development $PHP_INI_DIR/php.ini
```

## Paquetes RPM y Debian

FrankenPHP:

- `/etc/frankenphp/Caddyfile`: el archivo de configuración principal
- `/etc/frankenphp/Caddyfile.d/*.caddyfile`: archivos de configuración adicionales que se cargan automáticamente

PHP:

- `php.ini`: `/etc/php-zts/php.ini` (se proporciona un archivo `php.ini` con ajustes de producción por defecto)
- archivos de configuración adicionales: `/etc/php-zts/conf.d/*.ini`

## Binario estático

FrankenPHP:

- En el directorio de trabajo actual: `Caddyfile`

PHP:

- `php.ini`: El directorio en el que se ejecuta `frankenphp run` o `frankenphp php-server`, luego `/etc/frankenphp/php.ini`
- archivos de configuración adicionales: `/etc/frankenphp/php.d/*.ini`
- extensiones de PHP: no pueden cargarse, debe incluirlas en el binario mismo
- copie uno de `php.ini-production` o `php.ini-development` proporcionados [en las fuentes de PHP](https://github.com/php/php-src/).

## Configuración de Caddyfile

Las directivas `php_server` o `php` [de HTTP](https://caddyserver.com/docs/caddyfile/concepts#directives) pueden usarse dentro de los bloques de sitio para servir su aplicación PHP.

Ejemplo mínimo:

```caddyfile
localhost {
	# Habilitar compresión (opcional)
	encode zstd br gzip
	# Ejecutar archivos PHP en el directorio actual y servir activos
	php_server
}
```

También puede configurar explícitamente FrankenPHP usando la [opción global](https://caddyserver.com/docs/caddyfile/concepts#global-options) `frankenphp`:

```caddyfile
{
	frankenphp {
		num_threads <num_threads> # Establece el número de hilos de PHP para iniciar. Por defecto: 2x el número de CPUs disponibles.
		max_threads <num_threads> # Limita el número de hilos de PHP adicionales que pueden iniciarse en tiempo de ejecución. Por defecto: num_threads. Puede establecerse como 'auto'.
		max_wait_time <duration> # Establece el tiempo máximo que una solicitud puede esperar por un hilo de PHP libre antes de agotar el tiempo de espera. Por defecto: deshabilitado.
		php_ini <key> <value> # Establece una directiva php.ini. Puede usarse varias veces para establecer múltiples directivas.
		worker {
			file <path> # Establece la ruta al script del worker.
			num <num> # Establece el número de hilos de PHP para iniciar, por defecto es 2x el número de CPUs disponibles.
			env <key> <value> # Establece una variable de entorno adicional con el valor dado. Puede especificarse más de una vez para múltiples variables de entorno.
			watch <path> # Establece la ruta para observar cambios en archivos. Puede especificarse más de una vez para múltiples rutas.
			name <name> # Establece el nombre del worker, usado en logs y métricas. Por defecto: ruta absoluta del archivo del worker
			max_consecutive_failures <num> # Establece el número máximo de fallos consecutivos antes de que el worker se considere no saludable, -1 significa que el worker siempre se reiniciará. Por defecto: 6.
		}
	}
}

# ...
```

Alternativamente, puede usar la forma corta de una línea de la opción `worker`:

```caddyfile
{
	frankenphp {
		worker <file> <num>
	}
}

# ...
```

También puede definir múltiples workers si sirve múltiples aplicaciones en el mismo servidor:

```caddyfile
app.example.com {
    root /path/to/app/public
	php_server {
		root /path/to/app/public # permite un mejor almacenamiento en caché
		worker index.php <num>
	}
}

other.example.com {
    root /path/to/other/public
	php_server {
		root /path/to/other/public
		worker index.php <num>
	}
}

# ...
```

Usar la directiva `php_server` es generalmente lo que necesita,
pero si necesita un control total, puede usar la directiva de bajo nivel `php`.
La directiva `php` pasa toda la entrada a PHP, en lugar de verificar primero si
es un archivo PHP o no. Lea más sobre esto en la [página de rendimiento](performance.md#try_files).

Usar la directiva `php_server` es equivalente a esta configuración:

```caddyfile
route {
	# Agrega barra final para solicitudes de directorio
	@canonicalPath {
		file {path}/index.php
		not path */
	}
	redir @canonicalPath {path}/ 308
	# Si el archivo solicitado no existe, intenta archivos índice
	@indexFiles file {
		try_files {path} {path}/index.php index.php
		split_path .php
	}
	rewrite @indexFiles {http.matchers.file.relative}
	# ¡FrankenPHP!
	@phpFiles path *.php
	php @phpFiles
	file_server
}
```

Las directivas `php_server` y `php` tienen las siguientes opciones:

```caddyfile
php_server [<matcher>] {
	root <directory> # Establece la carpeta raíz del sitio. Por defecto: directiva `root`.
	split_path <delim...> # Establece las subcadenas para dividir la URI en dos partes. La primera subcadena coincidente se usará para dividir la "información de ruta" del path. La primera parte se sufija con la subcadena coincidente y se asumirá como el nombre del recurso real (script CGI). La segunda parte se establecerá como PATH_INFO para que el script la use. Por defecto: `.php`
	resolve_root_symlink false # Desactiva la resolución del directorio `root` a su valor real evaluando un enlace simbólico, si existe (habilitado por defecto).
	env <key> <value> # Establece una variable de entorno adicional con el valor dado. Puede especificarse más de una vez para múltiples variables de entorno.
	file_server off # Desactiva la directiva incorporada file_server.
	worker { # Crea un worker específico para este servidor. Puede especificarse más de una vez para múltiples workers.
		file <path> # Establece la ruta al script del worker, puede ser relativa a la raíz de php_server
		num <num> # Establece el número de hilos de PHP para iniciar, por defecto es 2x el número de CPUs disponibles.
		name <name> # Establece el nombre para el worker, usado en logs y métricas. Por defecto: ruta absoluta del archivo del worker. Siempre comienza con m# cuando se define en un bloque php_server.
		watch <path> # Establece la ruta para observar cambios en archivos. Puede especificarse más de una vez para múltiples rutas.
		env <key> <value> # Establece una variable de entorno adicional con el valor dado. Puede especificarse más de una vez para múltiples variables de entorno. Las variables de entorno para este worker también se heredan del php_server padre, pero pueden sobrescribirse aquí.
		match <path> # hace coincidir el worker con un patrón de ruta. Anula try_files y solo puede usarse en la directiva php_server.
	}
	worker <other_file> <num> # También puede usar la forma corta como en el bloque global frankenphp.
}
```

### Observando cambios en archivos

Dado que los workers solo inician su aplicación una vez y la mantienen en memoria, cualquier cambio
en sus archivos PHP no se reflejará inmediatamente.

Los workers pueden reiniciarse al cambiar archivos mediante la directiva `watch`.
Esto es útil para entornos de desarrollo.

```caddyfile
{
	frankenphp {
		worker {
			file  /path/to/app/public/worker.php
			watch
		}
	}
}
```

Esta función se utiliza frecuentemente en combinación con [hot reload](hot-reload.md).

Si el directorio `watch` no está especificado, retrocederá a `./**/*.{env,php,twig,yaml,yml}`,
lo cual vigila todos los archivos `.env`, `.php`, `.twig`, `.yaml` y `.yml` en el directorio y subdirectorios
donde se inició el proceso de FrankenPHP. También puede especificar uno o más directorios mediante un
[patrón de nombres de ficheros de shell](https://pkg.go.dev/path/filepath#Match):

```caddyfile
{
	frankenphp {
		worker {
			file  /path/to/app/public/worker.php
			watch /path/to/app # observa todos los archivos en todos los subdirectorios de /path/to/app
			watch /path/to/app/*.php # observa archivos que terminan en .php en /path/to/app
			watch /path/to/app/**/*.php # observa archivos PHP en /path/to/app y subdirectorios
			watch /path/to/app/**/*.{php,twig} # observa archivos PHP y Twig en /path/to/app y subdirectorios
		}
	}
}
```

- El patrón `**` significa observación recursiva
- Los directorios también pueden ser relativos (al lugar donde se inicia el proceso de FrankenPHP)
- Si tiene múltiples workers definidos, todos ellos se reiniciarán cuando cambie un archivo
- Tenga cuidado al observar archivos que se crean en tiempo de ejecución (como logs) ya que podrían causar reinicios no deseados de workers.

El observador de archivos se basa en [e-dant/watcher](https://github.com/e-dant/watcher).

## Coincidencia del worker con una ruta

En aplicaciones PHP tradicionales, los scripts siempre se colocan en el directorio público.
Esto también es cierto para los scripts de workers, que se tratan como cualquier otro script PHP.
Si desea colocar el script del worker fuera del directorio público, puede hacerlo mediante la directiva `match`.

La directiva `match` es una alternativa optimizada a `try_files` solo disponible dentro de `php_server` y `php`.
El siguiente ejemplo siempre servirá un archivo en el directorio público si está presente
y de lo contrario reenviará la solicitud al worker que coincida con el patrón de ruta.

```caddyfile
{
	frankenphp {
		php_server {
			worker {
				file /path/to/worker.php # el archivo puede estar fuera de la ruta pública
				match /api/* # todas las solicitudes que comiencen con /api/ serán manejadas por este worker
			}
		}
	}
}
```

## Variables de entorno

Las siguientes variables de entorno pueden usarse para inyectar directivas de Caddy en el `Caddyfile` sin modificarlo:

- `SERVER_NAME`: cambia [las direcciones en las que escuchar](https://caddyserver.com/docs/caddyfile/concepts#addresses), los nombres de host proporcionados también se usarán para el certificado TLS generado
- `SERVER_ROOT`: cambia el directorio raíz del sitio, por defecto es `public/`
- `CADDY_GLOBAL_OPTIONS`: inyecta [opciones globales](https://caddyserver.com/docs/caddyfile/options)
- `FRANKENPHP_CONFIG`: inyecta configuración bajo la directiva `frankenphp`

Al igual que en FPM y SAPIs CLI, las variables de entorno se exponen por defecto en la superglobal `$_SERVER`.

El valor `S` de [la directiva `variables_order` de PHP](https://www.php.net/manual/en/ini.core.php#ini.variables-order) siempre es equivalente a `ES` independientemente de la ubicación de `E` en otro lugar de esta directiva.

## Configuración de PHP

Para cargar [archivos de configuración adicionales de PHP](https://www.php.net/manual/en/configuration.file.php#configuration.file.scan),
puede usarse la variable de entorno `PHP_INI_SCAN_DIR`.
Cuando se establece, PHP cargará todos los archivos con la extensión `.ini` presentes en los directorios dados.

También puede cambiar la configuración de PHP usando la directiva `php_ini` en el `Caddyfile`:

```caddyfile
{
    frankenphp {
        php_ini memory_limit 256M

        # o

        php_ini {
            memory_limit 256M
            max_execution_time 15
        }
    }
}
```

### Deshabilitar HTTPS

Por defecto, FrankenPHP habilitará automáticamente HTTPS para todos los nombres de host, incluyendo `localhost`.
Si desea deshabilitar HTTPS (por ejemplo en un entorno de desarrollo), puede establecer la variable de entorno `SERVER_NAME` a `http://` o `:80`:

Alternativamente, puede usar todos los otros métodos descritos en la [documentación de Caddy](https://caddyserver.com/docs/automatic-https#activation).

Si desea usar HTTPS con la dirección IP `127.0.0.1` en lugar del nombre de host `localhost`, lea la sección de [problemas conocidos](known-issues.md#using-https127001-with-docker).

### Dúplex completo (HTTP/1)

Al usar HTTP/1.x, puede ser deseable habilitar el modo dúplex completo para permitir escribir una respuesta antes de que se haya leído todo el cuerpo
(por ejemplo: [Mercure](mercure.md), WebSocket, Eventos enviados por el servidor, etc.).

Esta es una configuración opcional que debe agregarse a las opciones globales en el `Caddyfile`:

```caddyfile
{
  servers {
    enable_full_duplex
  }
}
```

> [!CAUTION]
>
> Habilitar esta opción puede causar que clientes HTTP/1.x antiguos que no soportan dúplex completo se bloqueen.
> Esto también puede configurarse usando la configuración de entorno `CADDY_GLOBAL_OPTIONS`:

```sh
CADDY_GLOBAL_OPTIONS="servers {
  enable_full_duplex
}"
```

Puede encontrar más información sobre esta configuración en la [documentación de Caddy](https://caddyserver.com/docs/caddyfile/options#enable-full-duplex).

## Habilitar el modo de depuración

Al usar la imagen de Docker, establezca la variable de entorno `CADDY_GLOBAL_OPTIONS` a `debug` para habilitar el modo de depuración:

```console
docker run -v $PWD:/app/public \
    -e CADDY_GLOBAL_OPTIONS=debug \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```
