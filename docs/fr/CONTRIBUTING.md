# Contribuer

## Compiler PHP

### Avec Docker (Linux)

Construisez l'image Docker de développement :

```console
docker build -t frankenphp-dev -f dev.Dockerfile .
docker run --cap-add=SYS_PTRACE --security-opt seccomp=unconfined -p 8080:8080 -p 443:443 -p 443:443/udp -v $PWD:/go/src/app -it frankenphp-dev
```

L'image contient les outils de développement habituels (Go, GDB, Valgrind, Neovim...) et utilise les emplacements de configuration PHP suivants

- php.ini: `/etc/frankenphp/php.ini` Un fichier php.ini avec des préréglages de développement est fourni par défaut.
- fichiers de configuration supplémentaires: `/etc/frankenphp/php.d/*.ini`
- extensions php: `/usr/lib/frankenphp/modules/`

Si votre version de Docker est inférieure à 23.0, la construction échouera à cause d'un [problème de pattern](https://github.com/moby/moby/pull/42676) dans `.dockerignore`. Ajoutez les répertoires à `.dockerignore`.

```patch
 !testdata/*.php
 !testdata/*.txt
+!caddy
+!internal
```

### Sans Docker (Linux et macOS)

[Suivez les instructions pour compiler à partir des sources](compile.md) et passez l'indicateur de configuration `--debug`.

## Exécution de la suite de tests

```console
go test -tags watcher -race -v ./...
```

## Module Caddy

Construire Caddy avec le module FrankenPHP :

```console
cd caddy/frankenphp/
go build -tags watcher,brotli,nobadger,nomysql,nopgx
cd ../../
```

Exécuter Caddy avec le module FrankenPHP :

```console
cd testdata/
../caddy/frankenphp/frankenphp run
```

Le serveur est configuré pour écouter à l'adresse `127.0.0.1:80`:

> [!NOTE]
>
> Si vous utilisez Docker, vous devrez soit lier le port 80 du conteneur, soit exécuter depuis l'intérieur du conteneur.

```console
curl -vk http://127.0.0.1/phpinfo.php
```

## Serveur de test minimal

Construire le serveur de test minimal :

```console
cd internal/testserver/
go build
cd ../../
```

Lancer le test serveur :

```console
cd testdata/
../internal/testserver/testserver
```

Le serveur est configuré pour écouter à l'adresse `127.0.0.1:8080`:

```console
curl -v http://127.0.0.1:8080/phpinfo.php
```

## Construire localement les images Docker

Afficher le plan de compilation :

```console
docker buildx bake -f docker-bake.hcl --print
```

Construire localement les images FrankenPHP pour amd64 :

```console
docker buildx bake -f docker-bake.hcl --pull --load --set "*.platform=linux/amd64"
```

Construire localement les images FrankenPHP pour arm64 :

```console
docker buildx bake -f docker-bake.hcl --pull --load --set "*.platform=linux/arm64"
```

Construire à partir de zéro les images FrankenPHP pour arm64 & amd64 et les pousser sur Docker Hub :

```console
docker buildx bake -f docker-bake.hcl --pull --no-cache --push
```

## Déboguer les erreurs de segmentation avec les builds statiques

1. Téléchargez la version de débogage du binaire FrankenPHP depuis GitHub ou créez votre propre build statique incluant des symboles de débogage :

   ```console
   docker buildx bake \
       --load \
       --set static-builder.args.DEBUG_SYMBOLS=1 \
       --set "static-builder.platform=linux/amd64" \
       static-builder
   docker cp $(docker create --name static-builder-musl dunglas/frankenphp:static-builder-musl):/go/src/app/dist/frankenphp-linux-$(uname -m) frankenphp
   ```

2. Remplacez votre version actuelle de `frankenphp` par l'exécutable de débogage de FrankenPHP.
3. Démarrez FrankenPHP comme d'habitude (alternativement, vous pouvez directement démarrer FrankenPHP avec GDB : `gdb --args frankenphp run`).
4. Attachez-vous au processus avec GDB :

   ```console
   gdb -p `pidof frankenphp`
   ```

5. Si nécessaire, tapez `continue` dans le shell GDB
6. Faites planter FrankenPHP.
7. Tapez `bt` dans le shell GDB
8. Copiez la sortie

