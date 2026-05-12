# Utilizzare i worker

Avviare l'applicazione una volta, tenendola in memoria.
FrankenPHP gestirà le richieste in arrivo in pochi millisecondi.

## Avvio dei worker

### Esecuzione di un worker FrankenPHP con Docker

Impostare il valore della variabile di ambiente `FRANKENPHP_CONFIG` su `worker /path/to/your/worker/script.php`:

```bash
docker run \
    -e FRANKENPHP_CONFIG="worker /app/path/to/your/worker/script.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### Esecuzione di un worker FrankenPHP con il file binario autonomo

Utilizzare l'opzione `--worker` del comando `php-server` per servire il contenuto della cartella corrente utilizzando un worker:

```bash
frankenphp php-server --worker /path/to/your/worker/script.php
```

Se l'app PHP è [incorporata nel file binario](embed.md), si può aggiungere un `Caddyfile` personalizzato nella cartella principale dell'app.
Verrà utilizzato automaticamente.

È anche possibile [riavviare il worker in caso di modifiche al file](config.md#controllo-delle-modifiche-ai-file) con l'opzione `--watch`.
Il seguente comando attiverà un riavvio se qualsiasi file che termina con `.php` nella cartella o nelle sottocartelle `/path/to/your/app/` viene modificato:

```bash
frankenphp php-server --worker /path/to/your/worker/script.php --watch="/path/to/your/app/**/*.php"
```

Questa funzione viene spesso utilizzata in combinazione con [ricaricamento a caldo](hot-reload.md).

## Modalità worker per Symfony

Vedi [la documentazione sulla modalità worker di FrankenPHP con Symfony](symfony.md#modalità-worker-con-symfony-e-frankenphp).

## Modalità worker per Laravel Octane

Vedi [la documentazione di FrankenPHP Laravel Octane](laravel.md#laravel-octane).

## Scrittura di un worker personalizzato

L'esempio seguente mostra come creare il proprio worker senza fare affidamento su una libreria di terze parti:

```php
<?php
// public/index.php

// Avvio dell'applicazione
require __DIR__.'/vendor/autoload.php';

$myApp = new \App\Kernel();
$myApp->boot();

// Gestore fuori dal ciclo per migliori prestazioni (meno lavoro per richiesta)
$handler = static function () use ($myApp) {
    try {
        // Chiamato quando arriva una richiesta,
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

    // Richiama il garbage collector per ridurre la probabilità che parta durante la generazione di una pagina
    gc_collect_cycles();

    if (!$keepRunning) break;
}

// Pulizia
$myApp->shutdown();
```

Quindi, avviare l'app e utilizzare la variabile di ambiente `FRANKENPHP_CONFIG` per configurare il worker:

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

### Riavvia il worker dopo un certo numero di richieste

Poiché PHP non è stato originariamente progettato per processi di lunga durata, ci sono ancora molte librerie e codici legacy che perdono memoria.
Una soluzione alternativa all'utilizzo di questo tipo di codice in modalità worker consiste nel riavviare lo script worker dopo aver elaborato un certo numero di richieste:

Il precedente frammento di lavoro consente di configurare un numero massimo di richieste da gestire impostando una variabile di ambiente denominata `MAX_REQUESTS`.

### Riavvia i worker manualmente

Sebbene sia possibile riavviare i worker [in seguito alle modifiche dei file](config.md#controllo-delle-modifiche-ai-file), è anche possibile riavviare tutti i worker
tramite l'[API di amministrazione Caddy](https://caddyserver.com/docs/api). Se l'amministratore è abilitato in
[Caddyfile](config.md#caddyfile-config), si può eseguire il ping dell'endpoint di riavvio con una semplice richiesta POST:

```bash
curl -X POST http://localhost:2019/frankenphp/workers/restart
```

### Fallimenti dei worker

Se un worker si blocca con un codice di uscita diverso da zero, FrankenPHP lo riavvierà con una strategia di backoff esponenziale.
Se lo script worker rimane attivo più a lungo dell'ultimo backoff moltiplicato per due,
non penalizzerà lo script worker e lo riavvierà di nuovo.
Tuttavia, se il worker continua a fallire con un codice di uscita diverso da zero in un breve periodo di tempo
(ad esempio, avendo un errore di battitura in uno script), FrankenPHP si bloccherà con l'errore: `too many consecutive failures`.

Il numero di errori consecutivi può essere configurato nel [Caddyfile](config.md#caddyfile-config) con l'opzione `max_consecutive_failures`:

```caddyfile
frankenphp {
    worker {
        # ...
        max_consecutive_failures 10
    }
}
```

## Comportamento dei superglobali

[I superglobali PHP](https://www.php.net/manual/en/language.variables.superglobals.php) (`$_SERVER`, `$_ENV`, `$_GET`...)
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
- **Variabili globali**: le variabili nell'ambito globale del worker persistono tra le richieste.
- **Cache in memoria**: tutti i dati archiviati in memoria (array, oggetti) all'esterno del gestore delle richieste persistono.

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

Quando si scrivono worker, assicurarsi di reimpostare qualsiasi stato specifico della richiesta tra le richieste.
Framework come [Symfony](symfony.md) e [Laravel Octane](laravel.md) si occupano di ripristinare automaticamente la maggior parte degli stati, ma potrebbe comunque essere necessario ripristinare alcuni servizi. Con Symfony, i servizi che mantengono uno stato specifico della richiesta dovrebbero implementare [`Symfony\Contracts\Service\ResetInterface`](https://github.com/symfony/contracts/blob/main/Service/ResetInterface.php) in modo che vengano reimpostati dal kernel tra una richiesta e l'altra.
