---
title: Modalità worker di FrankenPHP: mantieni la tua app PHP in memoria
description: Esegui FrankenPHP in modalità worker per mantenere la tua applicazione PHP avviata tra le richieste, ridurre l'overhead di bootstrap e servire le risposte in millisecondi.
---

# Utilizzare i worker di FrankenPHP

Avvia la tua applicazione una volta e mantienila in memoria.
FrankenPHP gestirà le richieste in arrivo in pochi millisecondi.

## Avvio degli script worker di FrankenPHP

### Esecuzione di un worker FrankenPHP con Docker

Imposta il valore della variabile di ambiente `FRANKENPHP_CONFIG` su `worker /path/to/your/worker/script.php`:

```bash
docker run \
    -e FRANKENPHP_CONFIG="worker /app/path/to/your/worker/script.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### Esecuzione di un worker FrankenPHP con il binario autonomo

Usa l'opzione `--worker` del comando `php-server` per servire il contenuto della directory corrente utilizzando un worker:

```bash
frankenphp php-server --worker /path/to/your/worker/script.php
```

Se la tua app PHP è [incorporata nel binario](embed.md), puoi aggiungere un `Caddyfile` personalizzato nella directory radice dell'app.
Verrà utilizzato automaticamente.

È anche possibile [riavviare il worker in caso di modifiche ai file](config.md#monitorare-le-modifiche-ai-file) con l'opzione `--watch`.
Il seguente comando attiverà un riavvio se qualsiasi file che termina con `.php` nella directory `/path/to/your/app/` o nelle sue sottodirectory viene modificato:

```bash
frankenphp php-server --worker /path/to/your/worker/script.php --watch="/path/to/your/app/**/*.php"
```

Questa funzione viene spesso utilizzata in combinazione con il [ricaricamento a caldo](hot-reload.md).

## Modalità worker per Symfony

Vedi [la documentazione sulla modalità worker di FrankenPHP con Symfony](symfony.md#symfony-worker-mode-con-frankenphp).

## Modalità worker per Laravel Octane

Vedi [la documentazione di FrankenPHP Laravel Octane](laravel.md#laravel-octane).

## Scrittura di uno script worker FrankenPHP personalizzato

L'esempio seguente mostra come creare il tuo script worker senza fare affidamento su una libreria di terze parti:

```php
<?php
// public/index.php

// Avvio dell'applicazione
require __DIR__.'/vendor/autoload.php';

$myApp = new \App\Kernel();
$myApp->boot();

// Gestore fuori dal ciclo per migliori prestazioni (meno lavoro)
$handler = static function () use ($myApp) {
    try {
        // Chiamato quando viene ricevuta una richiesta,
        // i superglobali, php://input e simili vengono reimpostati
        echo $myApp->handle($_GET, $_POST, $_COOKIE, $_FILES, $_SERVER);
    } catch (\Throwable $exception) {
        // `set_exception_handler` viene chiamato solo quando termina lo script worker,
        // il che potrebbe non essere il comportamento atteso, quindi intercettare e gestire qui le eccezioni
        (new \MyCustomExceptionHandler)->handleException($exception);
    }
};

$maxRequests = (int)($_SERVER['MAX_REQUESTS'] ?? 0);
for ($nbRequests = 0; !$maxRequests || $nbRequests < $maxRequests; ++$nbRequests) {
    $keepRunning = \frankenphp_handle_request($handler);

    // Esegui qualcosa dopo aver inviato la risposta HTTP
    $myApp->terminate();

    // Richiama il garbage collector per ridurre la probabilità che venga attivato durante la generazione di una pagina
    gc_collect_cycles();

    if (!$keepRunning) break;
}

// Pulizia
$myApp->shutdown();
```

Quindi, avvia la tua app e utilizza la variabile di ambiente `FRANKENPHP_CONFIG` per configurare il tuo worker:

```bash
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

Per impostazione predefinita, vengono avviati 2 worker per CPU.
Puoi anche configurare il numero di worker da avviare:

