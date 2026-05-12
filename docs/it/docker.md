# Creazione di un'immagine Docker personalizzata

Le [immagini Docker](https://hub.docker.com/r/dunglas/frankenphp) sono basate su [immagini PHP ufficiali](https://hub.docker.com/_/php/).
Per le architetture più diffuse vengono fornite varianti Debian e Alpine Linux.
Si consigliano le varianti Debian.

Sono fornite varianti per PHP 8.2, 8.3, 8.4 e 8.5.

I tag seguono questo schema: `dunglas/frankenphp:<frankenphp-version>-php<php-version>-<os>`

- `<frankenphp-version>` e `<php-version>` sono rispettivamente i numeri di versione di FrankenPHP e PHP, che vanno dalla versione maggiore (ad esempio `1`), minore (ad esempio `1.2`) alle versioni patch (ad esempio `1.2.3`).
- `<os>` è `trixie` (per Debian Trixie), `bookworm` (per Debian Bookworm) o `alpine` (per l'ultima versione stabile di Alpine).

[Sfogliare i tag](https://hub.docker.com/r/dunglas/frankenphp/tags).

## Come utilizzare le immagini Docker di FrankenPHP

Creare un `Dockerfile` nel proprio progetto:

```dockerfile
FROM dunglas/frankenphp

COPY . /app/public
```

Quindi, eseguire questi comandi per creare ed eseguire l'immagine Docker:

```console
docker build -t my-php-app .
docker run -it --rm --name my-running-app my-php-app
```

## Come modificare la configurazione di FrankenPHP Docker

Per comodità, [un `Caddyfile` predefinito](https://github.com/php/frankenphp/blob/main/caddy/frankenphp/Caddyfile) contenente
le variabili di ambiente utili viene fornito nell'immagine.

## Come installare altre estensioni PHP

Lo script [`docker-php-extension-installer`](https://github.com/mlocati/docker-php-extension-installer) è fornito nell'immagine di base.
Aggiungere ulteriori estensioni PHP è semplice:

```dockerfile
FROM dunglas/frankenphp

# aggiungere eventuali altre estensioni qui:
RUN install-php-extensions \
	pdo_mysql \
	gd \
	intl \
	zip \
	opcache
```

## Come installare altri moduli Caddy

FrankenPHP è basato su Caddy e tutti i [moduli Caddy](https://caddyserver.com/docs/modules/) possono essere utilizzati con FrankenPHP.

Il modo più semplice per installare moduli Caddy personalizzati è utilizzare [xcaddy](https://github.com/caddyserver/xcaddy):

```dockerfile
FROM dunglas/frankenphp:builder AS builder

# Copiare xcaddy nell'immagine
COPY --from=caddy:builder /usr/bin/xcaddy /usr/bin/xcaddy

# Occorre abilitare CGO per la build di FrankenPHP
RUN CGO_ENABLED=1 \
    XCADDY_SETCAP=1 \
    XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
    CGO_CFLAGS=$(php-config --includes) \
    CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
    xcaddy build \
        --output /usr/local/bin/frankenphp \
        --with github.com/dunglas/frankenphp=./ \
        --with github.com/dunglas/frankenphp/caddy=./caddy/ \
        --with github.com/dunglas/caddy-cbrotli \
        # Mercure e Vulcain sono inclusi nella build ufficiale, ma possono essere rimossi a piacimento
        --with github.com/dunglas/mercure/caddy \
        --with github.com/dunglas/vulcain/caddy
        # Aggiungere eventuali altri moduli Caddy qui

FROM dunglas/frankenphp AS runner

# Sostituisce il binario ufficiale con quello contenente i moduli personalizzati
COPY --from=builder /usr/local/bin/frankenphp /usr/local/bin/frankenphp
```

L'immagine `builder` fornita da FrankenPHP contiene una versione compilata di `libphp`.
[Immagini builder](https://hub.docker.com/r/dunglas/frankenphp/tags?name=builder) sono fornite per tutte le versioni di FrankenPHP e PHP, sia per Debian sia per Alpine.

> [!TIP]
>
> Se si usano Alpine Linux e Symfony,
> potrebbe essere necessario [aumentare la dimensione dello stack predefinita](compile.md#uso-di-xcaddy).

## Abilitazione della modalità worker per impostazione predefinita

Impostare la variabile d'ambiente `FRANKENPHP_CONFIG` per avviare FrankenPHP con uno script worker:

```dockerfile
FROM dunglas/frankenphp

# ...

ENV FRANKENPHP_CONFIG="worker ./public/index.php"
```

## Utilizzo di un volume in fase di sviluppo

Per sviluppare facilmente con FrankenPHP, montare la cartella dell'host contenente il codice dell'applicazione come volume nel contenitore Docker:

```console
docker run -v $PWD:/app/public -p 80:80 -p 443:443 -p 443:443/udp --tty my-php-app
```

> [!TIP]
>
> L'opzione `--tty` consente di avere log leggibili invece dei log JSON.

Con Docker Compose:

```yaml
# compose.yaml

services:
  php:
    image: dunglas/frankenphp
    # scommentare questa riga se si vuole un proprio Dockerfile
    #build: .
    # scommentare questa riga per eseguire in ambiente di produzione
    # restart: always
    ports:
      - "80:80" # HTTP
      - "443:443" # HTTPS
      - "443:443/udp" # HTTP/3
    volumes:
      - ./:/app/public
      - caddy_data:/data
      - caddy_config:/config
    # commentare questa riga in produzione, consente di avere log leggibili in dev
    tty: true

# Volumi necessari per certificati e configurazione di Caddy
volumes:
  caddy_data:
  caddy_config:
```

## Esecuzione come utente non root

FrankenPHP può essere eseguito come utente non root in Docker.

Ecco un esempio di `Dockerfile` che esegue questa operazione:

```dockerfile
FROM dunglas/frankenphp

ARG USER=appuser

RUN <<-EOF
	# Usare "adduser -D ${USER}" in distro basate su Alpine
	useradd ${USER}
	# Capacità aggiuntive di bind su porte 80 e 443
	setcap CAP_NET_BIND_SERVICE=+eip /usr/local/bin/frankenphp
	# Dà accesso in scrittura a /config/caddy e /data/caddy
	chown -R ${USER}:${USER} /config/caddy /data/caddy
EOF

USER ${USER}
```

### Esecuzione senza capacità

Anche quando viene eseguito senza root, FrankenPHP necessita della funzionalità `CAP_NET_BIND_SERVICE` per associare il
server web su porte privilegiate (80 e 443).

Espondendo FrankenPHP su una porta non privilegiata (1024 e successive), è possibile eseguire
il server web come utente non root e senza la necessità di altre funzionalità:

```dockerfile
FROM dunglas/frankenphp

ARG USER=appuser

RUN <<-EOF
	# Usare "adduser -D ${USER}" in distro basate su Alpine
	useradd ${USER}
	# Rimuove capacità predefinite
	setcap -r /usr/local/bin/frankenphp
	# Dà accesso in scrittura a /config/caddy e /data/caddy
	chown -R ${USER}:${USER} /config/caddy /data/caddy
EOF

USER ${USER}
```

Successivamente, imposta la variabile di ambiente `SERVER_NAME` per utilizzare una porta non privilegiata.
Esempio: `:8000`

## Aggiornamenti dell'immagine Docker di FrankenPHP

Le immagini Docker vengono create:

- quando viene rilasciata una nuova versione
- tutti i giorni alle 4:00 UTC, se sono disponibili nuove versioni delle immagini ufficiali di PHP 

## Rafforzamento delle immagini

Per ridurre ulteriormente la superficie di attacco e le dimensioni delle immagini Docker di FrankenPHP, è anche possibile costruirle su
[Google distroless](https://github.com/GoogleContainerTools/distroless) o su un'immagine
[Docker rafforzata](https://www.docker.com/products/hardened-images).

> [!WARNING]
> Queste immagini di base minime non includono una shell o un gestore di pacchetti, il che rende il debug più difficile.
> Sono quindi consigliate solo per la produzione, se la sicurezza è una priorità elevata.

Quando si aggiungono estensioni PHP, sarà necessaria una fase di compilazione intermedia:

```dockerfile
FROM dunglas/frankenphp AS builder

# Estensioni PHP aggiuntive
RUN install-php-extensions pdo_mysql pdo_pgsql #...

# Copia le librerie condivise di frankenphp e tutte le estensioni installate in una posizione temporanea
# You can also do this step manually by analyzing ldd output of frankenphp binary and each extension .so file
RUN <<-EOF
	apt-get update
	apt-get install -y --no-install-recommends libtree
	mkdir -p /tmp/libs
	for target in $(which frankenphp) \
		$(find "$(php -r 'echo ini_get("extension_dir");')" -maxdepth 2 -name "*.so"); do
		libtree -pv "$target" 2>/dev/null | grep -oP '(?:── )\K/\S+(?= \[)' | while IFS= read -r lib; do
			[ -f "$lib" ] && cp -n "$lib" /tmp/libs/
		done
	done
EOF


# Immagine di base Debian Distroless, assicurarsi che coincida con la versione di Debian del builder
FROM gcr.io/distroless/base-debian13
# Immagine Docker rafforzata alternativa
# FROM dhi.io/debian:13

COPY --from=builder /usr/local/bin/frankenphp /usr/local/bin/frankenphp
COPY --from=builder /usr/local/lib/php/extensions /usr/local/lib/php/extensions
COPY --from=builder /tmp/libs /usr/lib

COPY --from=builder /usr/local/etc/php/conf.d /usr/local/etc/php/conf.d
COPY --from=builder /usr/local/etc/php/php.ini-production /usr/local/etc/php/php.ini

# Configurazione e dati devono essere scrivibili da tutti, anche su un filesystem in sola lettura
ENV XDG_CONFIG_HOME=/config XDG_DATA_HOME=/data
COPY --from=builder --chown=nonroot:nonroot /data /data
COPY --from=builder --chown=nonroot:nonroot /config /config

# Copia l'applicazione (mantenuta di proprietà di root) e Caddyfile
COPY . /app
COPY Caddyfile /etc/caddy/Caddyfile

USER nonroot
WORKDIR /app

ENTRYPOINT ["/usr/local/bin/frankenphp", "run", "--config", "/etc/caddy/Caddyfile"]
```

## Versioni di sviluppo

Le versioni di sviluppo sono disponibili su [`dunglas/frankenphp-dev`](https://hub.docker.com/repository/docker/dunglas/frankenphp-dev).
Una nuova build viene attivata ogni volta che un commit viene inviato al ramo main del repository GitHub.

I tag `latest*` puntano all'head del ramo `main`.
Sono disponibili anche i tag nel formato `sha-<git-commit-hash>`.
