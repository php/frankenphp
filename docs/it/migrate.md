---
title: Migrazione da Nginx e PHP-FPM a FrankenPHP
description: Guida passo-passo per migrare un'applicazione PHP da uno stack Nginx più PHP-FPM a FrankenPHP, coprendo Caddyfile, php.ini, thread e Docker.
---

# Migrazione da Nginx/PHP-FPM

FrankenPHP sostituisce sia il tuo server web (Nginx, Apache) che PHP-FPM con un singolo binario.
Questa guida copre una migrazione di base per una tipica applicazione PHP.

## Differenze chiave

| Configurazione PHP-FPM            | Equivalente FrankenPHP                                   |
| --------------------------------- | -------------------------------------------------------- |
| Nginx/Apache + PHP-FPM            | Singolo binario `frankenphp`                             |
| Impostazioni pool `php-fpm.conf`  | [`frankenphp` opzione globale](config.md#caddyfile-config) |
| Blocco `server {}` di Nginx       | Blocco sito `Caddyfile`                                  |
| `php_value` / `php_admin_value`   | [`php_ini` direttiva Caddyfile](config.md#php-config)    |
| `pm = static` / `pm.max_children` | `num_threads`                                            |
| `pm = dynamic`                    | [`max_threads auto`](performance.md#max_threads)         |
| Filtraggio richieste server web   | Route e matcher di Caddy                                 |

## Filtraggio delle richieste HTTP

Quando si migra da un pacchetto Nginx o Apache standard, controllare qualsiasi filtraggio delle richieste che è stato eseguito dal server web prima che PHP-FPM ricevesse la richiesta.
FrankenPHP è basato su Caddy, quindi i metodi e le intestazioni HTTP validi vengono passati attraverso il normale routing di Caddy a meno che il tuo `Caddyfile` o un altro proxy non li rifiuti prima.

Ad esempio, alcuni server web rifiutano le richieste `TRACE` per impostazione predefinita.
Se la tua applicazione deve mantenere tale comportamento, aggiungi un matcher esplicito prima di `php_server`:

```caddyfile
example.com {
    @trace method TRACE
    respond @trace 405

    root /var/www/app/public
    php_server
}
```

Applica lo stesso approccio a qualsiasi politica più rigorosa su metodi o intestazioni che il tuo precedente server web imponeva.
Lo stesso vale per i nomi delle intestazioni delle richieste: alcuni server web o proxy rifiutano o eliminano nomi di campi HTTP validi ma non comuni prima che PHP-FPM li veda, mentre FrankenPHP potrebbe passarli all'applicazione PHP.

## Passo 1: sostituire la configurazione del server web

Una tipica configurazione Nginx + PHP-FPM:

```nginx
# /etc/nginx/sites-available/example.com
server {
    listen 80;
    server_name example.com;
    root /var/www/app/public;
    index index.php;

    location / {
        try_files $uri $uri/ /index.php$is_args$args;
    }

    location ~ \.php$ {
        fastcgi_pass unix:/run/php/php-fpm.sock;
        fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
        include fastcgi_params;
    }
}
```

Diventa un singolo `Caddyfile`:

```caddyfile
example.com {
    root /var/www/app/public
    php_server
}
```

Questo è tutto. La direttiva `php_server` gestisce il routing PHP, il comportamento simile a `try_files` e la gestione dei file statici.

## Passo 2: migrare la configurazione PHP

Il tuo `php.ini` esistente funziona così com'è. Vedi [Configurazione](config.md) per sapere dove posizionarlo a seconda del tuo metodo di installazione.

Un'impostazione cambia significato: `max_execution_time` conta il tempo reale ("wall time") in FrankenPHP, mentre le build non-ZTS di PHP-FPM su Linux contano il tempo CPU.
L'attesa di una query di database, una chiamata HTTP, `sleep()` o il blocco di un file di sessione ora contano ai fini del limite, quindi un valore ottimizzato per PHP-FPM potrebbe essere troppo basso.

Puoi anche impostare le direttive direttamente nel `Caddyfile`:

```caddyfile
{
    frankenphp {
        php_ini memory_limit 256M
        php_ini max_execution_time 30
    }
}

example.com {
    root /var/www/app/public
    php_server
}
```

## Passo 3: regolare la dimensione del pool

In PHP-FPM, si regola `pm.max_children` per controllare il numero di processi worker.
In FrankenPHP, l'equivalente è `num_threads`:

```caddyfile
{
    frankenphp {
        num_threads 16
    }
}
```

Per impostazione predefinita, FrankenPHP avvia 2 thread per CPU. Per uno scaling dinamico simile a `pm = dynamic` di PHP-FPM:

```caddyfile
{
    frankenphp {
        num_threads 4
        max_threads auto
    }
}
```

## Passo 4: migrazione Docker

Una tipica configurazione Docker con PHP-FPM che utilizza due container (Nginx + PHP-FPM) può essere sostituita da un singolo container:

**Prima:**

```yaml
# compose.yaml
services:
  nginx:
    image: nginx:1
    volumes:
      # monta la configurazione Nginx nel container
      - ./config:/etc/nginx/conf.d
      - .:/var/www/app
    ports:
      - "80:80"
      - "443:443"

  php:
    image: php:8.5-fpm
    volumes:
      - .:/var/www/app
```

**Dopo:**

```yaml
# compose.yaml
services:
  php:
    image: dunglas/frankenphp:1-php8.5
    volumes:
      - .:/var/www/app
      # monta il Caddyfile nel container
      - ./config:/etc/frankenphp
      - caddy_data:/data
      - caddy_config:/config
    ports:
      - "80:80"
      - "443:443"
      - "443:443/udp"

volumes:
  caddy_data:
  caddy_config:
```

Se hai bisogno di estensioni PHP aggiuntive, consulta [Creare un'immagine Docker personalizzata](docker.md#how-to-install-more-php-extensions).

Per configurazioni Docker specifiche per framework, consulta [Symfony Docker](https://github.com/dunglas/symfony-docker) e [eseguire Laravel con l'immagine Docker di FrankenPHP](laravel.md#running-laravel-with-the-frankenphp-docker-image).

## Passo 5: considerare la modalità worker (opzionale)

In [modalità classica](classic.md), FrankenPHP funziona come PHP-FPM: ogni richiesta avvia l'applicazione da zero. Questo è un punto di partenza sicuro per la migrazione.

Per prestazioni migliori, puoi passare alla [modalità worker](worker.md), che avvia l'applicazione una volta e la mantiene in memoria:

```caddyfile
example.com {
    root /var/www/app/public
    php_server {
        root /var/www/app/public
        worker index.php 4
    }
}
```

> [!CAUTION]
>
> La modalità worker mantiene la tua applicazione in memoria tra le richieste. Assicurati che il tuo codice non si basi sullo stato globale che viene resettato tra le richieste. Framework come [Symfony](worker.md#worker-mode-for-symfony), [Laravel](laravel.md#laravel-octane) e [API Platform](https://api-platform.com) hanno un supporto nativo per questa modalità.

## Cosa puoi rimuovere

Dopo la migrazione, non avrai più bisogno di:

- Nginx o Apache
- PHP-FPM (servizio/processo `php-fpm`)
- Configurazione FastCGI
- Certificati TLS gestiti autonomamente (Caddy li gestisce automaticamente)
