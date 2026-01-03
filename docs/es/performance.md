# Rendimiento

Por defecto, FrankenPHP intenta ofrecer un buen compromiso entre rendimiento y facilidad de uso.
Sin embargo, es posible mejorar sustancialmente el rendimiento usando una configuración adecuada.

## Número de Hilos y Workers

Por defecto, FrankenPHP inicia 2 veces más hilos y workers (en modo worker) que el número de CPUs disponibles.

Los valores apropiados dependen en gran medida de cómo está escrita tu aplicación, qué hace y tu hardware.
Recomendamos encarecidamente cambiar estos valores. Para una mejor estabilidad del sistema, se recomienda que `num_threads` x `memory_limit` < `memoria_disponible`.

Para encontrar los valores correctos, es mejor ejecutar pruebas de carga que simulen tráfico real.
[k6](https://k6.io) y [Gatling](https://gatling.io) son buenas herramientas para esto.

Para configurar el número de hilos, usa la opción `num_threads` de las directivas `php_server` y `php`.
Para cambiar el número de workers, usa la opción `num` de la sección `worker` de la directiva `frankenphp`.

### `max_threads`

Aunque siempre es mejor saber exactamente cómo será tu tráfico, las aplicaciones reales tienden a ser más impredecibles.
La configuración `max_threads` [configuración](config.md#caddyfile-config) permite a FrankenPHP generar automáticamente hilos adicionales en tiempo de ejecución hasta el límite especificado.
`max_threads` puede ayudarte a determinar cuántos hilos necesitas para manejar tu tráfico y puede hacer que el servidor sea más resiliente a picos de latencia.
Si se establece en `auto`, el límite se estimará en función del `memory_limit` en tu `php.ini`. Si no puede hacerlo,
`auto` se establecerá por defecto en 2x `num_threads`. Ten en cuenta que `auto` puede subestimar fuertemente el número de hilos necesarios.
`max_threads` es similar a [pm.max_children](https://www.php.net/manual/es/install.fpm.configuration.php#pm.max-children) de PHP FPM. La principal diferencia es que FrankenPHP usa hilos en lugar de procesos y los delega automáticamente entre diferentes scripts de worker y el 'modo clásico' según sea necesario.

## Modo Worker

Habilitar [el modo worker](worker.md) mejora drásticamente el rendimiento,
pero tu aplicación debe adaptarse para ser compatible con este modo:
debes crear un script de worker y asegurarte de que la aplicación no tenga fugas de memoria.

## No Usar musl

La variante Alpine Linux de las imágenes Docker oficiales y los binarios predeterminados que proporcionamos usan [la libc musl](https://musl.libc.org).

Se sabe que PHP es [más lento](https://gitlab.alpinelinux.org/alpine/aports/-/issues/14381) cuando usa esta biblioteca C alternativa en lugar de la biblioteca GNU tradicional,
especialmente cuando se compila en modo ZTS (thread-safe), que es requerido para FrankenPHP. La diferencia puede ser significativa en un entorno con muchos hilos.

Además, [algunos errores solo ocurren cuando se usa musl](https://github.com/php/php-src/issues?q=sort%3Aupdated-desc+is%3Aissue+is%3Aopen+label%3ABug+musl).

En entornos de producción, recomendamos usar FrankenPHP vinculado a glibc, compilado con un nivel de optimización adecuado.

Esto se puede lograr usando las imágenes Docker de Debian, usando los paquetes de nuestros mantenedores [.deb](https://debs.henderkes.com) o [.rpm](https://rpms.henderkes.com), o [compilando FrankenPHP desde las fuentes](compile.md).

## Configuración del Runtime de Go

FrankenPHP está escrito en Go.

En general, el runtime de Go no requiere ninguna configuración especial, pero en ciertas circunstancias,
una configuración específica mejora el rendimiento.

Probablemente quieras establecer la variable de entorno `GODEBUG` en `cgocheck=0` (el valor predeterminado en las imágenes Docker de FrankenPHP).

Si ejecutas FrankenPHP en contenedores (Docker, Kubernetes, LXC...) y limitas la memoria disponible para los contenedores,
establece la variable de entorno `GOMEMLIMIT` en la cantidad de memoria disponible.

Para más detalles, [la página de documentación de Go dedicada a este tema](https://pkg.go.dev/runtime#hdr-Environment_Variables) es una lectura obligada para aprovechar al máximo el runtime.

## `file_server`

Por defecto, la directiva `php_server` configura automáticamente un servidor de archivos para
servir archivos estáticos (assets) almacenados en el directorio raíz.

Esta característica es conveniente, pero tiene un costo.
Para deshabilitarla, usa la siguiente configuración:

```caddyfile
php_server {
    file_server off
}
```

## `try_files`

Además de los archivos estáticos y los archivos PHP, `php_server` también intentará servir los archivos de índice de tu aplicación
y los índices de directorio (`/ruta/` -> `/ruta/index.php`). Si no necesitas índices de directorio,
puedes deshabilitarlos definiendo explícitamente `try_files` de esta manera:

```caddyfile
php_server {
    try_files {path} index.php
    root /ruta/a/tu/app # agregar explícitamente la raíz aquí permite un mejor almacenamiento en caché
}
```

Esto puede reducir significativamente el número de operaciones de archivo innecesarias.

Un enfoque alternativo con 0 operaciones innecesarias de sistema de archivos sería usar en su lugar la directiva `php` y separar
los archivos de PHP por ruta. Este enfoque funciona bien si toda tu aplicación es servida por un solo archivo de entrada.
Un ejemplo de [configuración](config.md#caddyfile-config) que sirve archivos estáticos detrás de una carpeta `/assets` podría verse así:

```caddyfile
route {
    @assets {
        path /assets/*
    }

    # todo lo que está detrás de /assets es manejado por el servidor de archivos
    file_server @assets {
        root /ruta/a/tu/app
    }

    # todo lo que no está en /assets es manejado por tu archivo index o worker PHP
    rewrite index.php
    php {
        root /ruta/a/tu/app # agregar explícitamente la raíz aquí permite un mejor almacenamiento en caché
    }
}
```

## Marcadores de Posición (Placeholders)

Puedes usar [marcadores de posición](https://caddyserver.com/docs/conventions#placeholders) en las directivas `root` y `env`.
Sin embargo, esto evita el almacenamiento en caché de estos valores y conlleva un costo significativo de rendimiento.

Si es posible, evita los marcadores de posición en estas directivas.

## `resolve_root_symlink`

Por defecto, si la raíz del documento es un enlace simbólico, se resuelve automáticamente por FrankenPHP (esto es necesario para que PHP funcione correctamente).
Si la raíz del documento no es un enlace simbólico, puedes deshabilitar esta característica.

```caddyfile
php_server {
    resolve_root_symlink false
}
```

Esto mejorará el rendimiento si la directiva `root` contiene [marcadores de posición](https://caddyserver.com/docs/conventions#placeholders).
La ganancia será negligible en otros casos.

## Registros (Logs)

El registro es obviamente muy útil, pero, por definición,
requiere operaciones de E/S y asignaciones de memoria, lo que reduce considerablemente el rendimiento.
Asegúrate de [establecer el nivel de registro](https://caddyserver.com/docs/caddyfile/options#log) correctamente,
y registra solo lo necesario.

## Rendimiento de PHP

FrankenPHP usa el intérprete oficial de PHP.
Todas las optimizaciones de rendimiento habituales relacionadas con PHP se aplican con FrankenPHP.

En particular:

- verifica que [OPcache](https://www.php.net/manual/es/book.opcache.php) esté instalado, habilitado y correctamente configurado
- habilita [optimizaciones del autoload de Composer](https://getcomposer.org/doc/articles/autoloader-optimization.md)
- asegúrate de que la caché `realpath` sea lo suficientemente grande para las necesidades de tu aplicación
- usa [preloading](https://www.php.net/manual/es/opcache.preloading.php)

Para más detalles, lee [la entrada de documentación dedicada de Symfony](https://symfony.com/doc/current/performance.html)
(la mayoría de los consejos son útiles incluso si no usas Symfony).

## Dividiendo el Pool de Hilos

Es común que las aplicaciones interactúen con servicios externos lentos, como una
API que tiende a ser poco confiable bajo alta carga o que consistentemente tarda 10+ segundos en responder.
En tales casos, puede ser beneficioso dividir el pool de hilos para tener pools "lentos" dedicados.
Esto evita que los endpoints lentos consuman todos los recursos/hilos del servidor y
limita la concurrencia de solicitudes hacia el endpoint lento, similar a un
pool de conexiones.

```caddyfile
{
    frankenphp {
        max_threads 100 # máximo 100 hilos compartidos por todos los workers
    }
}

ejemplo.com {
    php_server {
        root /app/public # la raíz de tu aplicación
        worker index.php {
            match /endpoint-lento/* # todas las solicitudes con la ruta /endpoint-lento/* son manejadas por este pool de hilos
            num 10 # mínimo 10 hilos para solicitudes que coincidan con /endpoint-lento/*
        }
        worker index.php {
            match * # todas las demás solicitudes son manejadas por separado
            num 20 # mínimo 20 hilos para otras solicitudes, incluso si los endpoints lentos comienzan a colgarse
        }
    }
}
```

En general, también es aconsejable manejar endpoints muy lentos de manera asíncrona, utilizando mecanismos relevantes como colas de mensajes.
