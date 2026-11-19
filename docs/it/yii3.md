---
title: Eseguire Yii3 con FrankenPHP (Docker, modalità worker)
description: Come eseguire un'applicazione Yii3 con FrankenPHP usando l'immagine Docker, un'installazione locale o la modalità worker con il pacchetto yii-runner-frankenphp.
---

# Yii3

## Eseguire Yii3 con l'immagine Docker di FrankenPHP

Servire un'applicazione web [Yii](https://www.yiiframework.com/) con FrankenPHP è facile come montare il progetto nella directory `/app` dell'immagine Docker ufficiale.

Esegui questo comando dalla directory principale della tua app Yii:

```console
docker run -p 80:80 -p 443:443 -p 443:443/udp -v $PWD:/app dunglas/frankenphp
```

E buon divertimento!

## Installare Yii3 con FrankenPHP localmente

In alternativa, puoi eseguire i tuoi progetti Yii con FrankenPHP dalla tua macchina locale:

1. [Scarica il binario corrispondente al tuo sistema](../#binario-standalone)
2. Aggiungi la seguente configurazione a un file chiamato `Caddyfile` nella directory principale del tuo progetto Yii:

   ```caddyfile
   {
   	frankenphp
   }

   # Il nome di dominio del tuo server
   localhost {
   	# Abilita la compressione (opzionale)
   	encode zstd br gzip
   	# Esegui i file PHP dalla directory public/ e servi gli asset
   	php_server {
   		root public/
   		try_files {path} index.php
   	}
   }
   ```

3. Avvia FrankenPHP dalla directory principale del tuo progetto Yii: `frankenphp run`

## Modalità worker di Yii3

Per eseguire la tua applicazione Yii in [modalità worker](worker.md), installa il [runner Yii FrankenPHP](https://github.com/yiisoft/yii-runner-frankenphp):

```console
composer require yiisoft/yii-runner-frankenphp
```

Quindi crea un file chiamato `worker.php` nella directory principale della tua applicazione:

```php
<?php

declare(strict_types=1);

use Yiisoft\Yii\Runner\FrankenPHP\FrankenPHPApplicationRunner;

require_once __DIR__ . '/vendor/autoload.php';

(new FrankenPHPApplicationRunner(rootPath: __DIR__))->run();
```

Aggiorna il tuo `Caddyfile` per avviare l'applicazione in modalità worker:

```caddyfile
{
	frankenphp
}

localhost {
	encode zstd br gzip
	php_server {
		root public/
		worker ./worker.php {
			# Invia tutte le richieste al worker
			match *
			# Ricarica i worker quando i file PHP cambiano (solo in sviluppo)
			watch ./**/*.php
		}
	}
}
```

Se la tua applicazione è basata sul [template ufficiale dell'app Yii](https://github.com/yiisoft/app), consulta il [readme del pacchetto](https://github.com/yiisoft/yii-runner-frankenphp) per un esempio completo di `worker.php` con configurazione di debug, ambiente e gestore errori.

Per limitare il numero di richieste che un worker gestisce prima di essere riavviato (utile per mitigare le perdite di memoria), imposta la variabile d'ambiente `MAX_REQUESTS`. Per impostazione predefinita, i worker gestiscono le richieste indefinitamente.

Quando usi la modalità worker, assicurati che i servizi stateful siano ripristinati dopo ogni richiesta. Consulta la [documentazione sulla modalità worker](worker.md) e la [documentazione di Yii DI `StateResetter`](https://github.com/yiisoft/di#resetting-services-state) per i dettagli.
