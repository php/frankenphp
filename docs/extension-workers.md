# Extension Workers

Extension Workers enable your [FrankenPHP extension](https://frankenphp.dev/docs/extensions/) to manage a dedicated pool of PHP threads for executing background tasks, handling asynchronous events, or implementing custom protocols. Useful for queue systems, event listeners, schedulers, etc.

## Registering the Worker

### Static Registration

If you don't need to make the worker configurable by the user (fixed script path, fixed number of threads), you can simply register the worker in the `init()` function.

```go
package myextension

import (
	"github.com/dunglas/frankenphp"
	"github.com/dunglas/frankenphp/caddy"
)

// Global handle to communicate with the worker pool
var worker frankenphp.Workers

func init() {
	// Register the worker when the module is loaded.
	worker = caddy.RegisterWorkers(
		"my-internal-worker", // Unique name
		"worker.php",         // Script path (relative to execution or absolute)
		2,                    // Fixed Thread count
		// Optional Lifecycle Hooks
		frankenphp.WithWorkerOnServerStartup(func() {
			// Global setup logic...
		}),
	)
}
```

### In a Caddy Module (Configurable by the user)

If you plan to share your extension (like a generic queue or event listener), you should wrap it in a Caddy module. This allows users to configure the script path and thread count via their `Caddyfile`. This requires implementing the `caddy.Provisioner` interface and parsing the Caddyfile ([see an example](https://github.com/dunglas/frankenphp-queue/blob/989120d394d66dd6c8e2101cac73dd622fade334/caddy.go)).

### In a Pure Go Application (Embedding)

If you are [embedding FrankenPHP in a standard Go application without caddy](https://pkg.go.dev/github.com/dunglas/frankenphp#example-ServeHTTP), you can register extension workers using `frankenphp.WithExtensionWorkers` when initializing options.

## Interacting with Workers

Once the worker pool is active, you can dispatch tasks to it. This can be done inside [native functions exported to PHP](https://frankenphp.dev/docs/extensions/#writing-the-extension), or from any Go logic such as a cron scheduler, an event listener (MQTT, Kafka), or a any other goroutine.

### Headless Mode : `SendMessage`

Use `SendMessage` to pass raw data directly to your worker script. This is ideal for queues or simple commands.

#### Example: An Async Queue Extension

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
	// 1. Ensure worker is ready
	if worker == nil {
		return false
	}

	// 2. Dispatch to the background worker
	_, err := worker.SendMessage(
		context.Background(), // Standard Go context
		unsafe.Pointer(data), // Data to pass to the worker
		nil, // Optional http.ResponseWriter
	)

	return err == nil
}
```

### HTTP Emulation :`SendRequest`

Use `SendRequest` if your extension needs to invoke a PHP script that expects a standard web environment (populating `$_SERVER`, `$_GET`, etc.).

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
	// 1. Prepare the request and recorder
	url := frankenphp.GoString(unsafe.Pointer(path))
	req, _ := http.NewRequest("GET", url, http.NoBody)
	rr := httptest.NewRecorder()

	// 2. Dispatch to the worker
	if err := worker.SendRequest(rr, req); err != nil {
		return nil
	}

	// 3. Return the captured response
	return frankenphp.PHPString(rr.Body.String(), false)
}
```

## Worker Script

The PHP worker script runs in a loop and can handle both raw messages and HTTP requests.

```php
<?php
// Handle both raw messages and HTTP requests in the same loop
$handler = function ($payload = null) {
    // Case 1: Message Mode
    if ($payload !== null) {
        return "Received payload: " . $payload;
    }

    // Case 2: HTTP Mode (standard PHP superglobals are populated)
    echo "Hello from page: " . $_SERVER['REQUEST_URI'];
};

while (frankenphp_handle_request($handler)) {
    gc_collect_cycles();
}
```

## Lifecycle Hooks

FrankenPHP provides hooks to execute Go code at specific points in the lifecycle.

| Hook Type  | Option Name                  | Signature            | Context & Use Case                                                     |
| :--------- | :--------------------------- | :------------------- | :--------------------------------------------------------------------- |
| **Server** | `WithWorkerOnServerStartup`  | `func()`             | Global setup. Run **Once**. Example: Connect to NATS/Redis.            |
| **Server** | `WithWorkerOnServerShutdown` | `func()`             | Global cleanup. Run **Once**. Example: Close shared connections.       |
| **Thread** | `WithWorkerOnReady`          | `func(threadID int)` | Per-thread setup. Called when a thread starts. Receives the Thread ID. |
| **Thread** | `WithWorkerOnShutdown`       | `func(threadID int)` | Per-thread cleanup. Receives the Thread ID.                            |

### Example

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

        // Server Startup (Global)
        frankenphp.WithWorkerOnServerStartup(func() {
            fmt.Println("Extension: Server starting up...")
        }),

        // Thread Ready (Per Thread)
        // Note: The function accepts an integer representing the Thread ID
        frankenphp.WithWorkerOnReady(func(id int) {
            fmt.Printf("Extension: Worker thread #%d is ready.\n", id)
        }),
    )
}
```