```bash
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php 42" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### Utilizzo di PSR-15

Se la tua app utilizza [PSR-15](https://www.php-fig.org/psr/psr-15/) invece dei superglobali, converti la richiesta/risposta ai margini del gestore con [`nyholm/psr7`](https://github.com/Nyholm/psr7) e [`nyholm/psr7-server`](https://github.com/Nyholm/psr7-server):

```console
composer require nyholm/psr7 nyholm/psr7-server psr/http-server-handler
```

```php
<?php
// public/index.php

require __DIR__.'/vendor/autoload.php';

use Nyholm\Psr7\Factory\Psr17Factory;
use Nyholm\Psr7Server\ServerRequestCreator;

$myApp = new \App\Kernel(); // implements Psr\Http\Server\RequestHandlerInterface
$myApp->boot();

$psr17Factory = new Psr17Factory();
$creator = new ServerRequestCreator(
    $psr17Factory, // ServerRequestFactory
    $psr17Factory, // UriFactory
    $psr17Factory, // UploadedFileFactory
    $psr17Factory, // StreamFactory
);

$handler = static function () use ($myApp, $creator) {
    $response = $myApp->handle($creator->fromGlobals());

    http_response_code($response->getStatusCode());
    foreach ($response->getHeaders() as $name => $values) {
        foreach ($values as $value) {
            header("$name: $value", false);
        }
    }
    echo $response->getBody();
};

$maxRequests = (int)($_SERVER['MAX_REQUESTS'] ?? 0);
for ($nbRequests = 0; !$maxRequests || $nbRequests < $maxRequests; ++$nbRequests) {
    $keepRunning = \frankenphp_handle_request($handler);
    gc_collect_cycles();
    if (!$keepRunning) break;
}
```

### Riavvia il worker dopo un certo numero di richieste

Poiché PHP non è stato originariamente progettato per processi di lunga durata, molte librerie e codice legacy perdono ancora memoria.
Una soluzione alternativa all'utilizzo di questo tipo di codice in modalità worker consiste nel riavviare lo script worker dopo aver elaborato un certo numero di richieste:

Il precedente script worker consente di configurare un numero massimo di richieste da gestire impostando una variabile di ambiente denominata `MAX_REQUESTS`.

### Riavvia i worker manualmente

Sebbene sia possibile riavviare i worker [in seguito alle modifiche dei file](config.md#monitorare-le-modifiche-ai-file), è anche possibile riavviare tutti i worker
in modo elegante tramite l'[API di amministrazione Caddy](https://caddyserver.com/docs/api). Se l'amministratore è abilitato nel tuo
[Caddyfile](config.md#configurazione-caddyfile), puoi eseguire il ping dell'endpoint di riavvio con una semplice richiesta POST:

```bash
curl -X POST http://localhost:2019/frankenphp/workers/restart
```

### Fallimenti dei worker

Se uno script worker si blocca con un codice di uscita diverso da zero, FrankenPHP lo riavvierà con una strategia di backoff esponenziale.
Se lo script worker rimane attivo più a lungo dell'ultimo backoff × 2,
non penalizzerà lo script worker e lo riavvierà di nuovo.
Tuttavia, se lo script worker continua a fallire con un codice di uscita diverso da zero in un breve periodo di tempo
(ad esempio, avendo un errore di battitura in uno script), FrankenPHP si bloccherà con l'errore: `too many consecutive failures`.

Il numero di errori consecutivi può essere configurato nel tuo [Caddyfile](config.md#configurazione-caddyfile) con l'opzione `max_consecutive_failures`:

```caddyfile
frankenphp {
    worker {
        # ...
        max_consecutive_failures 10
    }
}
```

### Numero di thread

Quando la modalità worker è abilitata, è richiesto un thread aggiuntivo per gestire le richieste in arrivo,
quindi il valore predefinito di `num_threads` è `1 + 2x(numero di CPU disponibili)`.

Pertanto, in tale modalità se `num_threads` e `num` sono impostati esplicitamente, `num_threads` deve essere strettamente maggiore di `num`.

## Comportamento dei superglobali

[I superglobali PHP](https://www.php.net/manual/language.variables.superglobals.php) (`$_SERVER`, `$_ENV`, `$_GET`...)
si comportano come segue:

- prima della prima chiamata a `frankenphp_handle_request()`, i superglobali contengono valori legati allo script worker stesso
- durante e dopo la chiamata a `frankenphp_handle_request()`, i superglobali contengono valori generati dalla richiesta HTTP elaborata, ogni chiamata a `frankenphp_handle_request()` modifica i valori dei superglobali

Per accedere ai superglobali dello script worker all'interno del callback, è necessario copiarli e importare la copia nell'ambito del callback:

```php
<?php
// Copia il superglobale $_SERVER del worker prima della prima chiamata a frankenphp_handle_request()
$workerServer = $_SERVER;

