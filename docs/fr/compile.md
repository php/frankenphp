# Compiler depuis les sources

Ce document explique comment créer un binaire FrankenPHP qui chargera PHP en tant que bibliothèque dynamique.
C'est la méthode recommandée.

Alternativement, des [versions entièrement ou partiellement statiques](static.md) peuvent également être créées.

## Installer PHP

FrankenPHP est compatible avec PHP 8.2 et versions ultérieures.

### Avec Homebrew (Linux et Mac)

La manière la plus simple d'installer une version de libphp compatible avec FrankenPHP est d'utiliser les paquets ZTS fournis par [Homebrew PHP](https://github.com/shivammathur/homebrew-php).

Tout d'abord, si ce n'est déjà fait, installez [Homebrew](https://brew.sh).

Ensuite, installez la variante ZTS de PHP, Brotli (facultatif, pour la prise en charge de la compression) et watcher (facultatif, pour la détection des modifications de fichiers) :

```console
brew install shivammathur/php/php-zts brotli watcher
brew link --overwrite --force shivammathur/php/php-zts
```

### En compilant PHP

Vous pouvez également compiler PHP à partir des sources avec les options requises par FrankenPHP en suivant ces étapes.

Tout d'abord, [téléchargez les sources de PHP](https://www.php.net/downloads.php) et extrayez-les :

```console
tar xf php-*
cd php-*/
```

Ensuite, exécutez le script `configure` avec les options nécessaires pour votre plateforme.
Les `flags` `./configure` suivants sont obligatoires, mais vous pouvez en ajouter d'autres, par exemple, pour compiler des extensions ou des fonctionnalités supplémentaires.

#### Linux

```console
./configure \
    --enable-embed \
    --enable-zts \
    --disable-zend-signals \
    --enable-zend-max-execution-timers
```

#### Mac

Utilisez le gestionnaire de paquets [Homebrew](https://brew.sh/) pour installer les dépendances obligatoires et optionnelles :

```console
brew install libiconv bison brotli re2c pkg-config watcher
echo 'export PATH="/opt/homebrew/opt/bison/bin:$PATH"' >> ~/.zshrc
```

Puis exécutez le script de configuration :

```console
./configure \
    --enable-embed \
    --enable-zts \
    --disable-zend-signals \
    --with-iconv=/opt/homebrew/opt/libiconv/
```

#### Compiler PHP

Finalement, compilez et installez PHP :

```console
make -j"$(getconf _NPROCESSORS_ONLN)"
sudo make install
```

## Installer les dépendances optionnelles

Certaines fonctionnalités de FrankenPHP dépendent de dépendances système optionnelles qui doivent être installées.
Ces fonctionnalités peuvent également être désactivées en passant des tags de compilation au compilateur Go.

| Fonctionnalité                 | Dépendance                                                                                                   | Tag de compilation pour la désactiver |
| ------------------------------ | ------------------------------------------------------------------------------------------------------------ | ------------------------------------- |
| Compression Brotli             | [Brotli](https://github.com/google/brotli)                                                                   | nobrotli                              |
| Redémarrage des workers en cas de changement de fichier | [Watcher C](https://github.com/e-dant/watcher/tree/release/watcher-c)                                        | nowatcher                             |
| [Mercure](mercure.md)          | [Bibliothèque Mercure Go](https://pkg.go.dev/github.com/dunglas/mercure) (installée automatiquement, licence AGPL) | nomercure                             |

## Compiler l'application Go

Vous pouvez maintenant construire le binaire final.

### Utiliser xcaddy

La méthode recommandée consiste à utiliser [xcaddy](https://github.com/caddyserver/xcaddy) pour compiler FrankenPHP.
`xcaddy` permet également d'ajouter facilement des [modules Caddy personnalisés](https://caddyserver.com/docs/modules/) et des extensions FrankenPHP :

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
    # Ajoutez les modules Caddy supplémentaires et les extensions FrankenPHP ici
    # Facultativement, si vous souhaitez compiler à partir de vos sources frankenphp :
    # --with github.com/dunglas/frankenphp=$(pwd) \
    # --with github.com/dunglas/frankenphp/caddy=$(pwd)/caddy
```

> [!TIP]
>
> Si vous utilisez musl libc (la bibliothèque par défaut sur Alpine Linux) et Symfony,
> vous pourriez avoir besoin d'augmenter la taille par défaut de la pile.
> Sinon, vous pourriez rencontrer des erreurs telles que `PHP Fatal error: Maximum call stack size of 83360 bytes reached during compilation. Try splitting expression`
>
> Pour ce faire, modifiez la variable d'environnement `XCADDY_GO_BUILD_FLAGS` en quelque chose comme
> `XCADDY_GO_BUILD_FLAGS=$'-ldflags "-w -s -extldflags \'-Wl,-z,stack-size=0x80000\'"'`
> (modifiez la valeur de la taille de la pile selon les besoins de votre application).

### Sans xcaddy

Il est également possible de compiler FrankenPHP sans `xcaddy` en utilisant directement la commande `go` :

```console
curl -L https://github.com/php/frankenphp/archive/refs/heads/main.tar.gz | tar xz
cd frankenphp-main/caddy/frankenphp
CGO_CFLAGS=$(php-config --includes) CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" go build -tags=nobadger,nomysql,nopgx
