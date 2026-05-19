# Compilazione dai sorgenti

Questo documento spiega come creare un binario FrankenPHP che caricherà PHP come libreria condivisa.
Questo è il metodo consigliato.

In alternativa, è possibile creare anche [build completamente e prevalentemente statiche](static.md).

## Installare PHP

FrankenPHP è compatibile con PHP 8.2 e versioni successive.

### Con Homebrew (Linux e Mac)

Il modo più semplice per installare una versione di libphp compatibile con FrankenPHP è utilizzare i pacchetti ZTS forniti da [Homebrew PHP](https://github.com/shivammathur/homebrew-php).

Innanzitutto, se non è già stato fatto, installare [Homebrew](https://brew.sh).

Quindi, installa la variante ZTS di PHP, Brotli (opzionale, per il supporto della compressione) e watcher (opzionale, per il rilevamento delle modifiche ai file):

```console
brew install shivammathur/php/php-zts brotli watcher
brew link --overwrite --force shivammathur/php/php-zts
```

### Compilare PHP

In alternativa, si può compilare PHP dai sorgenti con le opzioni necessarie a FrankenPHP, seguendo questi passaggi.

Per prima cosa, [recuperare i sorgenti PHP](https://www.php.net/downloads.php) ed estrarli:

```console
tar xf php-*
cd php-*/
```

Quindi, eseguire lo script `configure` con le opzioni necessarie per la propria piattaforma.
I seguenti flag `./configure` sono obbligatori, ma se ne possono aggiungerne altri, ad esempio, per compilare estensioni o funzionalità aggiuntive.

#### Linux e FreeBSD

```console
./configure \
    --enable-embed \
    --enable-zts \
    --disable-zend-signals \
    --enable-zend-max-execution-timers
```

#### Mac

Utilizzare il gestore pacchetti [Homebrew](https://brew.sh/) per installare le dipendenze obbligatorie e facoltative:

```console
brew install libiconv bison brotli re2c pkg-config watcher
echo 'export PATH="/opt/homebrew/opt/bison/bin:$PATH"' >> ~/.zshrc
```

Quindi eseguire lo script di configurazione:

```console
./configure \
    --enable-embed \
    --enable-zts \
    --disable-zend-signals \
    --with-iconv=/opt/homebrew/opt/libiconv/
```

#### Compilare PHP

Infine, compilare e installare PHP:

```console
make -j"$(getconf _NPROCESSORS_ONLN)"
sudo make install
```

## Installare dipendenze opzionali

Alcune funzionalità di FrankenPHP dipendono da dipendenze di sistema opzionali che devono essere installate.
In alternativa, queste funzionalità possono essere disabilitate passando i tag di build al compilatore Go.

| Caratteristica | Dipendenza | Crea tag per disabilitarlo |
| ------------------------------- | ------------------------------------------------------------------------------------------------------------ | ----------------------- |
| Compressione Brotli | [Brotli](https://github.com/google/brotli) | nobrotli |
| Riavvia i worker alla modifica del file | [Osservatore C](https://github.com/e-dant/watcher/tree/release/watcher-c) | nowatcher |
| [Mercure](mercure.md) | [Libreria Mercure Go](https://pkg.go.dev/github.com/dunglas/mercure) (installata automaticamente, licenza AGPL) | nomercure |

## Compilare l'app Go

Ora si può creare il file binario finale.

### Uso di xcaddy

Il modo consigliato è utilizzare [xcaddy](https://github.com/caddyserver/xcaddy) per compilare FrankenPHP.
`xcaddy` consente inoltre di aggiungere facilmente [moduli Caddy personalizzati](https://caddyserver.com/docs/modules/) ed estensioni FrankenPHP:

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/dunglas/frankenphp/caddy \
    --with github.com/dunglas/mercure/caddy \
    --with github.com/dunglas/vulcain/caddy \
    --with github.com/dunglas/caddy-cbrotli
    # Aggiungere qui moduli Caddy ed estensioni FrankenPHP aggiuntivi
    # facoltativo: se si vuole compilare dai propri sorgenti frankenphp:
    # --with github.com/dunglas/frankenphp=$(pwd) \
    # --with github.com/dunglas/frankenphp/caddy=$(pwd)/caddy

```

> [!TIP]
>
> Se stai usando musl libc (il valore predefinito su Alpine Linux) e Symfony,
> potrebbe essere necessario aumentare la dimensione dello stack predefinita.
> Altrimenti potresti ricevere errori come `PHP Fatal error: Maximum call stack size of 83360 bytes reached during compilation. Try splitting expression`
>
> Per fare ciò, cambiare la variabile d'ambiente `XCADDY_GO_BUILD_FLAGS` in qualcosa di simile
> `XCADDY_GO_BUILD_FLAGS=$'-ldflags "-w -s -extldflags \'-Wl,-z,stack-size=0x80000\'"'`
> (modificare il valore della dimensione dello stack in base alle esigenze dell'app).

### Senza xcaddy

In alternativa, è possibile compilare FrankenPHP senza `xcaddy` utilizzando direttamente il comando `go`:

```console
curl -L https://github.com/php/frankenphp/archive/refs/heads/main.tar.gz | tar xz
cd frankenphp-main/caddy/frankenphp
CGO_CFLAGS=$(php-config --includes) CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" go build -tags=nobadger,nomysql,nopgx
```
