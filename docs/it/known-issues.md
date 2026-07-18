# Problemi noti

## Estensioni PHP non supportate

Le seguenti estensioni non sono compatibili con FrankenPHP:

| Nome | Motivo | Alternative |
| ----------------------------------------------------------------------------------------------------------- | --------------- | ------------------------------------------------------------------------------------------------------------------- |
| [imap](https://www.php.net/manual/imap.installation.php) | Non thread-safe | [javanile/php-imap2](https://github.com/javanile/php-imap2), [webklex/php-imap](https://github.com/Webklex/php-imap) |
| [newrelic](https://docs.newrelic.com/docs/apm/agents/php-agent/getting-started/introduction-new-relic-php/) | Non thread-safe | - |

## Estensioni PHP difettose

Le seguenti estensioni presentano bug noti e comportamenti imprevisti se utilizzate con FrankenPHP:

| Nome | Problema |
| ------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [ext-openssl](https://www.php.net/manual/book.openssl.php) | Quando si utilizza musl libc, l'estensione OpenSSL potrebbe bloccarsi in caso di carichi pesanti. Il problema non si verifica quando si utilizza la più popolare libc GNU. Questo bug è [monitorato da PHP](https://github.com/php/php-src/issues/13648). |

## get_browser

La funzione [get_browser()](https://www.php.net/manual/function.get-browser.php) sembra funzionare male dopo un po'. Una soluzione alternativa consiste nel memorizzare nella cache (ad esempio con [APCu](https://www.php.net/manual/book.apcu.php)) i risultati per agente utente, poiché sono statici.

## Immagini Docker binarie autonome e basate su Alpine

Le immagini Docker binarie completamente statiche e basate su Alpine (`dunglas/frankenphp:*-alpine`) utilizzano [musl libc](https://musl.libc.org/) invece di [glibc e amici](https://www.etalabs.net/compare_libcs.html), per mantenere una dimensione binaria più piccola.
Ciò potrebbe portare ad alcuni problemi di compatibilità.
In particolare, il flag glob `GLOB_BRACE` [non è disponibile](https://www.php.net/manual/function.glob.php)

Se si riscontrano problemi, utilizzare la variante GNU del binario statico e delle immagini Docker basate su Debian.

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

Eseguire `docker network inspect bridge` e trovare la chiave `Containers` per identificare l'ultimo indirizzo IP attualmente assegnato sotto la chiave `IPv4Address` e incrementarlo di uno. Se non è in esecuzione alcun contenitore, il primo indirizzo IP assegnato è solitamente `172.17.0.2`.

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

## Script di composer che fanno riferimento a `@php`

[Gli script di composer](https://getcomposer.org/doc/articles/scripts.md) potrebbero voler eseguire un binario PHP per alcune attività, ad es. in [un progetto Laravel](laravel.md) per eseguire `@php artisan package:discover --ansi`. Questo [attualmente fallisce](https://github.com/php/frankenphp/issues/483#issuecomment-1899890915) per due motivi:

- composer non sa come chiamare il binario FrankenPHP;
- composer può aggiungere impostazioni PHP utilizzando il flag `-d` nel comando, che FrankenPHP non supporta ancora.

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

Quindi impostare la variabile di ambiente `PHP_BINARY` sul percorso dello script `php` ed eseguire composer:

```console
export PHP_BINARY=/usr/local/bin/php
composer install
```

## Risoluzione dei problemi TLS/SSL con file binari statici

Quando si utilizzano i file binari statici, è possibile che si verifichino i seguenti errori relativi a TLS, ad esempio quando si inviano email utilizzando STARTTLS:

```text
Unable to connect with STARTTLS: stream_socket_enable_crypto(): SSL operation failed with code 5. OpenSSL Error messages:
error:80000002:system library::No such file or directory
error:80000002:system library::No such file or directory
error:80000002:system library::No such file or directory
error:0A000086:SSL routines::certificate verify failed
```

Poiché il file binario statico non include i certificati TLS, è necessario indirizzare OpenSSL all'installazione dei certificati della CA locale.

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
