# Hot reload

FrankenPHP incluye una función de **hot reload** integrada diseñada para mejorar significativamente la experiencia del desarrollador.

![Mercure](../hot-reload.png)

Esta función proporciona un flujo de trabajo similar a **Hot Module Replacement (HMR)** encontrado en herramientas modernas de JavaScript (como Vite o webpack).
En lugar de actualizar manualmente el navegador después de cada cambio de archivo (código PHP, plantillas, archivos JavaScript y CSS...),
FrankenPHP actualiza el contenido en tiempo real.

La Hot Reload funciona de forma nativa con WordPress, Laravel, Symfony y cualquier otra aplicación o framework PHP.

Cuando está activada, FrankenPHP vigila el directorio de trabajo actual en busca de cambios en el sistema de archivos.
Cuando se modifica un archivo, envía una actualización [Mercure](mercure.md) al navegador.

Dependiendo de la configuración, el navegador:

- **Transformará el DOM** (preservando la posición de desplazamiento y el estado de los inputs) si [Idiomorph](https://github.com/bigskysoftware/idiomorph) está cargado.
- **Recargará la página** (recarga en vivo estándar) si Idiomorph no está presente.

## Configuración

Para habilitar la Hot Reload, active Mercure y luego agregue la subdirectiva `hot_reload` a la directiva `php_server` en su `Caddyfile`.

> [!WARNING]
> Esta función está destinada **únicamente a entornos de desarrollo**.
> No active `hot_reload` en producción, ya que vigilar el sistema de archivos implica una sobrecarga de rendimiento y expone endpoints internos.

```caddyfile
localhost

mercure {
    anonymous
}

root public/
php_server {
    hot_reload
}
```

Por omisión, FrankenPHP vigilará todos los archivos en el directorio de trabajo actual que coincidan con este patrón glob: `./**/*.{css,env,gif,htm,html,jpg,jpeg,js,mjs,php,png,svg,twig,webp,xml,yaml,yml}`

Es posible establecer explícitamente los archivos a vigilar usando la sintaxis glob:

```caddyfile
localhost

mercure {
    anonymous
}

root public/
php_server {
    hot_reload src/**/*{.php,.js} config/**/*.yaml
}
```

Use la forma larga para especificar el tema de Mercure a utilizar, así como qué directorios o archivos vigilar, proporcionando rutas a la opción `hot_reload`:

```caddyfile
localhost

mercure {
    anonymous
}

root public/
php_server {
    hot_reload {
        topic hot-reload-topic
        watch src/**/*.php
        watch assets/**/*.{ts,json}
        watch templates/
        watch public/css/
    }
}
```

## Integración Lado-Cliente

Mientras el servidor detecta los cambios, el navegador necesita suscribirse a estos eventos para actualizar la página.
FrankenPHP expone la URL del Mercure Hub a utilizar para suscribirse a los cambios de archivos a través de la variable de entorno `$_SERVER['FRANKENPHP_HOT_RELOAD']`.

Una biblioteca JavaScript de conveniencia, [frankenphp-hot-reload](https://www.npmjs.com/package/frankenphp-hot-reload), también está disponible para manejar la lógica lado-cliente.
Para usarla, agregue lo siguiente a su diseño principal:

```php
<!DOCTYPE html>
<title>FrankenPHP Hot Reload</title>
<?php if (isset($_SERVER['FRANKENPHP_HOT_RELOAD'])): ?>
<meta name="frankenphp-hot-reload:url" content="<?=$_SERVER['FRANKENPHP_HOT_RELOAD']?>">
<script src="https://cdn.jsdelivr.net/npm/idiomorph"></script>
<script src="https://cdn.jsdelivr.net/npm/frankenphp-hot-reload/+esm" type="module"></script>
<?php endif ?>
```

La biblioteca se suscribirá automáticamente al hub de Mercure, obtendrá la URL actual en segundo plano cuando se detecte un cambio en un archivo y transformará el DOM.
Está disponible como un paquete [npm](https://www.npmjs.com/package/frankenphp-hot-reload) y en [GitHub](https://github.com/dunglas/frankenphp-hot-reload).

Alternativamente, puede implementar su propia lógica lado-cliente suscribiéndose directamente al hub de Mercure usando la clase nativa de JavaScript `EventSource`.

### Modo Worker

Si está ejecutando su aplicación en [Modo Worker](https://frankenphp.dev/docs/worker/), el script de su aplicación permanece en memoria.
Esto significa que los cambios en su código PHP no se reflejarán inmediatamente, incluso si el navegador se recarga.

Para la mejor experiencia de desarrollador, debe combinar `hot_reload` con [la subdirectiva `watch` en la directiva `worker`](config.md#watching-for-file-changes).

- `hot_reload`: actualiza el **navegador** cuando los archivos cambian
- `worker.watch`: reinicia el worker cuando los archivos cambian

```caddy
localhost

mercure {
    anonymous
}

root public/
php_server {
    hot_reload
    worker {
        file /path/to/my_worker.php
        watch
    }
}
```

### Funcionamiento

1. **Vigilancia**: FrankenPHP monitorea el sistema de archivos en busca de modificaciones usando [la biblioteca `e-dant/watcher`](https://github.com/e-dant/watcher) internamente (contribuimos con el binding de Go).
2. **Reinicio (Modo Worker)**: si `watch` está habilitado en la configuración del worker, el worker de PHP se reinicia para cargar el nuevo código.
3. **Envío**: se envía una carga útil JSON que contiene la lista de archivos modificados al [hub de Mercure](https://mercure.rocks) integrado.
4. **Recepción**: El navegador, escuchando a través de la biblioteca JavaScript, recibe el evento de Mercure.
5. **Actualización**:

- Si se detecta **Idiomorph**, obtiene el contenido actualizado y transforma el HTML actual para que coincida con el nuevo estado, aplicando los cambios al instante sin perder el estado.
- De lo contrario, se llama a `window.location.reload()` para recargar la página.
