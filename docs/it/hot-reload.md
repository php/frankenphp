# Ricarica a caldo

FrankenPHP include una funzionalità di **ricarica a caldo** integrata, progettata per migliorare notevolmente l'esperienza degli sviluppatori.

![Ricarica a caldo](../hot-reload.png)

Questa funzionalità fornisce un flusso di lavoro simile alla **Sostituzione calda del modulo (HMR)** nei moderni strumenti JavaScript come Vite o webpack.
Invece di aggiornare manualmente il browser dopo ogni modifica di file (codice PHP, modelli, file JavaScript e CSS...),
FrankenPHP aggiorna il contenuto della pagina in tempo reale.

Funziona nativamente con WordPress, Laravel, Symfony e qualsiasi altra applicazione o framework PHP.

Quando abilitato, FrankenPHP controlla la cartella di lavoro corrente per eventuali modifiche al filesystem.
Quando un file viene modificato, invia un aggiornamento [Mercure](mercure.md) al browser.

A seconda della configurazione, il browser:

- **Modifica il DOM** (preservando la posizione di scorrimento e lo stato di input) se viene caricato [Idiomorph](https://github.com/bigskysoftware/idiomorph).
- **Ricarica la pagina** (ricarica live standard) se Idiomorph non è presente.

## Abilitazione della ricarica a caldo di FrankenPHP

Per abilitare il ricaricamento a caldo, abilitare Mercure, quindi aggiungere la sottodirettiva `hot_reload` alla direttiva `php_server` nel `Caddyfile`.

> [!WARNING]
>
> Questa funzionalità è destinata esclusivamente agli **ambienti di sviluppo**.
> Non abilitare `hot_reload` in produzione, poiché questa funzionalità non è sicura (espone dettagli interni sensibili) e rallenta l'applicazione.

```caddyfile
localhost

mercure {
    anonymous
}

root public/
php_server {
    hot_reload
}
```

Per impostazione predefinita, FrankenPHP controllerà tutti i file nella cartella di lavoro corrente che corrispondono a questo modello glob: `./**/*.{css,env,gif,htm,html,jpg,jpeg,js,mjs,php,png,svg,twig,webp,xml,yaml,yml}`

È possibile impostare i file da monitorare utilizzando esplicitamente la sintassi glob:

```caddyfile
localhost

mercure {
    anonymous
}

root public/
php_server {
    hot_reload src/**/*{.php,.js} config/**/*.yaml
}
```

Utilizza la forma lunga `hot_reload` per specificare l'argomento Mercure da utilizzare, nonché quali cartelle o file controllare:

```caddyfile
localhost

mercure {
    anonymous
}

root public/
php_server {
    hot_reload {
        topic hot-reload-topic
        watch src/**/*.php
        watch assets/**/*.{ts,json}
        watch templates/
        watch public/css/
    }
}
```

## Integrazione lato client per la ricarica a caldo di FrankenPHP

Mentre il server rileva le modifiche, il browser deve iscriversi a questi eventi per aggiornare la pagina.
FrankenPHP espone l'URL Mercure Hub da utilizzare per sottoscrivere le modifiche ai file tramite la variabile di ambiente `$_SERVER['FRANKENPHP_HOT_RELOAD']`.

È inoltre disponibile una comoda libreria JavaScript, [frankenphp-hot-reload](https://www.npmjs.com/package/frankenphp-hot-reload), per gestire la logica lato client.
Per usarlo, aggiungere quanto segue al layout principale:

```php
<!DOCTYPE html>
<title>FrankenPHP Hot Reload</title>
<?php if (isset($_SERVER['FRANKENPHP_HOT_RELOAD'])): ?>
<meta name="frankenphp-hot-reload:url" content="<?=$_SERVER['FRANKENPHP_HOT_RELOAD']?>">
<script src="https://cdn.jsdelivr.net/npm/idiomorph"></script>
<script src="https://cdn.jsdelivr.net/npm/frankenphp-hot-reload/+esm" type="module"></script>
<?php endif ?>
```

La libreria si iscriverà automaticamente all'hub Mercure, recupererà l'URL corrente in background quando viene rilevata una modifica al file e trasformerà il DOM.
È disponibile come pacchetto [npm](https://www.npmjs.com/package/frankenphp-hot-reload) e su [GitHub](https://github.com/dunglas/frankenphp-hot-reload).

In alternativa, si può implementare una logica lato client iscrivendosi direttamente all'hub Mercure, usando la classe JavaScript nativa `EventSource`.

### Preservazione dei nodi DOM esistenti

In rari casi, come quando si utilizzano strumenti di sviluppo [come la barra degli strumenti di debug web di Symfony](https://github.com/symfony/symfony/pull/62970),
potresti voler preservare specifici nodi DOM.
Per farlo, aggiungere l'attributo `data-frankenphp-hot-reload-preserve` all'elemento HTML pertinente:

```html
<div data-frankenphp-hot-reload-preserve><!-- My debug bar --></div>
```

## Ricarica a caldo con la modalità worker FrankenPHP

Se si esegue l'applicazione in [modalità worker](https://frankenphp.dev/docs/worker/), lo script dell'applicazione rimane in memoria.
Ciò significa che le modifiche al codice PHP non verranno applicate immediatamente, anche se il browser si ricarica.

Per offrire la migliore esperienza agli sviluppatori, si consiglia di combinare `hot_reload` con [la sottodirettiva `watch` nella direttiva `worker`](config.md#controllo-delle-modifiche-ai-file).

- `hot_reload`: aggiorna il **browser** quando i file cambiano
- `worker.watch`: riavvia il worker quando i file cambiano

```caddyfile
# Caddyfile: combina hot reload di FrankenPHP con watch del worker per un flusso di sviluppo completo
localhost

mercure {
    anonymous
}

root public/
php_server {
    hot_reload
    worker {
        file /path/to/my_worker.php
        watch
    }
}
```

## Come funziona la ricarica a caldo di FrankenPHP

1. **Osserva**: FrankenPHP monitora il filesystem per eventuali modifiche utilizzando [la libreria `e-dant/watcher`](https://github.com/e-dant/watcher) dietro le quinte (abbiamo contribuito al binding Go).
2. **Riavvia (modalità worker)**: se `watch` è abilitato nella configurazione del worker, il worker PHP viene riavviato per caricare il nuovo codice.
3. **Push**: un payload JSON contenente l'elenco dei file modificati viene inviato al [Mercure hub](https://mercure.rocks) integrato.
4. **Ricevi**: Il browser, in ascolto tramite la libreria JavaScript, riceve l'evento Mercure.
5. **Aggiorna**:

- Se viene rilevato **Idiomorph**, recupera il contenuto aggiornato e trasforma l'HTML corrente in modo che corrisponda al nuovo stato, applicando le modifiche istantaneamente senza perdere lo stato.
- Altrimenti, viene chiamato `window.location.reload()` per aggiornare la pagina.
