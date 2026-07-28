# App PHP come file binari autonomi

FrankenPHP ha la capacità di incorporare il codice sorgente e le risorse delle applicazioni PHP in un binario statico e autonomo.

Grazie a questa funzionalità, le applicazioni PHP possono essere distribuite come binari autonomi che includono l'applicazione stessa, l'interprete PHP e Caddy, un server web pronto per la produzione.

Scopri di più su questa funzionalità [nella presentazione fatta da Kévin alla SymfonyCon 2023](https://dunglas.dev/2023/12/php-and-symfony-apps-as-standalone-binaries/).

Per incorporare applicazioni Laravel, [leggi questa specifica voce della documentazione](laravel.md#app-laravel-come-binari-autonomi).

## Preparazione dell'app

Prima di creare il file binario autonomo, assicurarsi che l'app sia pronta per l'embed.

Ad esempio, probabilmente si vorrà:

- Installare le dipendenze di produzione dell'app
- Fare un dump dell'autoloader
- Abilitare la modalità di produzione dell'applicazione (se presente)
- Eliminare i file non necessari, come `.git` o i test per ridurre la dimensione del file binario finale

Ad esempio, per un'app Symfony, si può utilizzare i seguenti comandi:

```console
# Esporta il progetto per togliere .git/ e altri file
mkdir $TMPDIR/my-prepared-app
git archive HEAD | tar -x -C $TMPDIR/my-prepared-app
cd $TMPDIR/my-prepared-app

# Imposta le variabili d'ambiente adeguate
echo APP_ENV=prod > .env.local
echo APP_DEBUG=0 >> .env.local

# Rimuove i test e altri file superflui, per salvare spazio
# In alternativa, aggiungere i percorsi all'attributo export-ignore dentro al file .gitattributes
rm -Rf tests/

# Installa le dipendenze
composer install --ignore-platform-reqs --no-dev -a

# Ottimizza .env
composer dump-env prod
```

### Personalizzazione della configurazione

Per personalizzare [la configurazione](config.md), si possono inserire un file `Caddyfile` e un file `php.ini`
nella cartella principale dell'app per l'embed (`$TMPDIR/my-prepared-app` nell'esempio precedente).

## Creazione di un binario Linux

Il modo più semplice per creare un binario Linux è utilizzare il builder fornito, basato su Docker.

1. Creare un file denominato `static-build.Dockerfile` nel repository dell'app:

   ```dockerfile
   # static-build.Dockerfile
   FROM --platform=linux/amd64 dunglas/frankenphp:static-builder-gnu
   # Se si vuole eseguire il binario su sistemi musl-libc, usare invece static-builder-musl

   # Copia l'app
   WORKDIR /go/src/app/dist/app
   COPY . .

   # Costruisce il binario statico
   WORKDIR /go/src/app/
   RUN EMBED=dist/app/ ./build-static.sh
   ```

   > [!CAUTION]
   >
   > Alcuni file `.dockerignore` (ad esempio quello di [Symfony](https://github.com/dunglas/symfony-docker/blob/main/.dockerignore)
   > ignorano la cartella `vendor/` e i file `.env`. Assicurarsi di modificare o rimuovere il file `.dockerignore` prima della build.

2. Build:

   ```console
   docker build -t static-app -f static-build.Dockerfile .
   ```

3. Estrazione del file binario:

   ```console
   docker cp $(docker create --name static-app-tmp static-app):/go/src/app/dist/frankenphp-linux-x86_64 my-app ; docker rm static-app-tmp
   ```

Il file binario risultante è il file denominato `my-app` nella cartella corrente.

## Creazione di un binario per altri sistemi operativi

Se non si vuole usare Docker o si vuole creare un file binario macOS, usare lo script fornito:

```console
git clone https://github.com/php/frankenphp
cd frankenphp
EMBED=/path/to/your/app ./build-static.sh
```

Il file binario risultante è il file denominato `frankenphp-<os>-<arch>` nella cartella `dist/`.

## Uso del binario

Questo è tutto! Il file `my-app` (o `dist/frankenphp-<os>-<arch>` su altri sistemi operativi) contiene l'app autonoma!

Per avviare l'esecuzione dell'app Web:

```console
./my-app php-server
```

Se l'app contiene uno [worker script](worker.md), avvia il worker con qualcosa come:

```console
./my-app php-server --worker public/index.php
```

Per abilitare HTTPS (viene creato automaticamente un certificato Let's Encrypt), HTTP/2 e HTTP/3, specificare il nome di dominio da usare:

```console
./my-app php-server --domain localhost
```

Si possono anche eseguire gli script CLI PHP inclusi nel binario:

```console
./my-app php-cli bin/console
```

## Estensioni PHP

Per impostazione predefinita, lo script creerà le estensioni richieste dal file `composer.json` del progetto, se presente.
Se il file `composer.json` non esiste, vengono create le estensioni predefinite, come documentato nella [voce sulle build statiche](static.md).

Per personalizzare le estensioni, si può usare la variabile di ambiente `PHP_EXTENSIONS`.

## Personalizzazione della build

[Leggere la documentazione sulla build statica](static.md) per vedere come personalizzare il file binario (estensioni, versione PHP...).

## Distribuire il binario

Su Linux, il file binario creato viene compresso con [UPX](https://upx.github.io).

Su Mac, per ridurre la dimensione del file prima di inviarlo, lo si può comprimere.
Consigliamo di usare `xz`.
