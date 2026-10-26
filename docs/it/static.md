---
title: Build statica di FrankenPHP: server app PHP a binario singolo
description: Crea binari FrankenPHP statici o prevalentemente statici con PHP e Caddy incorporati su Linux e macOS, personalizza le estensioni e carica moduli dinamici con glibc.
---

# Creare una build statica

Invece di utilizzare un'installazione locale della libreria PHP,
è possibile creare una build statica o prevalentemente statica di FrankenPHP grazie al fantastico [progetto static-php-cli](https://github.com/crazywhalecc/static-php-cli) (nonostante il nome, questo progetto supporta tutte le SAPI, non solo la CLI).

Con questo metodo, un unico binario portatile conterrà l'interprete PHP, il server web Caddy e FrankenPHP!

Gli eseguibili nativi completamente statici non richiedono alcuna dipendenza e possono anche essere eseguiti su un'[immagine Docker `scratch`](https://docs.docker.com/build/building/base-images/#create-a-minimal-base-image-using-scratch).
Tuttavia, non possono caricare estensioni PHP dinamiche (come Xdebug) e hanno alcune limitazioni, perché utilizzano musl libc.

I binari prevalentemente statici richiedono solo `glibc` e possono caricare estensioni dinamiche.

Quando possibile, consigliamo di utilizzare build basate su glibc, prevalentemente statiche.

FrankenPHP supporta anche [l'incorporamento dell'app PHP nel binario statico](embed.md).

## Linux

Forniamo immagini Docker per creare binari Linux statici:

### Build basata su musl, completamente statica

Per un binario completamente statico che viene eseguito su qualsiasi distribuzione Linux senza dipendenze, ma non supporta il caricamento dinamico delle estensioni:

```console
docker buildx bake --load static-builder-musl
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log dunglas/frankenphp:static-builder-musl
cp "output/frankenphp-linux-$(uname -m)" frankenphp
```

Per prestazioni migliori in scenari fortemente concorrenti, considera l'utilizzo dell'allocatore [mimalloc](https://github.com/microsoft/mimalloc).

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log -e MIMALLOC=1 dunglas/frankenphp:static-builder-musl
```

### Build prevalentemente statica basata su glibc (con supporto per estensioni dinamiche)

Per un binario che supporta il caricamento dinamico delle estensioni PHP, pur mantenendo le estensioni selezionate compilate staticamente:

```console
docker buildx bake --load static-builder-gnu
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log dunglas/frankenphp:static-builder-gnu
cp "output/frankenphp-linux-$(uname -m)" frankenphp
```

Questo binario supporta tutte le versioni di glibc 2.28 e successive, ma non funziona su sistemi basati su musl (come Alpine Linux).

Il binario risultante, prevalentemente statico (eccetto `glibc`), è denominato `frankenphp` ed è disponibile nella cartella corrente.

Se la build fallisce, i log di static-php-cli si trovano in `output/log/`.

Se vuoi creare il binario statico senza Docker, dai un'occhiata alle istruzioni di macOS, che funzionano anche per Linux.

### Estensioni PHP personalizzate nella build statica

Per impostazione predefinita, vengono compilate le estensioni PHP più popolari. I valori predefiniti si trovano in [`craft.yml`](https://github.com/php/frankenphp/blob/main/craft.yml).

Per ridurre la dimensione del binario e ridurre la superficie di attacco, puoi scegliere l'elenco delle estensioni da compilare utilizzando la variabile `PHP_EXTENSIONS`.

Ad esempio, esegui il comando seguente per compilare solo l'estensione `opcache`:

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log -e PHP_EXTENSIONS=opcache,pdo_sqlite dunglas/frankenphp:static-builder-musl
# ...
```

Per aggiungere librerie che abilitano funzionalità aggiuntive alle estensioni che hai abilitato, puoi passare la variabile `PHP_EXTENSION_LIBS`:

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log \
  -e PHP_EXTENSIONS=gd \
  -e PHP_EXTENSION_LIBS=libjpeg,libwebp \
  dunglas/frankenphp:static-builder-musl
```

### Moduli Caddy extra

Per aggiungere moduli Caddy aggiuntivi o passare altri argomenti a [xcaddy](https://github.com/caddyserver/xcaddy), utilizza la variabile `XCADDY_ARGS`:

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log \
  -e XCADDY_ARGS="--with github.com/darkweak/souin/plugins/caddy --with github.com/dunglas/caddy-cbrotli --with github.com/dunglas/mercure/caddy --with github.com/dunglas/vulcain/caddy" \
  dunglas/frankenphp:static-builder-musl
```

In questo esempio, aggiungiamo il modulo cache HTTP [Souin](https://souin.io) per Caddy, nonché i moduli [cbrotli](https://github.com/dunglas/caddy-cbrotli), [Mercure](https://mercure.rocks) e [Vulcain](https://vulcain.rocks).

> [!TIP]
>
> I moduli cbrotli, Mercure e Vulcain sono inclusi per impostazione predefinita se `XCADDY_ARGS` è vuoto o non impostato.
> Se personalizzi il valore di `XCADDY_ARGS`, devi includerli esplicitamente se desideri che siano inclusi.

Scopri anche come [personalizzare la build statica di FrankenPHP](#personalizzare-la-build-statica-di-frankenphp)

### Token GitHub

Se raggiungi il rate limit dell'API GitHub, imposta un token di accesso personale in una variabile di ambiente denominata `GITHUB_TOKEN`:

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log -e GITHUB_TOKEN="xxx" dunglas/frankenphp:static-builder-musl
# ...
```

## macOS

Esegui il seguente script per creare un binario statico per macOS (devi avere [Homebrew](https://brew.sh/) installato):

```console
git clone https://github.com/php/frankenphp
cd frankenphp
./build-static.sh
```

Nota: questo script funziona anche su Linux (e probabilmente su altri Unix) ed è utilizzato internamente dalle immagini Docker che forniamo.

Pubblichiamo binari macOS solo per Apple Silicon. Sui Mac Intel, compila dal codice sorgente con questo script.

## Personalizzare la build statica di FrankenPHP

Le seguenti variabili d'ambiente possono essere passate a `docker run` e allo script `build-static.sh`
per personalizzare la build statica:

- `FRANKENPHP_VERSION`: la versione di FrankenPHP da utilizzare
- `PHP_VERSION`: la versione di PHP da utilizzare
- `PHP_EXTENSIONS`: le estensioni PHP da compilare ([elenco delle estensioni supportate](https://static-php.dev/en/guide/extensions.html))
- `PHP_EXTENSION_LIBS`: librerie extra da creare che aggiungono funzionalità alle estensioni
- `XCADDY_ARGS`: argomenti da passare a [xcaddy](https://github.com/caddyserver/xcaddy), ad esempio per aggiungere moduli Caddy aggiuntivi
- `EMBED`: percorso dell'applicazione PHP da incorporare nel binario
- `CLEAN`: quando impostato, libphp e tutte le sue dipendenze vengono create da zero (nessuna cache)
- `COMPRESS`: quando impostato su `1`, impacchetta il binario risultante con UPX (solo Linux; ignorato quando `DEBUG_SYMBOLS` è impostato)
- `DEBUG_SYMBOLS`: quando impostato, i simboli di debug non verranno rimossi e verranno aggiunti al binario
- `MIMALLOC`: (sperimentale, solo Linux) sostituisce mallocng di musl con [mimalloc](https://github.com/microsoft/mimalloc) per prestazioni migliorate. Ti consigliamo di usarlo solo per build che mirano a musl; per glibc, è preferibile disabilitare questa opzione e usare [`LD_PRELOAD`](https://microsoft.github.io/mimalloc/overrides.html) quando esegui il tuo binario.
- `RELEASE`: (solo manutentori) quando impostato, il binario risultante verrà caricato su GitHub
- `LIBC`: (Solo Linux) `musl` per un binario completamente statico (il predefinito), o `gnu` per collegarsi dinamicamente alla glibc del sistema. È una scorciatoia per `SPC_TARGET` e `SPC_TOOLCHAIN` di static-php-cli, che possono ancora essere impostati direttamente; devono essere vere variabili d'ambiente perché static-php-cli risolve la sua toolchain prima di leggere `craft.yml`. Il valore predefinito `musl` mira alla CPU di base dell'architettura (`x86_64-linux-musl` o `aarch64-linux-musl`) in modo che il binario venga eseguito su qualsiasi CPU; imposta `SPC_TARGET=native-native-musl` per ottimizzare invece per la macchina di build.

## Caricamento dinamico delle estensioni PHP nel binario statico

Con i binari basati su glibc o macOS, è possibile caricare le estensioni PHP in modo dinamico. Tuttavia, queste estensioni dovranno essere compilate con il supporto ZTS.
Poiché la maggior parte dei gestori di pacchetti attualmente non offre versioni ZTS delle proprie estensioni, dovrai compilarle tu stesso.

A tale scopo, puoi eseguire il contenitore Docker `static-builder-gnu`, accedervi in remoto e compilare le estensioni con `./configure --with-php-config=/go/src/app/dist/buildroot/bin/php-config`.

Passaggi di esempio per [l'estensione Xdebug](https://xdebug.org):

```console
docker buildx bake --load static-builder-gnu
docker run --rm -v "$PWD/dist:/go/src/app/dist" -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log dunglas/frankenphp:static-builder-gnu
docker run --rm -it -v "$PWD/dist:/go/src/app/dist" dunglas/frankenphp:static-builder-gnu /bin/bash
cd /go/src/app/dist/buildroot/bin
git clone https://github.com/xdebug/xdebug.git && cd xdebug
../phpize
./configure --with-php-config=/go/src/app/dist/buildroot/bin/php-config
make
exit
cp dist/buildroot/bin/xdebug/modules/xdebug.so xdebug-zts.so
cp "output/frankenphp-linux-$(uname -m)" frankenphp
```

Questo avrà creato `frankenphp` e `xdebug-zts.so` nella cartella corrente.
Se sposti `xdebug-zts.so` nella tua directory delle estensioni, aggiungi `zend_extension=xdebug-zts.so` al tuo php.ini ed esegui FrankenPHP, verrà caricato Xdebug.
