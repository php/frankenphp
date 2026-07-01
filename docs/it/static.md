# Creare una build statica

Invece di utilizzare un'installazione locale della libreria PHP,
è possibile creare una build statica o prevalentemente statica di FrankenPHP grazie al fantastico [progetto static-php-cli](https://github.com/crazywhalecc/static-php-cli) (nonostante il nome, questo progetto supporta tutte le SAPI, non solo la CLI).

Con questo metodo, un unico file binario portatile conterrà l'interprete PHP, il server web Caddy e FrankenPHP!

Gli eseguibili nativi completamente statici non richiedono alcuna dipendenza e possono anche essere eseguiti su un'[immagine Docker `scratch`](https://docs.docker.com/build/building/base-images/#create-a-minimal-base-image-using-scratch).
Tuttavia, non possono caricare estensioni PHP dinamiche (come Xdebug) e hanno alcune limitazioni, perché utilizzano musl libc.

Per lo più i binari statici richiedono solo `glibc` e possono caricare estensioni dinamiche.

Quando possibile, consigliamo di utilizzare build basate su glibc, per lo più statiche.

FrankenPHP supporta anche [l'incorporamento dell'app PHP nel file binario statico](embed.md).

## Linux

Forniamo immagini Docker per creare binari Linux statici:

### Build basata su Musl, completamente statica

Per un file binario completamente statico che viene eseguito su qualsiasi distribuzione Linux senza dipendenze, ma non supporta il caricamento dinamico delle estensioni:

```console
docker buildx bake --load static-builder-musl
docker cp $(docker create --name static-builder-musl dunglas/frankenphp:static-builder-musl):/go/src/app/dist/frankenphp-linux-$(uname -m) frankenphp ; docker rm static-builder-musl
```

Per prestazioni migliori in scenari fortemente simultanei, si prenda in considerazione l'utilizzo dell'allocatore [mimalloc](https://github.com/microsoft/mimalloc).

```console
docker buildx bake --load --set static-builder-musl.args.MIMALLOC=1 static-builder-musl
```

### Build per lo più statica basata su glibc (con supporto per estensioni dinamiche)

Per un binario che supporta il caricamento dinamico delle estensioni PHP, pur mantenendo le estensioni selezionate compilate staticamente:

```console
docker buildx bake --load static-builder-gnu
docker cp $(docker create --name static-builder-gnu dunglas/frankenphp:static-builder-gnu):/go/src/app/dist/frankenphp-linux-$(uname -m) frankenphp ; docker rm static-builder-gnu
```

Questo binario supporta tutte le versioni di glibc 2.17 e successive, ma non funziona su sistemi basati su Musl (come Alpine Linux).

Il file binario risultante, per lo più statico (eccetto `glibc`), è denominato `frankenphp` ed è disponibile nella cartella corrente.

Se vuoi creare il binario statico senza Docker, dai un'occhiata alle istruzioni di macOS, che funzionano anche per Linux.

### Estensioni PHP personalizzate nella build statica

Per impostazione predefinita, vengono compilate le estensioni PHP più popolari.

Per ridurre la dimensione del binario e ridurre la superficie di attacco, è possibile scegliere l'elenco delle estensioni da includere, utilizzando `PHP_EXTENSIONS` come Docker ARG.

Ad esempio, eseguire il comando seguente per creare solo l'estensione `opcache`:

```console
docker buildx bake --load --set static-builder-musl.args.PHP_EXTENSIONS=opcache,pdo_sqlite static-builder-musl
# ...
```

Per aggiungere librerie che abilitano funzionalità aggiuntive alle estensioni abilitate, si può passare `PHP_EXTENSION_LIBS` come Docker ARG:

```console
docker buildx bake \
  --load \
  --set static-builder-musl.args.PHP_EXTENSIONS=gd \
  --set static-builder-musl.args.PHP_EXTENSION_LIBS=libjpeg,libwebp \
  static-builder-musl
```

### Moduli Caddy extra

Per aggiungere moduli Caddy aggiuntivi o passare altri argomenti a [xcaddy](https://github.com/caddyserver/xcaddy), utilizzare `XCADDY_ARGS` Docker ARG:

```console
docker buildx bake \
  --load \
  --set static-builder-musl.args.XCADDY_ARGS="--with github.com/darkweak/souin/plugins/caddy --with github.com/dunglas/caddy-cbrotli --with github.com/dunglas/mercure/caddy --with github.com/dunglas/vulcain/caddy" \
  static-builder-musl
```

In questo esempio, aggiungiamo il modulo cache HTTP [Souin](https://souin.io) per Caddy nonché i moduli [cbrotli](https://github.com/dunglas/caddy-cbrotli), [Mercure](https://mercure.rocks) e [Vulcain](https://vulcain.rocks).

> [!TIP]
>
> I moduli cbrotli, Mercure e Vulcain sono inclusi per impostazione predefinita se `XCADDY_ARGS` è vuoto o non impostato.
> Se si personalizza il valore di `XCADDY_ARGS`, vanno inclusi esplicitamente.

Scopri anche come [personalizzare la build statica di FrankenPHP](#personalizzare-la-build-statica-di-frankenphp)

### Token GitHub

Se si raggiunge il rate limit dell'API GitHub, occorre impostare un token di accesso personale in una variabile di ambiente denominata `GITHUB_TOKEN`:

```console
GITHUB_TOKEN="xxx" docker --load buildx bake static-builder-musl
# ...
```

## macOS

Eseguire il seguente script per creare un file binario statico per macOS (serve [Homebrew](https://brew.sh/) installato):

```console
git clone https://github.com/php/frankenphp
cd frankenphp
./build-static.sh
```

Nota: questo script funziona anche su Linux (e probabilmente su altri Unix) ed è utilizzato internamente dalle immagini Docker che forniamo.

## Personalizzare la build statica di FrankenPHP

Le seguenti variabili d'ambiente possono essere passate a `docker build` e a `build-static.sh`
per personalizzare la build statica:

- `FRANKENPHP_VERSION`: la versione di FrankenPHP da utilizzare
- `PHP_VERSION`: la versione di PHP da utilizzare
- `PHP_EXTENSIONS`: le estensioni PHP da compilare ([elenco delle estensioni supportate](https://static-php.dev/en/guide/extensions.html))
- `PHP_EXTENSION_LIBS`: librerie extra da creare che aggiungono funzionalità alle estensioni
- `XCADDY_ARGS`: argomenti da passare a [xcaddy](https://github.com/caddyserver/xcaddy), ad esempio per aggiungere moduli Caddy aggiuntivi
- `EMBED`: percorso dell'applicazione PHP da incorporare nel binario
- `CLEAN`: quando impostato, libphp e tutte le sue dipendenze vengono create da zero (nessuna cache)
- `NO_COMPRESS`: non comprimere il file binario risultante utilizzando UPX
- `DEBUG_SYMBOLS`: quando impostato, i simboli di debug non verranno rimossi e verranno aggiunti al file binario
- `MIMALLOC`: (sperimentale, solo Linux) sostituisce mallocng di musl con [mimalloc](https://github.com/microsoft/mimalloc) per prestazioni migliorate. Ti consigliamo di utilizzarlo solo per build con targeting Musl, poiché glibc preferisce disabilitare questa opzione e utilizzare invece [`LD_PRELOAD`](https://microsoft.github.io/mimalloc/overrides.html) quando si esegue il codice binario.
- `RELEASE`: (solo manutentori) quando impostato, il binario risultante verrà caricato su GitHub

## Caricamento dinamico delle estensioni PHP nel file binario statico

Con i binari basati su glibc o macOS, si possono caricare le estensioni PHP in modo dinamico. Tuttavia, queste estensioni dovranno essere compilate con il supporto ZTS.
Poiché la maggior parte dei gestori di pacchetti attualmente non offre versioni ZTS delle proprie estensioni, andranno compilate a mano.

A tale scopo, è possibile creare ed eseguire il contenitore Docker `static-builder-gnu`, accedervi in ​​remoto e compilare le estensioni con `./configure --with-php-config=/go/src/app/dist/static-php-cli/buildroot/bin/php-config`.

Passaggi di esempio per [l'estensione Xdebug](https://xdebug.org):

```console
docker build -t gnu-ext -f static-builder-gnu.Dockerfile --build-arg FRANKENPHP_VERSION=1.0 .
docker create --name static-builder-gnu -it gnu-ext /bin/sh
docker start static-builder-gnu
docker exec -it static-builder-gnu /bin/sh
cd /go/src/app/dist/static-php-cli/buildroot/bin
git clone https://github.com/xdebug/xdebug.git && cd xdebug
source scl_source enable devtoolset-10
../phpize
./configure --with-php-config=/go/src/app/dist/static-php-cli/buildroot/bin/php-config
make
exit
docker cp static-builder-gnu:/go/src/app/dist/static-php-cli/buildroot/bin/xdebug/modules/xdebug.so xdebug-zts.so
docker cp static-builder-gnu:/go/src/app/dist/frankenphp-linux-$(uname -m) ./frankenphp
docker stop static-builder-gnu
docker rm static-builder-gnu
docker rmi gnu-ext
```

Questo creerà `frankenphp` e `xdebug-zts.so` nella cartella corrente.
Spostando `xdebug-zts.so` nella cartella dell'estensione, aggiungendo `zend_extension=xdebug-zts.so` a php.ini ed eseguendo FrankenPHP, caricherà Xdebug.