$handler = static function () use ($workerServer) {
    var_dump($_SERVER); // $_SERVER legato alla richiesta
    var_dump($workerServer); // $_SERVER dello script worker
};

// ...
```

La maggior parte dei superglobali (`$_GET`, `$_POST`, `$_COOKIE`, `$_FILES`, `$_SERVER`, `$_REQUEST`) vengono reimpostati automaticamente tra una richiesta e l'altra.
Tuttavia, **`$_ENV` attualmente non viene reimpostato tra una richiesta e l'altra**.
Ciò significa che qualsiasi modifica apportata a `$_ENV` durante una richiesta persisterà e sarà visibile alle richieste successive gestite dallo stesso worker thread.
Evitare di archiviare dati sensibili o specifici della richiesta in `$_ENV`.

## Persistenza dello stato

Poiché la modalità worker mantiene attivo il processo PHP tra le richieste, il seguente stato persiste tra le richieste:

- **Variabili statiche**: le variabili dichiarate con `static` all'interno di funzioni o metodi mantengono i loro valori tra le richieste.
- **Proprietà statiche della classe**: le proprietà statiche sulle classi persistono tra le richieste.
- **Variabili globali**: le variabili nell'ambito globale dello script worker persistono tra le richieste.
- **Cache in memoria**: tutti i dati archiviati in memoria (array, oggetti) all'esterno del gestore delle richieste persistono.
- **Impostazioni del motore di runtime**: le modifiche apportate con `ini_set()`, `stream_context_set_default()`, `date_default_timezone_set()`, `chdir()`, `stream_wrapper_register()`, `set_error_handler()` o `set_exception_handler()` rimangono in vigore per le richieste successive. Non archiviare mai valori specifici della richiesta o dell'utente in queste impostazioni, come le credenziali nel contesto del flusso predefinito: passa invece un contesto esplicito ad ogni chiamata.
- **Impostazioni a livello di processo**: `umask()` e `setlocale()` modificano l'intero processo FrankenPHP, non solo il thread corrente. La modalità worker non li ripristina tra le richieste, e in ogni modalità influenzano anche le richieste in esecuzione contemporaneamente. Impostali una volta all'avvio, o chiama `chmod()` sui file creati invece di modificare la umask.

Questo è previsto dalla progettazione ed è ciò che rende veloce la modalità worker. Tuttavia, richiede attenzione per evitare effetti collaterali indesiderati:

```php
<?php
function getCounter(): int {
    static $count = 0;
    return ++$count; // Incrementa tra le richieste!
}

$handler = static function () {
    echo getCounter(); // 1, 2, 3, ... per ogni richiesta su questo thread
};

while (\frankenphp_handle_request($handler)) {
    // ...
}
```

Quando si scrivono script worker, assicurarsi di reimpostare qualsiasi stato specifico della richiesta tra le richieste.
Framework come [Symfony](symfony.md) e [Laravel Octane](laravel.md) si occupano di ripristinare automaticamente la maggior parte degli stati, ma potrebbe comunque essere necessario ripristinare alcuni servizi. Con Symfony, i servizi che mantengono uno stato specifico della richiesta dovrebbero implementare [`Symfony\Contracts\Service\ResetInterface`](https://github.com/symfony/contracts/blob/main/Service/ResetInterface.php) in modo che vengano reimpostati dal kernel tra una richiesta e l'altra.
