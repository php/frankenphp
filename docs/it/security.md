---
title: "Modello di sicurezza di FrankenPHP: limiti di fiducia tra Go e PHP"
description: "Dove risiede il limite di fiducia di FrankenPHP: quali input non sono affidabili, di cosa è responsabile il runtime Go/C e cosa ricade sull'applicazione PHP o sui progetti upstream."
---

# Modello di sicurezza

Questo documento descrive il modello di fiducia di FrankenPHP: quali input sono affidabili, quali non lo sono e dove risiede il confine tra di essi.
Ha lo scopo di aiutare gli audit di sicurezza e gli scanner automatizzati a ragionare su **FrankenPHP stesso**, non sulle applicazioni PHP che serve.

Per i meccanismi interni qui menzionati (thread, il confine CGO, il sandboxing dell'ambiente), vedere [Internals](internals.md).
Per la persistenza dello stato nei processi a lunga esecuzione, vedere [Modalità Worker](worker.md).

## Limiti di fiducia

FrankenPHP viene eseguito in uno stack con quattro attori distinti:

| Attore                   | Fiducia                  | Note                                                                                                                                                |
| ----------------------- | ---------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Client remoto**       | Non affidabile              | La richiesta HTTP (metodo, URI, header, cookie, corpo, upload) è la principale fonte di input "tainted".                                             |
| **Operatore**            | Affidabile                | Fornisce la configurazione del deployment: il `Caddyfile`, le variabili d'ambiente, `php.ini`, le estensioni PHP e i moduli Caddy installati, e il codice dell'applicazione stesso. |
| **Codice applicazione PHP** | Affidabile *per provenienza* | Distribuito dall'operatore, quindi FrankenPHP non esegue mai codice fornito dall'attaccante, ma questo codice *consuma* dati di richiesta non affidabili.                    |
| **FrankenPHP (Go + C)** | Trusted computing base | Incorpora PHP, trasporta dati in entrata e in uscita, e isola richieste e thread. I suoi difetti sono ciò che questo documento considera.                            |

## Provenienza del codice vs. "taint" dei dati

Questa è la distinzione più importante, e quella che risolve la maggior parte della confusione "ci fidiamo di PHP o no?":

- **La provenienza del codice è affidabile.** FrankenPHP esegue solo file PHP distribuiti dall'operatore: lo script risolto nella document root o lo script worker configurato. Non valuta mai codice trasportato nella richiesta stessa (corpo, stringa di query, header): il confine SAPI trasporta *dati*, mai *codice* non affidabile.
- **I dati della richiesta sono "tainted".** Tutto ciò che il codice PHP legge dalla richiesta (`$_GET`, `$_POST`, `$_COOKIE`, `$_FILES`, `$_SERVER`, `php://input`) non è affidabile, esattamente come in qualsiasi SAPI PHP. Sanificarlo è compito dell'applicazione.

Quindi "ci fidiamo di ciò che proviene da PHP" è vero per il *codice*, mentre "la SAPI trasporta input non affidabili" è vero per i *dati*: i due non sono in conflitto.
Il compito di FrankenPHP è trasportare fedelmente questi dati "tainted" e impedire che i dati di una richiesta vengano divulgati in un'altra.

## Di cosa è responsabile FrankenPHP

La trusted computing base ha tre compiti. I difetti di sicurezza in FrankenPHP risiedono in uno di essi:

1. **Trasporto fedele**: mappare la richiesta nelle superglobali PHP e in `php://input`, e riportare l'output e gli header di PHP al client, senza introdurre iniezioni (iniezione di header/CRLF, request smuggling, esecuzione del file sbagliato).
2. **Isolamento**: impedire che lo stato con ambito di richiesta si incroci tra richieste, tra thread PHP e tra iterazioni del worker.
3. **Sicurezza della memoria**: gestire il confine CGO (Go ↔ C/PHP) senza corrompere la memoria.

## In scope: la superficie di attacco di FrankenPHP

Queste sono le superfici di cui FrankenPHP è responsabile. Una vulnerabilità qui è una vulnerabilità di FrankenPHP:

- **Mappatura richiesta a superglobale** (`cgi.go`, `frankenphp_register_server_vars`): costruire `$_SERVER`, `REMOTE_ADDR`, `SCRIPT_NAME`, `PATH_INFO` e le altre variabili CGI dalla richiesta.
- **Risoluzione del percorso dello script PHP**: il percorso della richiesta viene diviso su `split_path` (`.php` per impostazione predefinita) in `SCRIPT_NAME` / `PATH_INFO`, quindi unito alla document root con `sanitizedPathJoin` (`filepath.Join(root, filepath.Clean("/"+reqPath))`), che impedisce a `SCRIPT_FILENAME` di uscire dalla document root (path traversal). La direttiva `php_server` imposta inoltre una riscrittura predefinita `try_files` che instrada le richieste a file esistenti o al front controller, mitigando la classica insidia di PHP-FPM di eseguire il file sbagliato.
- **Isolamento dello stato in modalità worker**: FrankenPHP resetta `$_GET`, `$_POST`, `$_COOKIE`, `$_FILES`, `$_SERVER` e `$_REQUEST` tra le richieste, cancella esplicitamente `$_SESSION` (che altrimenti verrebbe divulgato tra le richieste) e gli errori restituiti da `error_get_last()` e `libxml_get_errors()`, e gli header restituiti da `http_get_last_response_headers()`, lo stato di `strtok()` e le variabili aggiunte da `output_add_rewrite_var()`, ma **`$_ENV` non viene resettato**, e le scritture di `putenv()`, le variabili `static`, le proprietà statiche delle classi, le variabili globali e le impostazioni del motore runtime (`ini_set()`, il contesto di stream predefinito, il fuso orario predefinito, la directory di lavoro, ...) persistono tra le richieste sullo stesso thread. I dati specifici della richiesta o dell'utente lasciati in quello stato possono trapelare in una richiesta successiva (vedere [Modalità Worker](worker.md#persistenza-dello-stato)).
- **Sandboxing dell'ambiente per thread**: `frankenphp_putenv()` / `frankenphp_getenv()` operano su un `sandboxed_env` locale al thread. Le scritture di `putenv()` rimangono all'interno di questo sandbox e non raggiungono mai l'ambiente OS globale del processo, quindi un segreto impostato da una richiesta non può trapelare ai processi figli (`proc_open`, `exec`, ...), al lato Go/Caddy o ad altre richieste. Un processo figlio che deve vedere tale variabile deve riceverla esplicitamente, tramite l'argomento ambiente di `proc_open()` o `exec()` (vedere [Internals](internals.md#sandboxing-dellambiente-per-thread)).
- **Confine di memoria CGO**: pinning delle stringhe Go e durata di `C.CString()` / `free()` attraverso il confine Go ↔ C.
- **API di amministrazione Caddy**: gli endpoint `/frankenphp/workers/restart` e `/frankenphp/threads`, esposti tramite l'API di amministrazione di Caddy (che ascolta su `localhost:2019` per impostazione predefinita). Esporre quell'endpoint oltre `localhost` è una decisione dell'operatore.
- **Gestione proxy affidabile**: gli header `X-Forwarded-*` in ingresso raggiungono sempre PHP come valori "tainted" di `$_SERVER['HTTP_X_FORWARDED_*']`; vengono considerati affidabili per derivare l'IP client reale e lo schema solo quando [`trusted_proxies`](production.md#esecuzione-dietro-un-reverse-proxy) è configurato.
- **Corpi di richiesta lenti**: un client che annuncia un corpo e poi lo invia a gocce o si blocca, trattiene il thread di gestione per tutta la durata. Con un pool di thread limitato, un numero sufficiente di tali connessioni lo esaurisce (Slow-POST DoS). FrankenPHP applica un timeout di inattività di 60s sulle letture del corpo per impostazione predefinita ([`request_body_timeout`](config.md#configurazione-del-caddyfile)), reimpostando la scadenza prima di ogni lettura in modo che un upload costante di qualsiasi dimensione abbia successo mentre uno bloccato viene interrotto e il thread rilasciato.

## Fuori scope

- **Vulnerabilità nel codice PHP dell'applicazione** (SQL injection, XSS, deserializzazione insicura, ecc.). FrankenPHP consegna dati di richiesta non affidabili all'applicazione senza modifiche; la difesa contro di essi è responsabilità dell'applicazione, proprio come con qualsiasi SAPI.
- **Difetti nei componenti upstream** utilizzati da FrankenPHP (PHP, Caddy, Go) o in progetti costruiti su di esso (Laravel Octane, Symfony Runtime). Segnalare questi problemi al progetto pertinente.

## Segnalazione di una vulnerabilità

Vedere [`SECURITY.md`](../SECURITY.md) per come segnalare un problema di sicurezza che riguarda FrankenPHP.
