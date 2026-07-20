# Deploy in produzione

In questo tutorial impareremo come eseguire il deploy di un'applicazione PHP su un singolo server con Docker Compose.

Se si usa Symfony, meglio far riferimento alla documentazione "[Deploy in produzione](https://github.com/dunglas/symfony-docker/blob/main/docs/production.md)" del progetto Symfony Docker (che utilizza FrankenPHP).

Se si usa API Platform (che utilizza anche FrankenPHP), fare riferimento alla [documentazione del framework](https://api-platform.com/docs/deployment/).

## Preparazione dell'app

Innanzitutto, creare un `Dockerfile` nella cartella principale del progetto:

```dockerfile
FROM dunglas/frankenphp

# Assicurarsi di sostituire "nome-dominio.example.com" con il nome di dominio desiderato
ENV SERVER_NAME=nome-dominio.example.com
# Se si vuole disabilitare HTTPS, usare invece questo valore:
#ENV SERVER_NAME=:80

# Se il progetto non usa "public" come document root, impostare questo valore:
# ENV SERVER_ROOT=web/

# Abilita le impostazioni di produzione di PHP
RUN mv "$PHP_INI_DIR/php.ini-production" "$PHP_INI_DIR/php.ini"

# Copia i file di PHP del progetto nella cartella public
COPY . /app/public
# Se si usa Symfony o Laravel, occorre invece copiare l'intero progetto:
#COPY . /app
```

Fare riferimento a "[Creazione di un'immagine Docker personalizzata](docker.md)" per ulteriori dettagli e opzioni,
e per imparare a personalizzare la configurazione, installare le estensioni PHP e i moduli Caddy.

Se il progetto utilizza Composer,
assicurarsi di includerlo nell'immagine Docker e di installare le dipendenze.

Quindi, aggiungere un file `compose.yaml`:

```yaml
# compose.yaml
services:
  php:
    image: dunglas/frankenphp
    restart: always
    ports:
      - "80:80" # HTTP
      - "443:443" # HTTPS
      - "443:443/udp" # HTTP/3
    volumes:
      - caddy_data:/data
      - caddy_config:/config

# Volumi necessari per certificati e configurazione di Caddy
volumes:
  caddy_data:
  caddy_config:
```

> [!NOTE]
>
> Gli esempi precedenti sono destinati all'utilizzo in produzione.
> In fase di sviluppo, si potrebbe voler utilizzare un volume, una diversa configurazione PHP e un valore diverso per la variabile d'ambiente `SERVER_NAME`.
>
> Dare un'occhiata al progetto [Symfony Docker](https://github.com/dunglas/symfony-docker)
> (che utilizza FrankenPHP) per un esempio più avanzato con immagini multi-stage,
> Composer, estensioni PHP extra, ecc.

Infine, se si usa Git, eseguire il commit e il push di questi file.

## Preparazione del server

Per il deploy dell'applicazione in produzione, è necessario un server.
In questo tutorial utilizzeremo una macchina virtuale fornita da DigitalOcean, ma qualsiasi server Linux può andar bene.
Se si ha già un server Linux con Docker installato, passare direttamente a [Configurazione di un nome di dominio](#configurazione-di-un-nome-di-dominio).

Altrimenti, utilizzare [questo link di affiliazione di DigitalOcean](https://m.do.co/c/5d8aabe3ab80) per ottenere $ 200 di credito gratuito, creare un account, quindi fare clic su "Crea una Droplet".
Quindi, fare clic sulla scheda "Marketplace" nella sezione "Scegli un'immagine" e cercare l'app denominata "Docker".
Ciò fornirà un server Ubuntu con le ultime versioni di Docker e Docker Compose già installate!

A scopo di test, saranno sufficienti i piani più economici.
Per un utilizzo in produzione reale, probabilmente è meglio scegliere un piano nella sezione "general purpose" adatto alle proprie esigenze.

![Deploy di FrankenPHP su DigitalOcean con Docker](../digitalocean-droplet.png)

Si possono mantenere i valori predefiniti per altre impostazioni o modificarli in base alle proprie esigenze.
Non dimenticare di aggiungere la chiave SSH o creare una password, quindi premere il pulsante "Finalize and create".

Quindi, attendere qualche secondo mentre il Droplet esegue il provisioning.
Quando il Droplet è pronto, usare SSH per connettersi:

```console
ssh root@<droplet-ip>
```

## Configurazione di un nome di dominio

Nella maggior parte dei casi, consigliamo di associare un nome di dominio al sito.
Se non si possiede ancora un nome di dominio, sarà necessario acquistarne uno.

Creare quindi un record DNS di tipo `A` per il nome di dominio che punta all'indirizzo IP del server:

```dns
your-domain-name.example.com.  IN  A     207.154.233.113
```

Esempio con il servizio di domini di DigitalOcean ("Networking" > "Domains"):

![Configurazione DNS su DigitalOcean](digitalocean-dns.png)

> [!NOTE]
>
> Let's Encrypt, il servizio utilizzato per impostazione predefinita da FrankenPHP per generare automaticamente un certificato TLS, non supporta l'utilizzo di indirizzi IP puri. L'utilizzo di un nome di dominio è obbligatorio per utilizzare Let's Encrypt.

## Deploy di FrankenPHP con Docker Compose

Copiare il progetto sul server utilizzando `git clone`, `scp` o qualsiasi altro strumento adatto.
Se si usa GitHub, è una buona idea usare [una chiave di deploy](https://docs.github.com/en/free-pro-team@latest/developers/overview/managing-deploy-keys#deploy-keys).
Le chiavi di deploy sono supportate anche da [GitLab](https://docs.gitlab.com/ee/user/project/deploy_keys/).

Esempio con Git:

```console
git clone git@github.com:<username>/<project-name>.git
```

Andare nella cartella contenente il progetto (`<project-name>`) e avviare l'app in modalità produzione:

```console
docker compose up --wait
```

Il server è ora attivo e funzionante e un certificato HTTPS è stato generato automaticamente.
Aprire `https://your-domain-name.example.com` e buon divertimento!

> [!CAUTION]
>
> Docker può avere un livello di cache, assicurarsi di avere la build giusta per ogni deploy o ripetere la build del progetto con l'opzione `--no-cache` per evitare problemi di cache.

## Esecuzione dietro un reverse proxy

Se FrankenPHP è in esecuzione dietro un reverse proxy o un load balancer (ad esempio, Nginx, AWS ELB, Google Cloud LB),
occorre configurare l'[opzione globale `trusted_proxies`](https://caddyserver.com/docs/caddyfile/options#trusted-proxies) nel Caddyfile
in modo che Caddy si fidi degli header `X-Forwarded-*` in entrata:

```caddyfile
{
	servers {
		trusted_proxies static <your-IPs>
	}
}
```

Se necessario, sostituire `<your-IPs>` con gli intervalli IP effettivi del proxy.

Inoltre, anche il framework PHP usato deve essere configurato per fidarsi del proxy.
Ad esempio, impostare la [variabile d'ambiente `TRUSTED_PROXIES`](https://symfony.com/doc/current/deployment/proxies.html) per Symfony,
o il [middleware `trustedproxies`](https://laravel.com/docs/trustedproxy) per Laravel.

Senza entrambe le configurazioni, header come `X-Forwarded-For` e `X-Forwarded-Proto` verranno ignorati,
il che potrebbe causare problemi come rilevamento HTTPS errato o indirizzi IP client errati.

## Deploy su più nodi

In caso di deploy su un cluster di macchine, si può utilizzare [Docker Swarm](https://docs.docker.com/engine/swarm/stack-deploy/),
che è compatibile con i file Compose forniti.
Per il deploy su Kubernetes, dare un'occhiata alla [tabella Helm fornita con API Platform](https://api-platform.com/docs/deployment/kubernetes/), che utilizza FrankenPHP.
