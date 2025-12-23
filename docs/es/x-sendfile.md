# Sirviendo archivos estáticos grandes de manera eficiente (`X-Sendfile`/`X-Accel-Redirect`)

Normalmente, los archivos estáticos pueden ser servidos directamente por el servidor web,
pero a veces es necesario ejecutar código PHP antes de enviarlos:
control de acceso, estadísticas, encabezados HTTP personalizados...

Desafortunadamente, usar PHP para servir archivos estáticos grandes es ineficiente en comparación con
el uso directo del servidor web (sobrecarga de memoria, rendimiento reducido...).

FrankenPHP permite delegar el envío de archivos estáticos al servidor web
**después** de ejecutar código PHP personalizado.

Para hacerlo, tu aplicación PHP simplemente necesita definir un encabezado HTTP personalizado
que contenga la ruta del archivo a servir. FrankenPHP se encarga del resto.

Esta funcionalidad es conocida como **`X-Sendfile`** para Apache y **`X-Accel-Redirect`** para NGINX.

En los siguientes ejemplos, asumimos que el directorio raíz del proyecto es `public/`
y que queremos usar PHP para servir archivos almacenados fuera del directorio `public/`,
desde un directorio llamado `private-files/`.

## Configuración

Primero, agrega la siguiente configuración a tu `Caddyfile` para habilitar esta funcionalidad:

```patch
	root public/
	# ...

+	# Necesario para Symfony, Laravel y otros proyectos que usan el componente Symfony HttpFoundation
+	request_header X-Sendfile-Type x-accel-redirect
+	request_header X-Accel-Mapping ../private-files=/private-files
+
+	intercept {
+		@accel header X-Accel-Redirect *
+		handle_response @accel {
+			root private-files/
+			rewrite * {resp.header.X-Accel-Redirect}
+			method * GET
+
+			# Elimina el encabezado X-Accel-Redirect establecido por PHP para mayor seguridad
+			header -X-Accel-Redirect
+
+			file_server
+		}
+	}

	php_server
```

## PHP puro

Establece la ruta relativa del archivo (desde `private-files/`) como valor del encabezado `X-Accel-Redirect`:

```php
header('X-Accel-Redirect: file.txt');
```

## Proyectos que usan el componente Symfony HttpFoundation (Symfony, Laravel, Drupal...)

Symfony HttpFoundation [soporta nativamente esta funcionalidad](https://symfony.com/doc/current/components/http_foundation.html#serving-files).
Determinará automáticamente el valor correcto para el encabezado `X-Accel-Redirect` y lo agregará a la respuesta.

```php
use Symfony\Component\HttpFoundation\BinaryFileResponse;

BinaryFileResponse::trustXSendfileTypeHeader();
$response = new BinaryFileResponse(__DIR__.'/../private-files/file.txt');

// ...
```
