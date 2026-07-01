# FrankenPHP: App Server moderno per PHP

<h1 align="center"><a href="https://frankenphp.dev"><img src="../../frankenphp.png" alt="FrankenPHP" width="600"></a></h1>

FrankenPHP è un server di applicazioni per PHP costruito sul server web [Caddy](https://caddyserver.com/).

FrankenPHP dà superpoteri alle app PHP grazie alle sue straordinarie funzionalità: [_Early Hints_](https://frankenphp.dev/docs/early-hints/), [modalità worker](https://frankenphp.dev/docs/worker/), [funzionalità in tempo reale](https://frankenphp.dev/docs/mercure/), [ricaricamento a caldo](https://frankenphp.dev/docs/hot-reload/), supporto automatico HTTPS, HTTP/2 e HTTP/3...

FrankenPHP funziona con qualsiasi app PHP e rende i progetti Laravel e Symfony più veloci che mai grazie alle loro integrazioni ufficiali con la modalità worker.

FrankenPHP può anche essere utilizzato come libreria Go autonoma per incorporare PHP in qualsiasi app utilizzando `net/http`.

[**Ulteriori informazioni** su _frankenphp.dev_](https://frankenphp.dev) e in questa presentazione:

<a href="https://dunglas.dev/2022/10/frankenphp-the-modern-php-app-server-written-in-go/"><img src="https://dunglas.dev/wp-content/uploads/2022/10/frankenphp.png" alt="Slides" width="600"></a>

## Per iniziare

### Script di installazione

Su Linux e macOS, copiare questa riga nel terminale per installare automaticamente
una versione adatta alla piattaforma usata:

```console
curl https://frankenphp.dev/install.sh | sh
```

Su Windows, eseguire in PowerShell:

```powershell
irm https://frankenphp.dev/install.ps1 | iex
```

### Binario autonomo

Forniamo i binari FrankenPHP per Linux, macOS e Windows
con [PHP 8.5](https://www.php.net/releases/8.5/).

I binari Linux sono collegati staticamente, quindi possono essere utilizzati su qualsiasi distribuzione Linux senza installare alcuna dipendenza. Anche i file binari di macOS sono autonomi.
Contengono le estensioni PHP più popolari.
Gli archivi di Windows contengono il binario PHP ufficiale per Windows.

[Scaricare FrankenPHP](https://github.com/php/frankenphp/releases)

### Pacchetti rpm

I nostri manutentori offrono pacchetti rpm per tutti i sistemi che utilizzano `dnf`. Per installare, eseguire:

```console
sudo dnf install https://rpm.henderkes.com/static-php-1-0.noarch.rpm
sudo dnf module enable php-zts:static-8.5 # sono disponibili da 8.2 a 8.5
sudo dnf install frankenphp
```

**Installazione delle estensioni:** `sudo dnf install php-zts-<extension>`

Per le estensioni non disponibili per impostazione predefinita, utilizzare [PIE](https://github.com/php/pie):

```console
sudo dnf install pie-zts
sudo pie-zts install asgrim/example-pie-extension
```

### Pacchetti deb

I nostri manutentori offrono pacchetti deb per tutti i sistemi che utilizzano `apt`. Per installare, eseguire:

```console
VERSION=85 # sono disponibili da 82 a 85
sudo curl https://pkg.henderkes.com/api/packages/${VERSION}/debian/repository.key -o /etc/apt/keyrings/static-php${VERSION}.asc
echo "deb [signed-by=/etc/apt/keyrings/static-php${VERSION}.asc] https://pkg.henderkes.com/api/packages/${VERSION}/debian php-zts main" | sudo tee -a /etc/apt/sources.list.d/static-php${VERSION}.list
sudo apt update
sudo apt install frankenphp
```

**Installazione delle estensioni:** `sudo apt install php-zts-<extension>`

Per le estensioni non disponibili per impostazione predefinita, utilizzare [PIE](https://github.com/php/pie):

```console
sudo apt install pie-zts
sudo pie-zts install asgrim/example-pie-extension
```

### Pacchetti apk

I nostri manutentori offrono pacchetti apk per tutti i sistemi che utilizzano `apk`. Per installare, eseguire:

```console
VERSION=85 # sono disponibili da 82 a 85
echo "https://pkg.henderkes.com/api/packages/${VERSION}/alpine/main/php-zts" | sudo tee -a /etc/apk/repositories
KEYFILE=$(curl -sJOw '%{filename_effective}' https://pkg.henderkes.com/api/packages/${VERSION}/alpine/key)
sudo mv ${KEYFILE} /etc/apk/keys/ &&
sudo apk update &&
sudo apk add frankenphp
```

**Installazione delle estensioni:** `sudo apk add php-zts-<extension>`

Per le estensioni non disponibili per impostazione predefinita, utilizzare [PIE](https://github.com/php/pie):

```console
sudo apk add pie-zts
sudo pie-zts install asgrim/example-pie-extension
```

### Homebrew

FrankenPHP è disponibile anche come pacchetto [Homebrew](https://brew.sh) per macOS e Linux.

```console
brew install dunglas/frankenphp/frankenphp
```

**Installazione delle estensioni:** utilizzare [PIE](https://github.com/php/pie).

### Utilizzo

Per servire il contenuto della cartella corrente, eseguire:

```console
frankenphp php-server
```

Si possono anche eseguire script da riga di comando con:

```console
frankenphp php-cli /path/to/your/script.php
```

Per i pacchetti deb e rpm, si può anche avviare il servizio systemd:

```console
sudo systemctl start frankenphp
```

### Docker

In alternativa, sono disponibili [Immagini Docker](https://frankenphp.dev/docs/docker/):

```console
docker run -v .:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

Aprire `https://localhost` e buon divertimento!

> [!TIP]
>
> Non tentare di utilizzare `https://127.0.0.1`. Usare `https://localhost` e accettare il certificato autofirmato.
> Utilizzare la [variabile di ambiente `SERVER_NAME`](/docs/it/config.md#variabili-dambiente) per modificare il dominio da utilizzare.

## Documentazione

- [Modalità classica](https://frankenphp.dev/docs/classic/)
- [Modalità worker](https://frankenphp.dev/docs/worker/)
- [Migrazione da Nginx/PHP-FPM](https://frankenphp.dev/docs/migrate/)
- [Supporto per Early Hints (codice di stato HTTP 103)](https://frankenphp.dev/docs/early-hints/)
- [Real time](https://frankenphp.dev/docs/mercure/)
- [Log](https://frankenphp.dev/docs/logging/)
- [Ricarica a caldo](https://frankenphp.dev/docs/hot-reload/)
- [Gestione efficiente di grossi file statici](https://frankenphp.dev/docs/x-sendfile/)
- [Configurazione](https://frankenphp.dev/docs/config/)
- [Scrittura delle estensioni PHP in Go](https://frankenphp.dev/docs/extensions/)
- [Immagini Docker](https://frankenphp.dev/docs/docker/)
- [Deploy in produzione](https://frankenphp.dev/docs/production/)
- [Ottimizzazione delle prestazioni](https://frankenphp.dev/docs/performance/)
- [Creare app PHP **autonome** e autoeseguibili](https://frankenphp.dev/docs/embed/)
- [Creare file binari statici](https://frankenphp.dev/docs/static/)
- [Compilare da sorgente](https://frankenphp.dev/docs/compile/)
- [Osservabilità](https://frankenphp.dev/docs/observability/)
- [Integrazione WordPress](https://frankenphp.dev/docs/wordpress/)
- [Integrazione Symfony](https://frankenphp.dev/docs/symfony/)
- [Integrazione Laravel](https://frankenphp.dev/docs/laravel/)
- [Problemi noti](https://frankenphp.dev/docs/known-issues/)
- [App demo (Symfony) e benchmark](https://github.com/dunglas/frankenphp-demo)
- [Documentazione della libreria Go](https://pkg.go.dev/github.com/dunglas/frankenphp)
- [Contributi e debug](https://frankenphp.dev/docs/contributing/)
- [Interni (panoramica dell'architettura)](/docs/internals.md)

## Esempi e scheletri

- [Symfony](https://frankenphp.dev/docs/symfony/)
- [API Platform](https://api-platform.com/docs/symfony)
- [Laravel](https://frankenphp.dev/docs/laravel/)
- [Sulu](https://sulu.io/blog/running-sulu-with-frankenphp)
- [WordPress](https://github.com/StephenMiracle/frankenwp)
- [Drupal](https://github.com/dunglas/frankenphp-drupal)
- [Joomla](https://github.com/alexandreelise/frankenphp-joomla)
- [TYPO3](https://github.com/ochorocho/franken-typo3)
- [Magento2](https://github.com/ekino/frankenphp-magento2)
