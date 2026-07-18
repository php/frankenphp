# Configurazione

FrankenPHP, Caddy e i moduli [Mercure](mercure.md) e [Vulcain](https://vulcain.rocks) possono essere configurati utilizzando [i formati supportati da Caddy](https://caddyserver.com/docs/getting-started#your-first-config).

Il formato più comune è `Caddyfile`, che è un formato di testo semplice e leggibile.
Per impostazione predefinita, FrankenPHP cercherà un `Caddyfile` nella cartella corrente.
È possibile specificare un percorso personalizzato con l'opzione `-c` o `--config`.

Di seguito è mostrato un `Caddyfile` minimo per servire un'applicazione PHP:

```caddyfile
# Il nome host a cui rispondere
localhost

# Facoltativo, il percorso da cui servire i file, altrimenti sarà usato il percorso attuale
#root public/
php_server
```

Viene fornito un `Caddyfile` più avanzato che abilita più funzionalità e fornisce comode variabili di ambiente [nel repository FrankenPHP](https://github.com/php/frankenphp/blob/main/caddy/frankenphp/Caddyfile),
e con immagini Docker.

PHP stesso può essere configurato [utilizzando un file `php.ini`](https://www.php.net/manual/configuration.file.php).

A seconda del metodo di installazione, FrankenPHP e l'interprete PHP cercheranno i file di configurazione nelle posizioni descritte di seguito.

## Docker

FrankenPHP:

- `/etc/frankenphp/Caddyfile`: il file di configurazione principale
- `/etc/frankenphp/Caddyfile.d/*.caddyfile`: file di configurazione aggiuntivi che vengono caricati automaticamente

PHP:

- `php.ini`: `/usr/local/etc/php/php.ini` (per impostazione predefinita non è fornito `php.ini`)
- file di configurazione aggiuntivi: `/usr/local/etc/php/conf.d/*.ini`
- Estensioni PHP: `/usr/local/lib/php/extensions/no-debug-zts-<YYYYMMDD>/`
- Dovresti copiare un modello ufficiale fornito dal progetto PHP:

```dockerfile
FROM dunglas/frankenphp

# Produzione:
RUN cp $PHP_INI_DIR/php.ini-production $PHP_INI_DIR/php.ini

# O sviluppo:
RUN cp $PHP_INI_DIR/php.ini-development $PHP_INI_DIR/php.ini
```

## Pacchetti RPM e Debian

FrankenPHP:

- `/etc/frankenphp/Caddyfile`: il file di configurazione principale
- `/etc/frankenphp/Caddyfile.d/*.caddyfile`: file di configurazione aggiuntivi che vengono caricati automaticamente

PHP:

- `php.ini`: `/etc/php-zts/php.ini` (per impostazione predefinita viene fornito un file `php.ini` con preimpostazioni di produzione)
- file di configurazione aggiuntivi: `/etc/php-zts/conf.d/*.ini`

## Binario statico

FrankenPHP:

- Nella cartella di lavoro corrente: `Caddyfile`

PHP:

- `php.ini`: la cartella in cui viene eseguito `frankenphp run` o `frankenphp php-server`, quindi `/etc/frankenphp/php.ini`
- file di configurazione aggiuntivi: `/etc/frankenphp/php.d/*.ini`
- Estensioni PHP: non possono essere caricate, raggruppatele nel binario stesso
- copia uno dei `php.ini-production` o `php.ini-development` forniti [nei sorgenti PHP](https://github.com/php/php-src/).

## Configurazione Caddyfile

`php_server` o `php` [direttive HTTP](https://caddyserver.com/docs/caddyfile/concepts#directives) possono essere utilizzate all'interno dei blocchi del sito per servire l'app PHP.

Esempio minimo:

```caddyfile
localhost {
	# Abilita la compressione (opzionale)
	encode zstd br gzip
	# Esegue i file PHP nella directory corrente e serve le risorse statiche
	php_server
}
```

Puoi anche configurare esplicitamente FrankenPHP utilizzando l'[opzione globale](https://caddyserver.com/docs/caddyfile/concepts#global-options) `frankenphp`:

```caddyfile
{
	frankenphp {
		num_threads <num_threads> # Imposta il numero di thread PHP da avviare. Predefinito: 2x il numero di CPU disponibili.
		max_threads <num_threads> # Limita il numero di thread PHP aggiuntivi avviabili a runtime. Predefinito: num_threads. Può essere impostato su 'auto'.
		max_wait_time <duration> # Imposta il tempo massimo di attesa di una richiesta per un thread PHP libero prima del timeout. Predefinito: disabilitato.
		max_idle_time <duration> # Imposta il tempo massimo di inattività di un thread autoscalato prima della disattivazione. Predefinito: 5s.
		max_requests <num> # (sperimentale) Imposta il numero massimo di richieste gestite da un thread PHP prima del riavvio, utile per mitigare memory leak. Si applica ai thread regolari e worker. Predefinito: 0 (illimitato).
		php_ini <key> <value> # Imposta una direttiva php.ini. Può essere usata più volte per impostare più direttive.
		worker {
			file <path> # Imposta il percorso dello script worker.
			num <num> # Imposta il numero di thread PHP da avviare, predefinito: 2x il numero di CPU disponibili.
			env <key> <value> # Imposta una variabile di ambiente aggiuntiva al valore specificato. Può essere indicata più volte.
			watch <path> # Imposta il percorso da osservare per le modifiche ai file. Può essere indicato più volte.
			name <name> # Imposta il nome del worker, usato nei log e nelle metriche. Predefinito: percorso assoluto del file worker.
			max_consecutive_failures <num> # Imposta il numero massimo di errori consecutivi prima che il worker sia considerato non sano; -1 significa che il worker verrà sempre riavviato. Predefinito: 6.
		}
	}
}

# ...
```

In alternativa, è possibile utilizzare la forma breve di una riga dell'opzione `worker`:

```caddyfile
{
	frankenphp {
		worker <file> <num>
	}
}

# ...
```

Puoi anche definire più worker se servi più app sullo stesso server:

```caddyfile
app.example.com {
    root /path/to/app/public
	php_server {
		root /path/to/app/public # consente una cache migliore
		worker index.php <num>
	}
}

other.example.com {
    root /path/to/other/public
	php_server {
		root /path/to/other/public
		worker index.php <num>
	}
}

# ...
```

L'uso della direttiva `php_server` è generalmente ciò che serve,
ma se si ha bisogno del controllo completo, si può utilizzare la direttiva `php` di livello inferiore.
La direttiva `php` passa tutti gli input a PHP, invece di verificare prima se
sia un file PHP o meno. Per ulteriori informazioni, consulta la [pagina sulle prestazioni](performance.md#try_files).

L'utilizzo della direttiva `php_server` equivale a questa configurazione:

```caddyfile
route {
	# Aggiunge lo slash finale alle richieste verso cartelle
	@canonicalPath {
		file {path}/index.php
		not path */
	}
	redir @canonicalPath {path}/ 308
	# Se il file richiesto non esiste, prova i file index
	@indexFiles file {
		try_files {path} {path}/index.php index.php
		split_path .php
	}
	rewrite @indexFiles {http.matchers.file.relative}
	# FrankenPHP!
	@phpFiles path *.php
	php @phpFiles
	file_server
}
```

Le direttive `php_server` e `php` hanno le seguenti opzioni:

```caddyfile
php_server [<matcher>] {
	root <directory> # Imposta la cartella root del sito. Predefinito: direttiva `root`.
	split_path <delim...> # Imposta le sottostringhe per dividere l'URI in due parti. La prima sottostringa corrispondente viene usata per separare la "path info" dal percorso. La prima parte viene suffissata con la sottostringa corrispondente ed è considerata il nome effettivo della risorsa (script CGI). La seconda parte viene impostata in PATH_INFO per lo script. Predefinito: `.php`
	resolve_root_symlink false # Disabilita la risoluzione della directory `root` al suo valore reale tramite link simbolico, se presente (abilitato per impostazione predefinita).
	env <key> <value> # Imposta una variabile di ambiente aggiuntiva al valore specificato. Può essere indicata più volte.
	file_server off # Disabilita la direttiva file_server integrata.
	worker { # Crea un worker specifico per questo server. Può essere indicato più volte per più worker.
		file <path> # Imposta il percorso dello script worker; può essere relativo alla root di php_server.
		num <num> # Imposta il numero di thread PHP da avviare, predefinito: 2x il numero di CPU disponibili.
		name <name> # Imposta il nome del worker, usato nei log e nelle metriche. Predefinito: percorso assoluto del file worker. Quando è definito in un blocco php_server inizia sempre con m#.
		watch <path> # Imposta il percorso da osservare per le modifiche ai file. Può essere indicato più volte.
		env <key> <value> # Imposta una variabile di ambiente aggiuntiva al valore specificato. Può essere indicata più volte. Le variabili per questo worker sono ereditate anche dal parent php_server, ma possono essere sovrascritte qui.
		match <path> # Associa il worker a un pattern di percorso. Sovrascrive try_files e può essere usato solo nella direttiva php_server.
	}
	worker <other_file> <num> # È possibile usare anche la forma breve, come nel blocco globale frankenphp.
}
```

### Controllo delle modifiche ai file

Poiché i worker avviano l'applicazione solo una volta e la mantengono in memoria, eventuali modifiche
ai file PHP non si rifletteranno immediatamente.

I worker possono invece essere riavviati in caso di modifiche ai file tramite la direttiva `watch`.
Ciò è utile per gli ambienti di sviluppo.

```caddyfile
{
	frankenphp {
		worker {
			file  /path/to/app/public/worker.php
			watch
		}
	}
}
```

Questa funzione viene spesso utilizzata in combinazione con [ricaricamento a caldo](hot-reload.md).

Se la cartella `watch` non è specificata, tornerà a `./**/*.{env,php,twig,yaml,yml}`,
che controlla tutti i file `.env`, `.php`, `.twig`, `.yaml` e `.yml` nella cartella e nelle sottocartelle
dove è stato avviato il processo FrankenPHP. Puoi invece anche specificare una o più cartelle tramite a
[modello nome file shell](https://pkg.go.dev/path/filepath#Match):

```caddyfile
{
	frankenphp {
		worker {
			file  /path/to/app/public/worker.php
			watch /path/to/app # osserva tutti i file in tutte le sottocartelle di /path/to/app
			watch /path/to/app/*.php # osserva i file che terminano con .php in /path/to/app
			watch /path/to/app/**/*.php # osserva i file PHP in /path/to/app e nelle sottocartelle
			watch /path/to/app/**/*.{php,twig} # osserva i file PHP e Twig in /path/to/app e nelle sottocartelle
		}
	}
}
```

- Il modello `**` indica la visione ricorsiva
- Le cartelle possono anche essere relative (a dove viene avviato il processo FrankenPHP)
- Se sono definiti più worker, tutti verranno riavviati quando un file cambia
- Fare attenzione a guardare i file creati in fase di esecuzione (come i log) poiché potrebbero causare riavvii indesiderati del worker.

Il file watcher è basato su [e-dant/watcher](https://github.com/e-dant/watcher).

## Abbinamento del worker a un percorso

Nelle applicazioni PHP tradizionali, gli script vengono sempre inseriti nella cartella pubblica.
Questo vale anche per i worker, che vengono trattati come qualsiasi altro script PHP.
Se si desidera invece inserire lo script worker all'esterno della cartella pubblica, lo si può fare tramite la direttiva `match`.

La direttiva `match` è un'alternativa ottimizzata a `try_files` disponibile solo all'interno di `php_server` e `php`.
L'esempio seguente servirà sempre un file nella cartella pubblica, se presente,
altrimenti inoltrerà la richiesta al worker che corrisponde al modello di percorso.

```caddyfile
{
	frankenphp {
		php_server {
			worker {
				file /path/to/worker.php # il file può trovarsi fuori dal percorso pubblico
				match /api/* # ogni richiesta che inizia per /api/ sarà gestita da questo worker
			}
		}
	}
}
```

## Riavvio dei thread dopo una serie di richieste (sperimentale)

FrankenPHP può riavviare automaticamente i thread PHP dopo aver gestito un determinato numero di richieste.
Quando un thread raggiunge il limite, viene riavviato completamente,
ripulire tutta la memoria e lo stato. Altri thread continuano a servire le richieste durante il riavvio.

Se noti che la memoria aumenta nel tempo, la soluzione ideale è segnalare la perdita
all'estensione responsabile o al manutentore della libreria.
Ma quando la soluzione dipende da una terza parte che non controlli,
`max_requests` fornisce una soluzione pragmatica e, si spera, temporanea per la produzione:

```caddyfile
{
	frankenphp {
		max_requests 500
	}
}
```

## Variabili d'ambiente

Le seguenti variabili di ambiente possono essere utilizzate per inserire le direttive Caddy in `Caddyfile` senza modificarlo:

- `SERVER_NAME`: modifica [gli indirizzi su cui ascoltare](https://caddyserver.com/docs/caddyfile/concepts#addresses), i nomi host forniti verranno utilizzati anche per il certificato TLS generato
- `SERVER_ROOT`: cambia la cartella principale del sito, il valore predefinito è `public/`
- `CADDY_GLOBAL_OPTIONS`: inserire [opzioni globali](https://caddyserver.com/docs/caddyfile/options)
- `FRANKENPHP_CONFIG`: inietta la configurazione nella direttiva `frankenphp`

Per quanto riguarda le SAPI FPM e CLI, le variabili di ambiente sono esposte per impostazione predefinita nel superglobale `$_SERVER`.

Il valore `S` della [direttiva PHP `variables_order`](https://www.php.net/manual/ini.core.php#ini.variables-order) è sempre equivalente a `ES` indipendentemente dal posizionamento di `E` altrove in questa direttiva.

## Configurazione PHP

Per caricare [file di configurazione PHP aggiuntivi](https://www.php.net/manual/configuration.file.php#configuration.file.scan),
è possibile utilizzare la variabile di ambiente `PHP_INI_SCAN_DIR`.
Quando impostato, PHP caricherà tutti i file con estensione `.ini` presenti nelle cartelle indicate.

Puoi anche modificare la configurazione PHP utilizzando la direttiva `php_ini` in `Caddyfile`:

```caddyfile
{
    frankenphp {
        php_ini memory_limit 256M

        # or

        php_ini {
            memory_limit 256M
            max_execution_time 15
        }
    }
}
```

### Disabilitare HTTPS

Per impostazione predefinita, FrankenPHP abiliterà automaticamente HTTPS per tutti i nomi host, incluso `localhost`.
Se si desidera disabilitare HTTPS (ad esempio in un ambiente di sviluppo), si può impostare la variabile di ambiente `SERVER_NAME` a `http://` o `:80`:

In alternativa, è possibile utilizzare tutti gli altri metodi descritti nella [documentazione Caddy](https://caddyserver.com/docs/automatic-https#activation).

Se si vuole utilizzare HTTPS con l'indirizzo IP `127.0.0.1` invece del nome host `localhost`, leggere la sezione [problemi noti](known-issues.md#uso-di-https127001-con-docker).

### Duplex completo (HTTP/1)

Quando si utilizza HTTP/1.x, potrebbe essere opportuno abilitare la modalità full-duplex per consentire la scrittura di una risposta prima  che l'intero body
sia stato letto. (ad esempio: [Mercure](mercure.md), WebSocket, eventi inviati dal server, ecc.)

Questa è una configurazione opt-in che deve essere aggiunta alle opzioni globali in `Caddyfile`:

```caddyfile
{
  servers {
    enable_full_duplex
  }
}
```

> [!CAUTION]
>
> L'abilitazione di questa opzione potrebbe causare un deadlock dei vecchi client HTTP/1.x che non supportano il full-duplex.
> Questo può anche essere configurato utilizzando la configurazione dell'ambiente `CADDY_GLOBAL_OPTIONS`:

```sh
CADDY_GLOBAL_OPTIONS="servers {
  enable_full_duplex
}"
```

Puoi trovare ulteriori informazioni su questa impostazione nella [documentazione Caddy](https://caddyserver.com/docs/caddyfile/options#enable-full-duplex).

## Abilita la modalità debug

Quando si utilizza l'immagine Docker, impostare la variabile di ambiente `CADDY_GLOBAL_OPTIONS` su `debug` per abilitare la modalità debug:

```console
docker run -v $PWD:/app/public \
    -e CADDY_GLOBAL_OPTIONS=debug \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

## Completamento della shell

FrankenPHP fornisce il supporto integrato per il completamento della shell per Bash, Zsh, Fish e PowerShell. Ciò abilita il completamento automatico per tutti i comandi (inclusi i comandi personalizzati come `php-server`, `php-cli` e `extension-init`) e i relativi flag.

### Bash

Per caricare i completamenti nella sessione di shell corrente:

```console
source <(frankenphp completion bash)
```

Per caricare i completamenti per ogni nuova sessione, eseguire:

**Linux:**

```console
frankenphp completion bash > /usr/share/bash-completion/completions/frankenphp
```

**macOS:**

```console
frankenphp completion bash > $(brew --prefix)/share/bash-completion/completions/frankenphp
```

### Zsh

Se il completamento della shell non è già abilitato nell'ambiente, andrà abilitato. È possibile eseguire quanto segue una volta:

```console
echo "autoload -U compinit; compinit" >> ~/.zshrc
```

Per caricare i completamenti per ogni sessione, eseguire una volta:

```console
frankenphp completion zsh > "${fpath[1]}/_frankenphp"
```

Sarà necessario avviare una nuova shell affinché questa configurazione abbia effetto.

### Fish

Per caricare i completamenti nella sessione di shell corrente:

```console
frankenphp completion fish | source
```

Per caricare i completamenti per ogni nuova sessione, eseguire una volta:

```console
frankenphp completion fish > ~/.config/fish/completions/frankenphp.fish
```

### PowerShell

Per caricare i completamenti nella sessione di shell corrente:

```powershell
frankenphp completion powershell | Out-String | Invoke-Expression
```

Per caricare i completamenti per ogni nuova sessione, eseguire una volta:

```powershell
frankenphp completion powershell | Out-File -FilePath (Join-Path (Split-Path $PROFILE) "frankenphp.ps1")
Add-Content -Path $PROFILE -Value '. (Join-Path (Split-Path $PROFILE) "frankenphp.ps1")'
```

Sarà necessario avviare una nuova shell affinché questa configurazione abbia effetto.