## Déboguer les erreurs de segmentation dans GitHub Actions

1. Ouvrir `.github/workflows/tests.yml`
2. Activer les symboles de débogage de la bibliothèque PHP

   ```patch
       - uses: shivammathur/setup-php@v2
         # ...
         env:
           phpts: ts
   +       debug: true
   ```

3. Activer `tmate` pour se connecter au conteneur

   ```patch
       - name: Set CGO flags
         run: echo "CGO_CFLAGS=$(php-config --includes)" >> "$GITHUB_ENV"
   +   - run: |
   +       sudo apt install gdb
   +       mkdir -p /home/runner/.config/gdb/
   +       printf "set auto-load safe-path /\nhandle SIG34 nostop noprint pass" > /home/runner/.config/gdb/gdbinit
   +   - uses: mxschmitt/action-tmate@v3
   ```

4. Se connecter au conteneur
5. Ouvrir `frankenphp.go`
6. Activer `cgosymbolizer`

   ```patch
   - //_ "github.com/ianlancetaylor/cgosymbolizer"
   + _ "github.com/ianlancetaylor/cgosymbolizer"
   ```

7. Télécharger le module : `go get`
8. Dans le conteneur, vous pouvez utiliser GDB et similaires :

   ```console
   go test -tags watcher -c -ldflags=-w
   gdb --args frankenphp.test -test.run ^MyTest$
   ```

9. Quand le bug est corrigé, annulez tous les changements.

## Ressources Diverses pour le Développement

- [Intégration de PHP dans uWSGI](https://github.com/unbit/uwsgi/blob/master/plugins/php/php_plugin.c)
- [Intégration de PHP dans NGINX Unit](https://github.com/nginx/unit/blob/master/src/nxt_php_sapi.c)
- [Intégration de PHP dans Go (go-php)](https://github.com/deuill/go-php)
- [Intégration de PHP dans Go (GoEmPHP)](https://github.com/mikespook/goemphp)
- [Intégration de PHP dans C++](https://gist.github.com/paresy/3cbd4c6a469511ac7479aa0e7c42fea7)
- [Extending and Embedding PHP par Sara Golemon](https://books.google.fr/books?id=zMbGvK17_tYC&pg=PA254&lpg=PA254#v=onepage&q&f=false)
- [Qu'est-ce que TSRMLS_CC, au juste ?](http://blog.golemon.com/2006/06/what-heck-is-tsrmlscc-anyway.html)
- [Intégration de PHP sur Mac](https://gist.github.com/jonnywang/61427ffc0e8dde74fff40f479d147db4)
- [Bindings SDL](https://pkg.go.dev/github.com/veandco/go-sdl2@v0.4.21/sdl#Main)

## Ressources Liées à Docker

- [Définition du fichier Bake](https://docs.docker.com/build/customize/bake/file-definition/)
- [`docker buildx build`](https://docs.docker.com/engine/reference/commandline/buildx_build/)

## Commande utile

```console
apk add strace util-linux gdb
strace -e 'trace=!futex,epoll_ctl,epoll_pwait,tgkill,rt_sigreturn' -p 1
```

## Traduire la documentation

Pour traduire la documentation et le site dans une nouvelle langue, procédez comme suit :

1. Créez un nouveau répertoire nommé avec le code ISO à 2 caractères de la langue dans le répertoire `docs/` de ce dépôt
2. Copiez tous les fichiers `.md` à la racine du répertoire `docs/` dans le nouveau répertoire (utilisez toujours la version anglaise comme source de traduction, car elle est toujours à jour).
3. Copiez les fichiers `README.md` et `CONTRIBUTING.md` du répertoire racine vers le nouveau répertoire.
4. Traduisez le contenu des fichiers, mais ne changez pas les noms de fichiers, ne traduisez pas non plus les chaînes commençant par `> [!` (c'est un balisage spécial pour GitHub).
5. Créez une Pull Request avec les traductions
6. Dans le [référentiel du site](https://github.com/dunglas/frankenphp-website/tree/main), copiez et traduisez les fichiers de traduction dans les répertoires `content/`, `data/` et `i18n/`.
7. Traduire les valeurs dans le fichier YAML créé.
8. Ouvrir une Pull Request sur le dépôt du site.
