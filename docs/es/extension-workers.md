# Extension Workers

Los Extension Workers permiten que tu [extensión FrankenPHP](https://frankenphp.dev/docs/extensions/) gestione un pool dedicado de hilos PHP para ejecutar tareas en segundo plano, manejar eventos asíncronos o implementar protocolos personalizados. Útil para sistemas de colas, listeners de eventos, programadores, etc.

## Registrando el Worker

### Registro Estático

Si no necesitas hacer que el worker sea configurable por el usuario (ruta de script fija, número fijo de hilos), simplemente puedes registrar el worker en la función `init()`.

```go
package myextension

import (
	"github.com/dunglas/frankenphp"
	"github.com/dunglas/frankenphp/caddy"
)

// Manejador global para comunicarse con el pool de workers
var worker frankenphp.Workers

func init() {
	// Registrar el worker cuando se carga el módulo.
	worker = caddy.RegisterWorkers(
		"my-internal-worker", // Nombre único
		"worker.php",         // Ruta del script (relativa a la ejecución o absoluta)
		2,                    // Número fijo de hilos
		// Hooks de ciclo de vida opcionales
		frankenphp.WithWorkerOnServerStartup(func() {
			// Lógica de configuración global...
		}),
	)
}
```

### En un Módulo Caddy (Configurable por el usuario)

Si planeas compartir tu extensión (como una cola genérica o listener de eventos), deberías envolverla en un módulo Caddy. Esto permite a los usuarios configurar la ruta del script y el número de hilos a través de su `Caddyfile`. Esto requiere implementar la interfaz `caddy.Provisioner` y analizar el Caddyfile ([ver un ejemplo](https://github.com/dunglas/frankenphp-queue/blob/989120d394d66dd6c8e2101cac73dd622fade334/caddy.go)).

### En una Aplicación Go Pura (Embedding)

Si estás [embebiendo FrankenPHP en una aplicación Go estándar sin caddy](https://pkg.go.dev/github.com/dunglas/frankenphp#example-ServeHTTP), puedes registrar extension workers usando `frankenphp.WithExtensionWorkers` al inicializar las opciones.

## Interactuando con Workers

Una vez que el pool de workers está activo, puedes enviar tareas a él. Esto se puede hacer dentro de [funciones nativas exportadas a PHP](https://frankenphp.dev/docs/extensions/#writing-the-extension), o desde cualquier lógica Go como un programador cron, un listener de eventos (MQTT, Kafka), o cualquier otra goroutine.

### Modo Sin Cabeza: `SendMessage`

Usa `SendMessage` para pasar datos sin procesar directamente a tu script worker. Esto es ideal para colas o comandos simples.

#### Ejemplo: Una Extensión de Cola Asíncrona

```go
// #include <Zend/zend_types.h>
import "C"
import (
	"context"
	"unsafe"
	"github.com/dunglas/frankenphp"
)

//export_php:function my_queue_push(mixed $data): bool
func my_queue_push(data *C.zval) bool {
	// 1. Asegurar que el worker esté listo
	if worker == nil {
		return false
	}

	// 2. Enviar al worker en segundo plano
	_, err := worker.SendMessage(
		context.Background(), // Contexto estándar de Go
		unsafe.Pointer(data), // Datos para pasar al worker
		nil, // http.ResponseWriter opcional
	)

	return err == nil
}
```

### Emulación HTTP: `SendRequest`

Usa `SendRequest` si tu extensión necesita invocar un script PHP que espera un entorno web estándar (poblando `$_SERVER`, `$_GET`, etc.).

```go
// #include <Zend/zend_types.h>
import "C"
import (
	"net/http"
	"net/http/httptest"
	"unsafe"
	"github.com/dunglas/frankenphp"
)

//export_php:function my_worker_http_request(string $path): string
func my_worker_http_request(path *C.zend_string) unsafe.Pointer {
	// 1. Preparar la solicitud y el grabador
	url := frankenphp.GoString(unsafe.Pointer(path))
	req, _ := http.NewRequest("GET", url, http.NoBody)
	rr := httptest.NewRecorder()

	// 2. Enviar al worker
	if err := worker.SendRequest(rr, req); err != nil {
		return nil
	}

	// 3. Devolver la respuesta capturada
	return frankenphp.PHPString(rr.Body.String(), false)
}
```

## Script Worker

El script worker PHP se ejecuta en un bucle y puede manejar tanto mensajes sin procesar como solicitudes HTTP.

```php
<?php
// Manejar tanto mensajes sin procesar como solicitudes HTTP en el mismo bucle
$handler = function ($payload = null) {
    // Caso 1: Modo Mensaje
    if ($payload !== null) {
        return "Payload recibido: " . $payload;
    }

    // Caso 2: Modo HTTP (las superglobales estándar de PHP están pobladas)
    echo "Hola desde la página: " . $_SERVER['REQUEST_URI'];
};

while (frankenphp_handle_request($handler)) {
    gc_collect_cycles();
}
```

## Hooks de Ciclo de Vida

FrankenPHP proporciona hooks para ejecutar código Go en puntos específicos del ciclo de vida.

| Tipo de Hook | Nombre de Opción             | Firma                | Contexto y Caso de Uso                                                      |
| :----------- | :--------------------------- | :------------------- | :-------------------------------------------------------------------------- |
| **Server**   | `WithWorkerOnServerStartup`  | `func()`             | Configuración global. Se ejecuta **Una vez**. Ejemplo: Conectar a NATS/Redis. |
| **Server**   | `WithWorkerOnServerShutdown` | `func()`             | Limpieza global. Se ejecuta **Una vez**. Ejemplo: Cerrar conexiones compartidas. |
| **Thread**   | `WithWorkerOnReady`          | `func(threadID int)` | Configuración por hilo. Llamado cuando un hilo inicia. Recibe el ID del hilo. |
| **Thread**   | `WithWorkerOnShutdown`       | `func(threadID int)` | Limpieza por hilo. Recibe el ID del hilo.                                   |

### Ejemplo

```go
package myextension

import (
    "fmt"
    "github.com/dunglas/frankenphp"
    frankenphpCaddy "github.com/dunglas/frankenphp/caddy"
)

func init() {
    workerHandle = frankenphpCaddy.RegisterWorkers(
        "my-worker", "worker.php", 2,

        // Inicio del Servidor (Global)
        frankenphp.WithWorkerOnServerStartup(func() {
            fmt.Println("Extension: Servidor iniciando...")
        }),

        // Hilo Listo (Por Hilo)
        // Nota: La función acepta un entero que representa el ID del hilo
        frankenphp.WithWorkerOnReady(func(id int) {
            fmt.Printf("Extension: Hilo worker #%d está listo.\n", id)
        }),
    )
}
```
