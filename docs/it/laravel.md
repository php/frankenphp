# Laravel

## Esecuzione di Laravel con l'immagine Docker di FrankenPHP

Servire un'applicazione web [Laravel](https://laravel.com) con FrankenPHP è facile come montare il progetto nella cartella `/app` dell'immagine Docker ufficiale.

Eseguire questo comando dalla cartella principale dell'app Laravel:

```console
docker run -p 80:80 -p 443:443 -p 443:443/udp -v $PWD:/app dunglas/frankenphp
```

Buon divertimento!

## Installazione di Laravel con FrankenPHP in locale

In alternativa, si possono eseguire progetti Laravel con FrankenPHP dal computer locale:

1. [Scaricare il binario corrispondente al sistema](../#standalone-binary)
2. Aggiungere la seguente configurazione a un file denominato `Caddyfile` nella cartella root del progetto Laravel:

   ```caddyfile
   # Caddyfile
   {
   	frankenphp
   }

   # Nome dominio del server
   localhost {
   	# Imposta la webroot alla cartella public/ 
   	root public/
   	# Abilita la compressione (facoltativo)
   	encode zstd br gzip
   	# Esegue i file PHP e serve i file da public/ 
   	php_server {
   		try_files {path} index.php
   	}
   }
   ```

3. Avviare FrankenPHP dalla cartella principale del progetto Laravel: `frankenphp run`

## Laravel Octane

Octane può essere installato tramite il gestore pacchetti Composer:

```console
composer require laravel/octane
```

Dopo aver installato Octane, si può eseguire il comando artisan `octane:install`, che installerà il file di configurazione di Octane nell'applicazione:

```console
php artisan octane:install --server=frankenphp
```

Il server Octane può essere avviato tramite il comando artisan `octane:frankenphp`.

```console
php artisan octane:frankenphp
```

Il comando `octane:frankenphp` accetta le seguenti opzioni:

- `--host`: l'indirizzo IP a cui il server deve associarsi (impostazione predefinita: `127.0.0.1`)
- `--port`: la porta su cui deve essere disponibile il server (impostazione predefinita: `8000`)
- `--admin-port`: la porta su cui deve essere disponibile il server di amministrazione (impostazione predefinita: `2019`)
- `--workers`: il numero di worker che dovrebbero essere disponibili per gestire le richieste (impostazione predefinita: `auto`)
- `--max-requests`: il numero di richieste da elaborare prima di ricaricare il server (impostazione predefinita: `500`)
- `--caddyfile`: il percorso del file FrankenPHP `Caddyfile` (impostazione predefinita: [stubbed `Caddyfile` in Laravel Octane](https://github.com/laravel/octane/blob/2.x/src/Commands/stubs/Caddyfile))
- `--https`: abilita HTTPS, HTTP/2 e HTTP/3 e genera e rinnova automaticamente i certificati
- `--http-redirect`: abilita il reindirizzamento da HTTP a HTTPS (abilitato solo se viene passato --https)
- `--watch`: ricarica automaticamente il server quando l'applicazione viene modificata
- `--poll`: utilizza il polling del file system durante la visione per osservare i file su una rete
- `--log-level`: registra i messaggi al livello di log specificato o superiore, utilizzando il logger Caddy nativo

> [!TIP]
> Per ottenere log JSON strutturati (utili quando si utilizzano soluzioni di analisi dei log), passare esplicitamente l'opzione `--log-level`.

Vedi anche [come utilizzare Mercure con Octane](#supporto-mercure).

Saperne di più su [Laravel Octane nella documentazione ufficiale](https://laravel.com/docs/octane).

## App Laravel come file binari autonomi

Utilizzando la [funzione di incorporamento dell'applicazione di FrankenPHP](embed.md), è possibile distribuire Laravel
app come file binari autonomi.

Seguire questi passaggi per creare il pacchetto dell'app Laravel come binario autonomo per Linux:

1. Creare un file chiamato `static-build.Dockerfile` nel repository dell'app:

   ```dockerfile
   # static-build.Dockerfile
   FROM --platform=linux/amd64 dunglas/frankenphp:static-builder-gnu
   # Se si vuole eseguire il binario su sistemi musl-libc, usare invece static-builder-musl

   # Copia l'app
   WORKDIR /go/src/app/dist/app
   COPY . .

   # Rimuove i test e altri file superflui, per risparmiare spazio
   # Si possono anche aggiungere tali file a .dockerignore
   RUN rm -Rf tests/

   # Copia il file .env
   RUN cp .env.example .env
   # Cambia APP_ENV e APP_DEBUG per la produzione
   RUN sed -i'' -e 's/^APP_ENV=.*/APP_ENV=production/' -e 's/^APP_DEBUG=.*/APP_DEBUG=false/' .env

   # Altre modifiche al file .env, se necessario

   # Installa le dipendenze
   RUN composer install --ignore-platform-reqs --no-dev -a

   # Costruisce il binario statico
   WORKDIR /go/src/app/
   RUN EMBED=dist/app/ ./build-static.sh
   ```

   > [!CAUTION]
   >
   > Alcuni file `.dockerignore`
   > ignorano la cartella `vendor/` e i file `.env`. Assicurarsi di modificare o rimuovere il file `.dockerignore` prima della compilazione.

2. Build:

   ```console
   docker build -t static-laravel-app -f static-build.Dockerfile .
   ```

3. Estrarre il file binario:

   ```console
   docker cp $(docker create --name static-laravel-app-tmp static-laravel-app):/go/src/app/dist/frankenphp-linux-x86_64 frankenphp ; docker rm static-laravel-app-tmp
   ```

4. Popolare le cache:

   ```console
   frankenphp php-cli artisan optimize
   ```

5. Eseguire le migrazioni del database (se presenti):

   ```console
   frankenphp php-cli artisan migrate
   ```

6. Generare la chiave segreta dell'app:

   ```console
   frankenphp php-cli artisan key:generate
   ```

7. Avviare il server:

   ```console
   frankenphp php-server
   ```

La nuova app è ora pronta!

Ulteriori informazioni sulle opzioni disponibili e su come creare file binari per altri sistemi operativi si possono trovare nella documentazione sull'[embed di applicazioni](embed.md).

### Modifica del percorso di archiviazione

Per impostazione predefinita, Laravel archivia file, cache, log, ecc. caricati nella cartella `storage/` dell'applicazione.
Questo non è adatto per le applicazioni integrate, poiché ogni nuova versione verrà estratta in una cartella temporanea diversa.

Imposta la variabile di ambiente `LARAVEL_STORAGE_PATH` (ad esempio, nel file `.env`) o chiama il metodo `Illuminate\Foundation\Application::useStoragePath()` per utilizzare una cartella esterna a quella temporanea.

### Supporto Mercure

[Mercure](https://mercure.rocks) è un ottimo modo per aggiungere funzionalità di real time alle app Laravel.
FrankenPHP include [supporto Mercure pronto all'uso](mercure.md).

Se non utilizzi [Octane](#laravel-octane), consulta [la voce della documentazione Mercure](mercure.md).

Se si usa Octane, si può abilitare il supporto Mercure aggiungendo le seguenti righe al file `config/octane.php`:

```php
// config/octane.php
// ...

return [
    // ...

    'mercure' => [
        'anonymous' => true,
        'publisher_jwt' => '!ChangeThisMercureHubJWTSecretKey!',
        'subscriber_jwt' => '!ChangeThisMercureHubJWTSecretKey!',
    ],
];
```

È possibile utilizzare [tutte le direttive supportate da Mercure](https://mercure.rocks/docs/hub/config#directives) in questo array.

Per pubblicare e sottoscrivere gli aggiornamenti, si consiglia di utilizzare la libreria [Laravel Mercure Broadcaster](https://github.com/mvanduijker/laravel-mercure-broadcaster).
In alternativa, consultare [la documentazione Mercure](mercure.md) per farlo in puro PHP e JavaScript.

### Esecuzione di Octane con file binari autonomi

È anche possibile impacchettare le app Laravel Octane come file binari autonomi!

Per fare ciò, [installa Octane correttamente](#laravel-octane) e segui i passaggi descritti nella [sezione sulle app Laravel come binari autonomi](#app-laravel-come-binari-autonomi).

Quindi, per avviare FrankenPHP in modalità worker tramite Octane, eseguire:

```console
PATH="$PWD:$PATH" frankenphp php-cli artisan octane:frankenphp
```

> [!CAUTION]
>
> Affinché il comando funzioni, il file binario autonomo **deve** chiamarsi `frankenphp`,
> perché Octane necessita di un programma chiamato `frankenphp` disponibile nel percorso.
