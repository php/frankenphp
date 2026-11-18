---
title: Problemi noti di FrankenPHP, estensioni incompatibili e soluzioni alternative
description: Riferimento alle limitazioni di FrankenPHP, incluse estensioni PHP non supportate, avvertenze su musl libc, configurazione TLS di Docker con 127.0.0.1 e script Composer @php.
---

# Problemi noti

## Estensioni PHP non supportate

Le seguenti estensioni non sono compatibili con FrankenPHP:

| Nome                                                                                                        | Motivo          | Alternative                                                                                                                                                                    |
| ----------------------------------------------------------------------------------------------------------- | --------------- |---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [imap](https://www.php.net/manual/imap.installation.php)                                                 | Non thread-safe | [javanile/php-imap2](https://github.com/javanile/php-imap2), [webklex/php-imap](https://github.com/Webklex/php-imap), [ImapEngine](https://github.com/DirectoryTree/ImapEngine) |
| [newrelic](https://docs.newrelic.com/docs/apm/agents/php-agent/getting-started/introduction-new-relic-php/) | Non thread-safe | -                                                                                                                                                                               |
| [pcov](https://github.com/krakjoe/pcov)                                                                     | Non thread-safe ([SIGSEGV in modalità classica](https://github.com/krakjoe/pcov/issues/132)) | Raccogliere la copertura con la SAPI CLI (eseguire PHPUnit/Pest al di fuori di FrankenPHP) |

## Estensioni PHP difettose

Le seguenti estensioni presentano bug noti e comportamenti imprevisti se utilizzate con FrankenPHP:

| Nome                                  | Problema                                                                                                                                         |
|---------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------|
| [datadog](https://github.com/DataDog) | Sperimenta instabilità durante la profilazione di FrankenPHP. Questo bug è [monitorato da DataDog](https://github.com/DataDog/dd-trace-php/issues/3729). |
| [blackfire](https://blackfire.io/) | [Il supporto FrankenPHP è in beta e non ancora completo di funzionalità](https://docs.blackfire.io/php/integrations/frankenphp).                            |
| [imagick](https://github.com/Imagick/imagick) | I thread OpenMP di ImageMagick sono in conflitto con i thread di FrankenPHP, causando instabilità e crash. Mitigare disabilitando il threading di ImageMagick tramite `\Imagick::setResourceLimit(\Imagick::RESOURCETYPE_THREAD, 1)` o compilando ImageMagick con `--disable-openmp`. OpenMP è già disabilitato nei binari statici e nei pacchetti ufficiali `apt`/`apk`/`rpm`, quindi sono interessate solo le immagini Docker e le installazioni Homebrew. |

## Il superamento del limite di memoria rigido di PHP arresta l'intero server

A partire da PHP 8.2.35, 8.3.36, 8.4.27 e 8.5.12, il superamento di `memory_limit` non arresta più lo script immediatamente: l'errore fatale "Allowed memory size exhausted" viene sollevato alla successiva chiamata di funzione o iterazione del ciclo.
Se lo script alloca oltre il limite di memoria rigido prima di ciò, PHP esce dal processo.
Il limite rigido predefinito è il doppio di `memory_limit` (o `memory_limit` più 1 GiB al di sopra di 1 GiB) e può essere modificato con l'impostazione INI `memory_hard_limit_percent`.

Il codice che continua a essere eseguito oltre `memory_limit` può raggiungerlo, ad esempio una singola allocazione maggiore di `memory_limit` (come `array_merge()` su array di grandi dimensioni) o concatenazioni di stringhe consecutive.

PHP-FPM perde solo un processo figlio.
FrankenPHP serve tutte le richieste da thread di un singolo processo, quindi ogni richiesta in corso viene persa e il server si ferma fino al riavvio.
Eseguire FrankenPHP sotto un supervisore che lo riavvia, come una policy di riavvio di Docker o `Restart=` di systemd (già impostato nei pacchetti `deb` e `rpm`).

## get_browser

La funzione [get_browser()](https://www.php.net/manual/function.get-browser.php) sembra funzionare male dopo un po'. Una soluzione alternativa consiste nel memorizzare nella cache (ad esempio con [APCu](https://www.php.net/manual/book.apcu.php)) i risultati per agente utente, poiché sono statici.

## Binari autonomi e immagini Docker basate su Alpine

Le immagini Docker binarie completamente statiche e basate su Alpine (`dunglas/frankenphp:*-alpine`) utilizzano [musl libc](https://musl.libc.org/) invece di [glibc e amici](https://www.etalabs.net/compare_libcs.html), per mantenere una dimensione binaria più piccola.
Ciò potrebbe portare ad alcuni problemi di compatibilità.
In particolare, il flag glob `GLOB_BRACE` [non è disponibile](https://www.php.net/manual/function.glob.php).

Preferire l'utilizzo della variante GNU del binario statico e delle immagini Docker basate su Debian se si riscontrano problemi.

## Uso di `https://127.0.0.1` con Docker

Per impostazione predefinita, FrankenPHP genera un certificato TLS per `localhost`.
È l'opzione più semplice e consigliata per lo sviluppo locale.

Se invece si vuole utilizzare `127.0.0.1` come host, è possibile configurarlo per generare un certificato impostando il nome del server su `127.0.0.1`.

Sfortunatamente, questo non è sufficiente quando si utilizza Docker, a causa del [suo sistema di rete](https://docs.docker.com/network/).
Si riceverà un errore TLS simile a `curl: (35) LibreSSL/3.3.6: error:1404B438:SSL routines:ST_CONNECT:tlsv1 alert internal error`.

Se si usa Linux, una soluzione è utilizzare [il driver di rete host](https://docs.docker.com/network/network-tutorial-host/):

```console
docker run \
    -e SERVER_NAME="127.0.0.1" \
    -v $PWD:/app/public \
    --network host \
    dunglas/frankenphp
```

Il driver di rete host non è supportato su Mac e Windows. Su queste piattaforme si dovrà ricavare l'indirizzo IP del contenitore e includerlo nei nomi dei server.

Eseguire `docker network inspect bridge` e cercare la chiave `Containers` per identificare l'ultimo indirizzo IP attualmente assegnato sotto la chiave `IPv4Address` e incrementarlo di uno. Se non è in esecuzione alcun contenitore, il primo indirizzo IP assegnato è solitamente `172.17.0.2`.

Quindi, includerlo nella variabile di ambiente `SERVER_NAME`:

```console
docker run \
    -e SERVER_NAME="127.0.0.1, 172.17.0.3" \
    -v $PWD:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

> [!CAUTION]
>
> Assicurarsi di sostituire `172.17.0.3` con l'IP che verrà assegnato al contenitore.

Ora dovrebbe essere possibile accedere a `https://127.0.0.1` dal computer host.

In caso contrario, avviare FrankenPHP in modalità debug per cercare di capire il problema:

```console
docker run \
    -e CADDY_GLOBAL_OPTIONS="debug" \
    -e SERVER_NAME="127.0.0.1" \
    -v $PWD:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

## Script Composer che fanno riferimento a `@php`

[Gli script di Composer](https://getcomposer.org/doc/articles/scripts.md) potrebbero voler eseguire un binario PHP per alcune attività, ad es. in [un progetto Laravel](laravel.md) per eseguire `@php artisan package:discover --ansi`. Questo [attualmente fallisce](https://github.com/php/frankenphp/issues/483#issuecomment-1899890915) per due motivi:

- Composer non sa come chiamare il binario FrankenPHP;
- Composer può aggiungere impostazioni PHP utilizzando il flag `-d` nel comando, che FrankenPHP non supporta ancora.

Come soluzione alternativa, possiamo creare uno script di shell in `/usr/local/bin/php` che rimuove i parametri non supportati e quindi chiama FrankenPHP:

```bash
#!/usr/bin/env bash
# /usr/local/bin/php
args=("$@")
index=0
for i in "$@"
do
    if [ "$i" == "-d" ]; then
        unset 'args[$index]'
        unset 'args[$index+1]'
    fi
    index=$((index+1))
done

/usr/local/bin/frankenphp php-cli ${args[@]}
```

Quindi impostare la variabile di ambiente `PHP_BINARY` sul percorso dello script `php` ed eseguire Composer:

```console
export PHP_BINARY=/usr/local/bin/php
composer install
```

## Risoluzione dei problemi TLS/SSL con binari statici

Quando si utilizzano i binari statici, è possibile che si verifichino i seguenti errori relativi a TLS, ad esempio quando si inviano email utilizzando STARTTLS:

```text
Unable to connect with STARTTLS: stream_socket_enable_crypto(): SSL operation failed with code 5. OpenSSL Error messages:
error:80000002:system library::No such file or directory
error:80000002:system library::No such file or directory
error:80000002:system library::No such file or directory
error:0A000086:SSL routines::certificate verify failed
```

Poiché il binario statico non include i certificati TLS, è necessario indirizzare OpenSSL all'installazione dei certificati CA locali.

Esaminare l'output di [`openssl_get_cert_locations()`](https://www.php.net/manual/function.openssl-get-cert-locations.php),
per trovare dove devono essere installati i certificati CA e archiviarli in questa posizione.

> [!WARNING]
>
> I contesti Web e CLI possono avere impostazioni diverse.
> Assicurarsi di eseguire `openssl_get_cert_locations()` nel contesto corretto.

[I certificati CA estratti da Mozilla possono essere scaricati sul sito cURL](https://curl.se/docs/caextract.html).

In alternativa, molte distribuzioni, tra cui Debian, Ubuntu e Alpine, forniscono pacchetti denominati `ca-certificates` che contengono questi certificati.

È anche possibile utilizzare `SSL_CERT_FILE` e `SSL_CERT_DIR` per suggerire a OpenSSL dove cercare i certificati CA:

```console
# Imposta le variabili d'ambiente dei certificati TLS
export SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
export SSL_CERT_DIR=/etc/ssl/certs
```
