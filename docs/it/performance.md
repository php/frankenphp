# Prestazioni

Per impostazione predefinita, FrankenPHP cerca di offrire un buon compromesso tra prestazioni e facilità d'uso.
Tuttavia, è possibile migliorare sostanzialmente le prestazioni utilizzando una configurazione appropriata.

## Ottimizzazione dei thread e dei worker

Per impostazione predefinita, FrankenPHP avvia un numero di thread e worker due volte superiore (in modalità worker) rispetto al numero disponibile di core della CPU.

I valori appropriati dipendono fortemente da come è scritta l'applicazione, da cosa fa e dall'hardware.
Consigliamo vivamente di modificare questi valori. Per la migliore stabilità del sistema, si consiglia di avere `num_threads` x `memory_limit` < `available_memory`.

Per trovare i valori corretti, è meglio eseguire test di carico simulando il traffico reale.
[k6](https://k6.io) e [Gatling](https://gatling.io) sono ottimi strumenti per questo.

Per configurare il numero di thread, utilizzare l'opzione `num_threads` delle direttive `php_server` e `php`.
Per modificare il numero di worker, utilizzare l'opzione `num` della sezione `worker` della direttiva `frankenphp`.

### `max_threads`

Sebbene sia sempre meglio sapere esattamente come sarà il traffico, le applicazioni nella vita reale tendono ad essere più
imprevedibili. La [configurazione](config.md#caddyfile-config) `max_threads` consente a FrankenPHP di generare automaticamente thread aggiuntivi in ​​fase di esecuzione fino al limite specificato.
`max_threads` può aiutare a capire quanti thread sono necessari per gestire il traffico e può rendere il server più resistente ai picchi di latenza.
Se impostato su `auto`, il limite verrà stimato in base a `memory_limit` in `php.ini`. Se non è in grado di farlo,
`auto` verrà invece impostato per impostazione predefinita sul doppio di `num_threads`. Si tenga presente che `auto` potrebbe sottostimare fortemente il numero di thread necessari.
`max_threads` è simile a [pm.max_children](https://www.php.net/manual/install.fpm.configuration.php#pm.max-children) di PHP FPM. La differenza principale è che FrankenPHP utilizza i thread invece di
processi e li delega automaticamente tra diversi worker e "modalità classica" in base alle esigenze.

## Modalità worker per una velocità maggiore

L'abilitazione della [modalità worker](worker.md) migliora notevolmente le prestazioni,
ma l'app deve essere adattata per essere compatibile con questa modalità:
è necessario creare un worker ed essere sicuri che l'app non abbia leak di memoria.

## Evitare musl in produzione: meglio le build glibc

La variante Alpine Linux delle immagini Docker ufficiali e i file binari predefiniti che forniamo utilizzano [musl libc](https://musl.libc.org).

PHP è noto per essere [più lento](https://gitlab.alpinelinux.org/alpine/aports/-/issues/14381) quando si utilizza questa libreria C alternativa invece della tradizionale libreria GNU,
soprattutto se compilato in modalità ZTS (thread-safe), richiesta per FrankenPHP. La differenza può essere significativa in un ambiente con numerosi thread.

Inoltre, [alcuni bug si verificano solo quando si utilizza musl](https://github.com/php/php-src/issues?q=sort%3Aupdated-desc+is%3Aissue+is%3Aopen+label%3ABug+musl).

In ambienti di produzione, consigliamo di utilizzare FrankenPHP collegato a glibc, compilato con un livello di ottimizzazione appropriato.

Ciò può essere ottenuto utilizzando le immagini Debian Docker, utilizzando [i pacchetti .deb, .rpm o .apk dei nostri manutentori](https://pkgs.henderkes.com) o [compilando FrankenPHP dai sorgenti](compile.md).

Per contenitori più snelli o più sicuri, si potrebbe prendere in considerazione [un'immagine Debian rafforzata](docker.md#hardening-images) anziché Alpine.

## Configurazione runtime di Go per FrankenPHP

FrankenPHP è scritto in Go.

In generale, il runtime Go non richiede alcuna configurazione speciale, ma in determinate circostanze,
la configurazione specifica migliora le prestazioni.

Probabilmente si vorrà impostare la variabile di ambiente `GODEBUG` su `cgocheck=0` (l'impostazione predefinita nelle immagini Docker FrankenPHP).

Se si esegue FrankenPHP in contenitori (Docker, Kubernetes, LXC...) e la memoria disponibile per i contenitori è limitata,
impostare la variabile di ambiente `GOMEMLIMIT` sulla quantità di memoria disponibile.

Per ulteriori dettagli, fare riferimento alle [variabili di ambiente runtime Go](https://pkg.go.dev/runtime#hdr-Environment_Variables) per ottenere il massimo dal runtime.

## `file_server`

Per impostazione predefinita, la direttiva `php_server` configura automaticamente un file server su
servire file statici (risorse) archiviati nella cartella principale.

Questa funzionalità è conveniente, ma ha un costo.
Per disabilitarlo, utilizzare la seguente configurazione:

```caddyfile
php_server {
    file_server off
}
```

## `try_files`

Oltre ai file statici e ai file PHP, `php_server` proverà anche a servire l'indice dell'applicazione
e file di indice della cartella (`/path/` -> `/path/index.php`). Se non si ha bisogno degli indici di cartelle,
si possono disabilitare definendo esplicitamente `try_files` in questo modo:

```caddyfile
php_server {
    try_files {path} index.php
    root /root/to/your/app # aggiungere esplicitamente la root qui consente una cache migliore
}
```

Ciò può ridurre significativamente il numero di operazioni sui file non necessarie.
Un worker equivalente della configurazione precedente sarebbe:

```caddyfile
route {
    php_server { # usare "php" invece di "php_server" se non si necessita del file server
        root /root/to/your/app
        worker /path/to/worker.php {
            match * # invia tutte le richieste direttamente al worker
        }
    }
}
```

Un approccio alternativo con zero operazioni non necessarie sul file system sarebbe invece quello di utilizzare la direttiva `php` e dividere
file da PHP per percorso. Questo approccio funziona bene se l'intera applicazione è servita da un unico file di ingresso.
Una [configurazione](config.md#caddyfile-config) di esempio che serve file statici dietro una cartella `/assets` potrebbe assomigliare a questa:

```caddyfile
# Caddyfile: separa risorse statiche e richieste PHP per evitare ricerche sul filesystem
route {
    @assets {
        path /assets/*
    }

    # tutto ciò che è sotto /assets viene gestito dal file server
    file_server @assets {
        root /root/to/your/app
    }

    # tutto ciò che non è in /assets viene gestito dal file index o worker PHP
    rewrite index.php
    php {
        root /root/to/your/app # aggiungere esplicitamente la root qui consente una cache migliore
    }
}
```

## Evitare i segnaposto Caddyfile nei percorsi attivi

È possibile utilizzare [segnaposto](https://caddyserver.com/docs/conventions#placeholders) nelle direttive `root` e `env`.
Tuttavia, ciò impedisce la memorizzazione nella cache di questi valori e comporta un costo significativo in termini di prestazioni.

Se possibile, evitare i segnaposto in queste direttive.

## `resolve_root_symlink`

Per impostazione predefinita, se la document root è un collegamento simbolico, viene automaticamente risolto da FrankenPHP (questo è necessario affinché PHP funzioni correttamente).
Se la document root non è un collegamento simbolico, si può disabilitare questa funzione.

```caddyfile
php_server {
    resolve_root_symlink false
}
```

Ciò migliorerà le prestazioni se la direttiva `root` contiene [segnaposto](https://caddyserver.com/docs/conventions#placeholders).
Negli altri casi il guadagno sarà trascurabile.

## Prestazioni dei log

I log sono molto utili, ma, per definizione,
richiedono operazioni di I/O e allocazioni di memoria, il che riduce notevolmente le prestazioni.
Assicurarsi di [impostare il livello di log](https://caddyserver.com/docs/caddyfile/options#log) correttamente,
per salvare solo ciò che è necessario.

## Ottimizzazione delle prestazioni PHP per FrankenPHP

FrankenPHP utilizza l'interprete PHP ufficiale.
Tutte le consuete ottimizzazioni delle prestazioni relative a PHP si applicano a FrankenPHP.

In particolare:

- controllare che [OPcache](https://www.php.net/manual/book.opcache.php) sia installato, abilitato e configurato correttamente
- attivare le [ottimizzazioni del caricatore automatico Composer](https://getcomposer.org/doc/articles/autoloader-optimization.md)
- assicurarsi che la cache di `realpath` sia sufficientemente grande per le esigenze dell'applicazione
- utilizzare il [preloading](https://www.php.net/manual/opcache.preloading.php)

Per maggiori dettagli, leggere [la documentazione sull'ottimizzazione delle prestazioni di Symfony](https://symfony.com/doc/current/performance.html)
(la maggior parte dei suggerimenti sono utili anche senza utilizzare Symfony).

## Suddivisione del pool di thread FrankenPHP per endpoint lenti

È normale che le applicazioni interagiscano con servizi esterni lenti, come un'API
che tende a essere inaffidabile in condizioni di carico elevato o che impiega costantemente più di 10 secondi per rispondere.
In questi casi, può essere utile dividere il pool di thread per avere pool "lenti" dedicati.
Ciò impedisce agli endpoint lenti di consumare tutte le risorse/thread del server e
limita la concorrenza delle richieste destinate all'endpoint lento, in modo simile a un
pool di connessione.

```caddyfile
# Caddyfile: pool di thread FrankenPHP dedicato agli endpoint lenti
example.com {
    php_server {
        root /app/public # la root dell'applicazione
        worker index.php {
            match /slow-endpoint/* # tutte le richieste con percorso /slow-endpoint/* sono gestite da questo pool di thread
            num 1 # minimo 1 thread per le richieste che corrispondono a /slow-endpoint/*
            max_threads 20 # consente fino a 20 thread per le richieste che corrispondono a /slow-endpoint/*, se necessario
        }
        worker index.php {
            match * # tutte le altre richieste sono gestite separatamente
            num 1 # minimo 1 thread per le altre richieste, anche se gli endpoint lenti iniziano a bloccarsi
            max_threads 20 # consente fino a 20 thread per le altre richieste, se necessario
        }
    }
}
```

In genere è consigliabile gestire anche gli endpoint molto lenti in modo asincrono, utilizzando meccanismi pertinenti come le code di messaggi.
