# Symfony

## Eseguire Symfony con Symfony Docker

Per i progetti [Symfony](https://symfony.com), raccomandiamo l'uso di [Symfony Docker](https://github.com/dunglas/symfony-docker), un progetto ufficiale gestito dall'autore di FrankenPHP. Fornisce un ambiente completo basato su Docker con FrankenPHP, automatizzando HTTPS, HTTP/2, HTTP/3 e supporto ai worker.

## Installare Symfony e FrankenPHP in locale

In alternativa, si possono eseguire progetti Symfony con FrankenPHP da una macchina locale:

1. [Installare FrankenPHP](../#getting-started)
2. Aggiungere la configurazione seguente a un file chiamato `Caddyfile` nella cartella principale del progetto Symfony:

   ```caddyfile
   # Caddyfile
   # Il dominio del server
   localhost

   root public/
   php_server {
   	# Opzionale: abilita la modalità worker per prestazioni migliori
   	worker ./public/index.php
   }
   ```

   Si veda la [documentazione sulle prestazioni](performance.md) per altre ottimizzazioni.

3. Lanciare FrankenPHP dalla cartella principale del progetto Symfony: `frankenphp run`

## Modalità worker con Symfony e FrankenPHP

A partire da Symfony 7.4, la modalità worker di FrankenPHP è supportata nativamente.

Per versioni precedenti, installare il pacchetto [PHP Runtime](https://github.com/php-runtime/runtime):

```console
composer require runtime/frankenphp-symfony
```

Lanciare il server, definendo la variabile d'ambiente `APP_RUNTIME` per usare il runtime di FrankenPHP in Symfony:

```console
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php" \
    -e APP_RUNTIME=Runtime\\FrankenPhpSymfony\\Runtime \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

Per approfondire, si veda [la modalità worker](worker.md).

### Verificare la compatibilità dei worker

[Igor PHP](https://github.com/igor-php/igor-php) è un linter statico che analizza progetti Symfony cercando violazioni statiche prima che si verifichino in produzione: servizi senza `ResetInterface`, proprietà con stato che non vengono reimpostate, variabili mutabili locali, chiamate a `exit()` o `die()`, scritture superglobali. Verifica sia il codice dell'applicazione sia i servizi dichiarati in `vendor/`.

```console
composer require --dev igor-php/igor-php
vendor/bin/igor-php .
```

## Ricarica a caldo con Symfony

È abilitato per impostazione predefinita in [Symfony Docker](https://github.com/dunglas/symfony-docker).

Per usare la funzionalità di [ricarica a caldo](hot-reload.md) senza Symfony Docker, abilitare [Mercure](mercure.md) e aggiungere la sotto-direttiva `hot_reload` alla direttiva `php_server` in `Caddyfile`:

```caddyfile
localhost

mercure {
	anonymous
}

root public/
php_server {
	hot_reload
	worker ./public/index.php
}
```

Aggiungere quindi il codice seguente al file `templates/base.html.twig`:

```twig
{# templates/base.html.twig #}
{% if app.request.server.has('FRANKENPHP_HOT_RELOAD') %}
    <meta name="frankenphp-hot-reload:url" content="{{ app.request.server.get('FRANKENPHP_HOT_RELOAD') }}">
    <script src="https://cdn.jsdelivr.net/npm/idiomorph"></script>
    <script src="https://cdn.jsdelivr.net/npm/frankenphp-hot-reload/+esm" type="module"></script>
{% endif %}
```

Infine, eseguire `frankenphp run` dalla cartella principale del progetto Symfony.

## Asset pre-compressi

Il componente [AssetMapper](https://symfony.com/doc/current/frontend/asset_mapper.html) può comprimere gli asset con Brotli e Zstandard durante il deploy. FrankenPHP (attraverso `file_server` di Caddy) può servire direttamente questi file compressi direttamente, evitando la complessità di una compressione al volo.

1. Compilare e comprimere gli asset:

   ```console
   php bin/console asset-map:compile
   ```

2. Aggiornare `Caddyfile` per servire gli asset compressi:

   ```caddyfile
   # Caddyfile
   localhost

   @assets path /assets/*
   file_server @assets {
   	precompressed zstd br gzip
   }

   root public/
   php_server {
   	worker ./public/index.php
   }
   ```

La direttiva `precompressed` dice a Caddy di cercare versioni compresse del file richiesto (come `app.css.zst`, `app.css.br`) e li serve direttamente ai client che li supportano.

## Servire grandi file statici (`X-Sendfile`)

FrankenPHP può servire in modo efficiente [grandi file statici](x-sendfile.md) dopo l'esecuzione di codice PHP (per controllo di accessi, statistiche, eccetera).

Symfony HttpFoundation supporta nativamente la [funzionalità](https://symfony.com/doc/current/components/http_foundation.html#serving-files).
Dopo aver configurato [Caddyfile](x-sendfile.md#configurazione-di-x-accel-redirect-in-caddyfile), troverà automaticamente il valore corretto dell'header `X-Accel-Redirect` da aggiungere alla risposta:

```php
use Symfony\Component\HttpFoundation\BinaryFileResponse;

BinaryFileResponse::trustXSendfileTypeHeader();
$response = new BinaryFileResponse(__DIR__.'/../private-files/file.txt');

// ...
```

## Applicazioni Symfony come binari indipendenti

Usando la funzionalità di [embed di FrankenPHP](embed.md), si possono distribuire applicazioni Symfony
come binari indipendenti.

Seguire questi passi per preparare e impacchettare l'app Symfony:

1. Preparare l'app:

   ```console
   # Esporta il progetto per eliminare .git/, ecc.
   mkdir $TMPDIR/my-prepared-app
   git archive HEAD | tar -x -C $TMPDIR/my-prepared-app
   cd $TMPDIR/my-prepared-app

   # Imposta le variabili d'ambiente corrette
   echo APP_ENV=prod > .env.local
   echo APP_DEBUG=0 >> .env.local

   # Rimuovi i test e gli altri file non necessari per risparmiare spazio
   # In alternativa, aggiungi questi file con l'attributo export-ignore nel file .gitattributes
   rm -Rf tests/

   # Installa le dipendenze
   composer install --ignore-platform-reqs --no-dev -a

   # Ottimizza .env
   composer dump-env prod
   ```

2. Creare un file chiamato `static-build.Dockerfile` nel repository dell'app:

   ```dockerfile
   # static-build.Dockerfile
   FROM --platform=linux/amd64 dunglas/frankenphp:static-builder-gnu
   # Se si intende eseguire il binario su sistemi musl-libc, usare invece static-builder-musl

   # Copiare l'applicazione
   WORKDIR /go/src/app/dist/app
   COPY . .

   # Compila il binario statico
   WORKDIR /go/src/app/
   RUN EMBED=dist/app/ ./build-static.sh
   ```

   > [!CAUTION]
   >
   > Alcuni file `.dockerignore`
   > ignorano la cartella `vendor/` e i file `.env`. Assicurarsi di modificare o rimuovere il file `.dockerignore` prima della compilazione.

3. Build:

   ```console
   docker build -t static-symfony-app -f static-build.Dockerfile .
   ```

4. Estrarre il file binario:

   ```console
   docker cp $(docker create --name static-symfony-app-tmp static-symfony-app):/go/src/app/dist/frankenphp-linux-x86_64 my-app ; docker rm static-symfony-app-tmp
   ```

5. Avviare il server:

   ```console
   ./my-app php-server
   ```

Ulteriori informazioni sulle opzioni disponibili e su come creare file binari per altri sistemi operativi si possono trovare nella documentazione sull'[embed di applicazioni](embed.md).
