# Contribuire

Per una panoramica dell'architettura di FrankenPHP (tipi di thread, macchina a stati, confine CGO, flusso di richiesta), consultare la [documentazione interna](/docs/internals.md).

## Compilazione PHP

### Con Docker (Linux)

Costruire l'immagine Docker di sviluppo:

```console
docker build -t frankenphp-dev -f dev.Dockerfile .
docker run --cap-add=SYS_PTRACE --security-opt seccomp=unconfined -p 8080:8080 -p 443:443 -p 443:443/udp -v $PWD:/go/src/app -it frankenphp-dev
```

L'immagine contiene i consueti strumenti di sviluppo (Go, GDB, Valgrind, Neovim...) e utilizza le seguenti posizioni di impostazione php

- php.ini: `/etc/frankenphp/php.ini` Per impostazione predefinita viene fornito un file php.ini con preimpostazioni di sviluppo.
- file di configurazione aggiuntivi: `/etc/frankenphp/php.d/*.ini`
- estensioni php: `/usr/lib/frankenphp/modules/`

Se la versione di Docker è precedente alla 23.0, la compilazione avrà esito negativo a causa di dockerignore [problema relativo al modello](https://github.com/moby/moby/pull/42676). Aggiungere a `.dockerignore`:

```patch
 !testdata/*.php
 !testdata/*.txt
+!caddy
+!internal
```

### Senza Docker (Linux e macOS)

[Seguire le istruzioni per compilare dai sorgenti](https://frankenphp.dev/docs/compile/) e passare il flag di configurazione `--debug`.

## Esecuzione della suite di test

```console
export CGO_CFLAGS=-O0 -g $(php-config --includes) CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)"
go test -race -v ./...
```

## Modulo Caddy

Costruire Caddy con il modulo FrankenPHP Caddy:

```console
cd caddy/frankenphp/
go build -tags nobadger,nomysql,nopgx
cd ../../
```

Eseguire Caddy con il modulo FrankenPHP Caddy:

```console
cd testdata/
../caddy/frankenphp/frankenphp run
```

Il server è in ascolto su `127.0.0.1:80`:

> [!NOTE]
> Se utilizzi Docker, dovrai associare la porta 80 del container o eseguire l'esecuzione dall'interno del container

```console
curl -vk http://127.0.0.1/phpinfo.php
```

## Server di prova minimo

Costruire il server di test minimo:

```console
cd internal/testserver/
go build
cd ../../
```

Eseguire il server di prova:

```console
cd testdata/
../internal/testserver/testserver
```

Il server è in ascolto su `127.0.0.1:8080`:

```console
curl -v http://127.0.0.1:8080/phpinfo.php
```

## Sviluppo su Windows

1. Configurare Git per utilizzare sempre le terminazioni di riga `lf`

   ```powershell
   git config --global core.autocrlf false
   git config --global core.eol lf
   ```

2. Installare Visual Studio, Git e Go:

   ```powershell
   winget install -e --id Microsoft.VisualStudio.2022.Community --override "--passive --wait --add Microsoft.VisualStudio.Workload.NativeDesktop --add Microsoft.VisualStudio.Component.VC.Llvm.Clang --includeRecommended"
   winget install -e --id GoLang.Go
   winget install -e --id Git.Git
   ```

3. Installare vcpkg:

   ```powershell
   cd C:\
   git clone https://github.com/microsoft/vcpkg
   .\vcpkg\bootstrap-vcpkg.bat
   ```

4. [Scaricare l'ultima versione della libreria watcher per Windows](https://github.com/e-dant/watcher/releases) ed estraila in una cartella denominata `C:\watcher`
5. [Scaricare l'ultima versione **Thread Safe** di PHP e di PHP SDK per Windows](https://windows.php.net/download/), estraili nelle cartelle denominate `C:\php` e `C:\php-devel`
6. Clonare il repository Git di FrankenPHP:

   ```powershell
   git clone https://github.com/php/frankenphp C:\frankenphp
   cd C:\frankenphp
   ```

7. Installare le dipendenze:

   ```powershell
   C:\vcpkg\vcpkg.exe install
   ```

8. Configurare le variabili di ambiente necessarie (PowerShell):

   ```powershell
   $env:PATH += ';C:\Program Files\Microsoft Visual Studio\2022\Community\VC\Tools\Llvm\bin'
   $env:CC = 'clang'
   $env:CXX = 'clang++'
   $env:CGO_CFLAGS = "-O0 -g -IC:\frankenphp\vcpkg_installed\x64-windows\include -IC:\watcher -IC:\php-devel\include -IC:\php-devel\include\main -IC:\php-devel\include\TSRM -IC:\php-devel\include\Zend -IC:\php-devel\include\ext"
   $env:CGO_LDFLAGS = '-LC:\frankenphp\vcpkg_installed\x64-windows\lib -lbrotlienc -LC:\watcher -llibwatcher-c -LC:\php -LC:\php-devel\lib -lphp8ts -lphp8embed'
   ```

9. Eseguire i test:

   ```powershell
   go test -race -ldflags '-extldflags="-fuse-ld=lld"' ./...
   cd caddy
   go test -race -ldflags '-extldflags="-fuse-ld=lld"' -tags nobadger,nomysql,nopgx ./...
   cd ..
   ```

10. Costruire il binario:

    ```powershell
    cd caddy/frankenphp
    go build -ldflags '-extldflags="-fuse-ld=lld"' -tags nobadger,nomysql,nopgx
    cd ../..
    ```

## Creazione di immagini Docker localmente

Stampare il piano di compilazione:

```console
docker buildx bake -f docker-bake.hcl --print
```

Creare immagini FrankenPHP per amd64 localmente:

```console
docker buildx bake -f docker-bake.hcl --pull --load --set "*.platform=linux/amd64"
```

Creare immagini FrankenPHP per arm64 localmente:

```console
docker buildx bake -f docker-bake.hcl --pull --load --set "*.platform=linux/arm64"
```

Creare immagini FrankenPHP da zero per arm64 e amd64 e inviale a Docker Hub:

```console
docker buildx bake -f docker-bake.hcl --pull --no-cache --push
```

## Debug degli errori di segmentazione con build statiche

1. Scaricare la versione di debug del binario FrankenPHP da GitHub o creare la propria build statica personalizzata includendo i simboli di debug:

   ```console
   docker buildx bake \
       --load \
       --set static-builder.args.DEBUG_SYMBOLS=1 \
       --set "static-builder.platform=linux/amd64" \
       static-builder
   docker cp $(docker create --name static-builder-musl dunglas/frankenphp:static-builder-musl):/go/src/app/dist/frankenphp-linux-$(uname -m) frankenphp
   ```

2. Sostituire la versione corrente di `frankenphp` con l'eseguibile di debug FrankenPHP
3. Avviare FrankenPHP come al solito (in alternativa si può avviare direttamente FrankenPHP con GDB: `gdb --args frankenphp run`)
4. Accedere al processo con GDB:

   ```console
   gdb -p `pidof frankenphp`
   ```

5. Se necessario, digitare `continue` nella shell GDB
6. Mandare in crash FrankenPHP
7. Digitarere `bt` nella shell GDB
8. Copiare l'output

## Debug degli errori di segmentazione nelle azioni GitHub

1. Aprire `.github/workflows/tests.yml`
2. Abilitare i simboli di debug PHP

   ```patch
       - uses: shivammathur/setup-php@v2
         # ...
         env:
           phpts: ts
   +       debug: true
   ```

3. Abilitare `tmate` per connettersi al contenitore

   ```patch
       - name: Set CGO flags
         run: echo "CGO_CFLAGS=-O0 -g $(php-config --includes)" >> "$GITHUB_ENV"
   +   - run: |
   +       sudo apt install gdb
   +       mkdir -p /home/runner/.config/gdb/
   +       printf "set auto-load safe-path /\nhandle SIG34 nostop noprint pass" > /home/runner/.config/gdb/gdbinit
   +   - uses: mxschmitt/action-tmate@v3
   ```

4. Connettersi al contenitore
5. Aprire `frankenphp.go`
6. Abilitare `cgosymbolizer`

   ```patch
   -	//_ "github.com/ianlancetaylor/cgosymbolizer"
   +	_ "github.com/ianlancetaylor/cgosymbolizer"
   ```

7. Scaricare il modulo: `go get`
8. Nel contenitore è possibile utilizzare GDB e simili:

   ```console
   go test -tags -c -ldflags=-w
   gdb --args frankenphp.test -test.run ^MyTest$
   ```

9. Una volta risolto il bug, annullare tutte queste modifiche

## Configurazione dell'ambiente di sviluppo (WSL/Unix)

### Configurazione iniziale

Seguire le istruzioni in [compilazione dai sorgenti](https://frankenphp.dev/docs/compile/).
I passaggi presuppongono il seguente ambiente:

- Go installato su `/usr/local/go`
- Sorgente PHP clonato in `~/php-src`
- PHP creato in: `/usr/local/bin/php`
- Sorgente FrankenPHP clonata in `~/frankenphp`

### Configurazione CLion per sviluppo di colla CGO/fonte PHP

1. Installare CLion (sul sistema operativo host)

- Scaricare da [JetBrains](https://www.jetbrains.com/clion/download/)
   - Avviare (se su Windows, in WSL):

     ```bash
     clion &>/dev/null
     ```

2. Aprire il progetto in CLion

- Aprire CLion → Open → Selezionare la cartella `~/frankenphp`
   - Aggiungere una catena di build: Impostazioni → Compilazione, Esecuzione, Distribuzione → Destinazioni di build personalizzate
   - Selezionare un target di build qualsiasi, in `Build` impostare uno strumento esterno (chiamalo ad esempio go build)
   - Impostare uno script wrapper che crei frankenphp, chiamato `go_compile_frankenphp.sh`

   ```bash
   CGO_CFLAGS="-O0 -g" ./go.sh
   ```

- In Programma, selezionare `go_compile_frankenphp.sh`
   - Lasciare gli argomenti vuoti
   - Cartella di lavoro: `~/frankenphp/caddy/frankenphp`

3. Configurare le destinazioni della corsa

- Andare su Esegui → Modifica configurazioni
   - Crea:
     -frankenphp:
       - Tipo: applicazione nativa
       - Target: seleziona il target `go build` creato in precedenza
       - Eseguibile: `~/frankenphp/caddy/frankenphp/frankenphp`
       - Argomenti: gli argomenti con cui su vuole avviare Frankenphp, ad es. `php-cli test.php`

4. Eseguire il debug dei file Go da CLion

- Fare clic con il tasto destro su un file \*.go nella vista Progetto a sinistra
   - Sostituire il tipo di file → C/C++

Ora si possono inserire punti di interruzione nei file C, C++ e Go.
   Per ottenere l'evidenziazione della sintassi per le importazioni da php-src, potrebbe essere necessario comunicare a CLion i percorsi di inclusione. Creare un
   File `compile_flags.txt` in `~/frankenphp` con il seguente contenuto:

   ```gcc
   -I/usr/local/include/php
   -I/usr/local/include/php/Zend
   -I/usr/local/include/php/main
   -I/usr/local/include/php/TSRM
   ```

---

### Configurazione di GoLand per lo sviluppo di FrankenPHP

Utilizzare GoLand per lo sviluppo Go primario, ma il debugger non può eseguire il debug del codice C.

1. Installare GoLand (sul sistema operativo host)

- Scaricare da [JetBrains](https://www.jetbrains.com/go/download/)

     ```bash
     goland &>/dev/null
     ```

2. Aprire in GoLand

- Avviare GoLand → Apri → Selezionare la cartella `~/frankenphp`

---

### Configurazione di Go

- Selezionare Go Build
  - Nome `frankenphp`
  - Tipo di esecuzione: Directory
- Cartella: `~/frankenphp/caddy/frankenphp`
- Cartella di output: `~/frankenphp/caddy/frankenphp`
- Cartella di lavoro: `~/frankenphp/caddy/frankenphp`
- Ambiente (da adattare per il proprio output $(php-config ...)):
  `CGO_CFLAGS=-O0 -g -I/usr/local/include/php -I/usr/local/include/php/main -I/usr/local/include/php/TSRM -I/usr/local/include/php/Zend -I/usr/local/include/php/ext -I/usr/local/include/php/ext/date/lib;CGO_LDFLAGS=-lm -lpthread -lsqlite3 -lxml2 -lbrotlienc -lbrotlidec -lbrotlicommon -lwatcher`
- Vai agli argomenti dello strumento: `-tags=nobadger,nomysql,nopgx`
- Argomenti del programma: ad es. `php-cli -i`

Per eseguire il debug dei file C da GoLand

- Fare clic con il tasto destro su un file \*.c nella vista Progetto a sinistra
- Sostituire il tipo di file → Go

Ora si possono inserire punti di interruzione nei file C, C++ e Go.

---

### Configurazione di GoLand su Windows

1. Seguire la [sezione Sviluppo Windows](#windows-development)

2. Installare GoLand

- Scaricare da [JetBrains](https://www.jetbrains.com/go/download/)
   - Avviare GoLand

3. Aprire in GoLand

- Selezionare **Apri** → Scegliere la cartella in cui `frankenphp` è stato clonato

4. Configurare Go Build

- Andare a **Esegui** → **Modifica configurazioni**
   - Fare clic su **++** e seleziona **Vai a costruire**
   - Nome: `frankenphp`
   - Tipo di esecuzione: **Directory**
   - Cartella: `.\caddy\frankenphp`
   - Cartella di output: `.\caddy\frankenphp`
   - Cartella di lavoro: `.\caddy\frankenphp`
   - Vai agli argomenti dello strumento: `-tags=nobadger,nomysql,nopgx`
   - Variabili d'ambiente: vedere la [sezione Sviluppo Windows](#windows-development)
   - Argomenti del programma: ad es. `php-server`

---

### Note di debug e integrazione

- Utilizzare CLion per eseguire il debug degli interni PHP e del codice colla `cgo`
- Utilizza GoLand per lo sviluppo e il debugging primario di Go
- FrankenPHP può essere aggiunto come configurazione di esecuzione in CLion per il debug C/Go unificato, se necessario, ma l'evidenziazione della sintassi non funzionerà nei file Go

## Risorse varie per lo sviluppo

- [Embed PHP in uWSGI](https://github.com/unbit/uwsgi/blob/master/plugins/php/php_plugin.c)
- [Embed PHP nell'unità NGINX](https://github.com/nginx/unit/blob/master/src/nxt_php_sapi.c)
- [Embed PHP in Go (go-php)](https://github.com/deuill/go-php)
- [Embed PHP in Go (GoEmPHP)](https://github.com/mikespook/goemphp)
- [Embed PHP in C++](https://gist.github.com/paresy/3cbd4c6a469511ac7479aa0e7c42fea7)
- [Estensione ed Embed di PHP di Sara Golemon](https://books.google.fr/books?id=zMbGvK17_tYC&pg=PA254&lpg=PA254#v=onepage&q&f=false)
- [What the heck is TSRMLS_CC, anyway?](http://blog.golemon.com/2006/06/what-heck-is-tsrmlscc-anyway.html)
- [Binding SDL](https://pkg.go.dev/github.com/veandco/go-sdl2@v0.4.21/sdl#Main)

## Risorse relative a Docker

- [Definizione del file Bake](https://docs.docker.com/build/customize/bake/file-definition/)
- [`docker buildx build`](https://docs.docker.com/engine/reference/commandline/buildx_build/)

## Comando utile

```console
apk add strace util-linux gdb
strace -e 'trace=!futex,epoll_ctl,epoll_pwait,tgkill,rt_sigreturn' -p 1
```

## Tradurre la documentazione

Per tradurre la documentazione e il sito in una nuova lingua,
seguire questi passaggi:

1. Creare una nuova cartella chiamata con il codice ISO a due caratteri della lingua nella cartella `docs/` di questo repository
2. Copiare tutti i file `.md` nella radice della cartella `docs/` nella nuova cartella (utilizzare sempre la versione inglese come fonte per la traduzione, poiché è sempre aggiornata)
3. Copiare i file `README.md` e `CONTRIBUTING.md` dalla cartella principale alla nuova cartella
4. Tradurre il contenuto dei file, ma non modificare i nomi dei file, inoltre non tradurre le stringhe che iniziano con `> [!` (è un markup speciale per GitHub)
5. Creare una pull request con le traduzioni
6. Nel [repository del sito](https://github.com/dunglas/frankenphp-website/tree/main), copiare e tradurre i file di traduzione nelle cartelle `content/`, `data/` e `i18n/`
7. Tradurre i valori nel file YAML creato
8. Aprire una pull request sul repository del sito
