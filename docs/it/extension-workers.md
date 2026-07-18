# Extension worker

Gli extension worker consentono all'[estensione FrankenPHP](https://frankenphp.dev/docs/extensions/) di gestire un pool dedicato di thread PHP per l'esecuzione di attività in background, la gestione di eventi asincroni o l'implementazione di protocolli personalizzati. Utile per sistemi di code, ascoltatori di eventi, pianificatori, ecc.

## Registrare il worker

### Registrazione statica

Se non è necessario rendere il worker configurabile dall'utente (percorso di script fisso, numero fisso di thread), è possibile semplicemente registrare il worker nella funzione `init()`.

```go
// Registrazione statica
package myextension

import (
	"github.com/dunglas/frankenphp"
	"github.com/dunglas/frankenphp/caddy"
)

// Handle globale per comunicare con il pool dei worker
var worker frankenphp.Workers

func init() {
	// Registra il worker quando il modulo viene caricato.
	worker = caddy.RegisterWorkers(
		"my-internal-worker", // nome univoco
		"worker.php",         // Percorso dello script (relativo all'esecuzione o assoluto)
		2,                    // Conteggio fisso dei thread
		// Hook opzionali del ciclo di vita
		frankenphp.WithWorkerOnServerStartup(func() {
			// Logica globale di setup...
		}),
	)
}
```

### In un modulo Caddy (configurabile dall'utente)

Se si vuole condividere l'estensione (come una coda generica o un ascoltatore di eventi), andrebbe inserita in un modulo Caddy. Ciò consente agli utenti di configurare il percorso dello script e il conteggio dei thread tramite `Caddyfile`. Ciò richiede l'implementazione dell'interfaccia `caddy.Provisioner` e l'analisi del Caddyfile ([vedi l'esempio del modulo Caddy frankenphp-queue](https://github.com/dunglas/frankenphp-queue/blob/main/caddy.go)).

### In un'applicazione Go pura (embed)

Se si usa l'[embed di FrankenPHP in un'applicazione Go standard senza caddy](https://pkg.go.dev/github.com/dunglas/frankenphp#example-ServeHTTP), si possono registrare i worker usando `frankenphp.WithExtensionWorkers` durante l'inizializzazione delle opzioni.

## Interagire coi worker

Una volta che il pool di worker è attivo, gli si possono inviare attività. Lo si può fare all'interno di [funzioni native esportate in PHP](https://frankenphp.dev/docs/extensions/#writing-the-extension) o da qualsiasi logica Go, come un cron, un ascoltatore di eventi (MQTT, Kafka) o qualsiasi altra goroutine.

### Modalità headless: `SendMessage`

Utilizzare `SendMessage` per passare i dati grezzi direttamente al worker. Questo è l'ideale per code o comandi semplici.

#### Esempio di estensione della coda asincrona

```go
// Estensione FrankenPHP: invia messaggi a un worker tramite SendMessage
// #include <Zend/zend_types.h>
import "C"
import (
	"context"
	"unsafe"
	"github.com/dunglas/frankenphp"
)

//export_php:function my_queue_push(mixed $data): bool
func my_queue_push(data *C.zval) bool {
	// 1. Verifica che il worker sia pronto
	if worker == nil {
		return false
	}

	// 2. Invia al worker in background
	_, err := worker.SendMessage(
		context.Background(), // Contesto Go standard
		unsafe.Pointer(data), // Dati da passare al worker
		nil, // http.ResponseWriter (facoltativo)
	)

	return err == nil
}
```

### Emulazione HTTP: `SendRequest`

Utilizzare `SendRequest` se l'estensione deve richiamare uno script PHP che prevede un ambiente Web standard (popolando `$_SERVER`, `$_GET`, ecc.).

```go
// Estensione FrankenPHP: invoca uno script PHP con worker tramite SendRequest (emulazione HTTP)
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
	// 1. Prepara richiesta e recorder
	url := frankenphp.GoString(unsafe.Pointer(path))
	req, _ := http.NewRequest("GET", url, http.NoBody)
	rr := httptest.NewRecorder()

	// 2. Invia al worker
	if err := worker.SendRequest(rr, req); err != nil {
		return nil
	}

	// 3. Restituisce la risposta catturata
	return frankenphp.PHPString(rr.Body.String(), false)
}
```

## Script worker

Lo script worker PHP viene eseguito in un ciclo e può gestire sia messaggi non elaborati sia richieste HTTP.

```php
<?php
// Estensione FrankenPHP con script worker: gestisce messaggi grezzi e richieste HTTP
$handler = function ($payload = null) {
    // Caso 1: Modalità messaggio
    if ($payload !== null) {
        return "Received payload: " . $payload;
    }

    // Caso 2: Modalità HTTP (popola le variabili superglobali di PHP)
    echo "Hello from page: " . $_SERVER['REQUEST_URI'];
};

while (frankenphp_handle_request($handler)) {
    gc_collect_cycles();
}
```

## Hook del ciclo di vita

FrankenPHP fornisce hook per eseguire il codice Go in punti specifici del ciclo di vita.

| Tipo di hook | Nome opzione | Firma | Contesto e caso d'uso |
| :--------- | :--------------------------- | :------------------ | :--------------------------------------------------------------------- |
| **Server** | `WithWorkerOnServerStartup` | `func()` | Configurazione globale. Eseguito **Una volta**. Esempio: connettersi a NATS/Redis.            |
| **Server** | `WithWorkerOnServerShutdown` | `func()` | Pulizia globale. Eseguito **Una volta**. Esempio: chiudere le connessioni condivise.       |
| **Thread** | `WithWorkerOnReady` | `func(threadID int)` | Configurazione per thread. Chiamato quando inizia un thread. Riceve l'ID del thread. |
| **Thread** | `WithWorkerOnShutdown` | `func(threadID int)` | Pulizia per thread. Riceve l'ID del thread.                            |

### Esempio

```go
// Estensione FrankenPHP con worker con hook del ciclo di vita
package myextension

import (
    "fmt"
    "github.com/dunglas/frankenphp"
    frankenphpCaddy "github.com/dunglas/frankenphp/caddy"
)

func init() {
    workerHandle = frankenphpCaddy.RegisterWorkers(
        "my-worker", "worker.php", 2,

        // Avvio del server (globale)
        frankenphp.WithWorkerOnServerStartup(func() {
            fmt.Println("Extension: Server starting up...")
        }),

        // Thread pronto
        // Nota: la funzione accetta un intero che rappresenta l'ID del thread
        frankenphp.WithWorkerOnReady(func(id int) {
            fmt.Printf("Estensione: thread worker #%d pronto.\n", id)
        }),
    )
}
```
