# Créer un binaire statique

Au lieu d'utiliser une installation locale de la bibliothèque PHP, il est possible de créer un build statique de FrankenPHP grâce à l'excellent projet [static-php-cli](https://github.com/crazywhalecc/static-php-cli) (malgré son nom, ce projet prend en charge tous les SAPIs, pas seulement CLI).

Avec cette méthode, un binaire portable unique contiendra l'interpréteur PHP, le serveur web Caddy et FrankenPHP !

Les exécutables natifs entièrement statiques ne nécessitent aucune dépendance et peuvent même être exécutés sur une [image Docker `scratch`](https://docs.docker.com/build/building/base-images/#create-a-minimal-base-image-using-scratch).
Cependant, ils ne peuvent pas charger les extensions dynamiques de PHP (comme Xdebug) et ont quelques limitations parce qu'ils utilisent la librairie musl.

La plupart des binaires statiques ne nécessitent que la `glibc` et peuvent charger des extensions dynamiques.

Lorsque c'est possible, nous recommandons d'utiliser des binaires statiques basés sur la glibc.

FrankenPHP permet également [d'embarquer l'application PHP dans le binaire statique](embed.md).

## Linux

Nous fournissons des images Docker pour créer des binaires statiques pour Linux :

### Build entièrement statique, basé sur musl

Pour un binaire entièrement statique qui fonctionne sur n'importe quelle distribution Linux sans dépendances,
mais qui ne prend pas en charge le chargement dynamique des extensions :

```console
docker buildx bake --load static-builder-musl
docker cp $(docker create --name static-builder-musl dunglas/frankenphp:static-builder-musl):/go/src/app/dist/frankenphp-linux-$(uname -m) frankenphp ; docker rm static-builder-musl
```

Pour améliorer les performances dans les scénarios fortement concurrents, envisagez d'utiliser l'allocateur [mimalloc](https://github.com/microsoft/mimalloc).

```console
docker buildx bake --load --set static-builder-musl.args.MIMALLOC=1 static-builder-musl
```

### Construction principalement statique (avec prise en charge des extensions dynamiques), basé sur la glibc

Pour un binaire qui supporte le chargement dynamique des extensions PHP tout en ayant les extensions sélectionnées compilées statiquement :

```console
docker buildx bake --load static-builder-gnu
docker cp $(docker create --name static-builder-gnu dunglas/frankenphp:static-builder-gnu):/go/src/app/dist/frankenphp-linux-$(uname -m) frankenphp ; docker rm static-builder-gnu
```

Ce binaire supporte toutes les versions 2.17 et supérieures de la glibc mais ne fonctionne pas sur les systèmes basés sur musl (comme Alpine Linux).

Le binaire résultant, principalement statique (à l'exception de `glibc`), est nommé `frankenphp` et est disponible dans le répertoire courant.

Si vous voulez construire le binaire statique sans Docker, jetez un coup d'œil aux instructions pour macOS, qui fonctionnent aussi pour Linux.

### Extensions personnalisées

Par défaut, la plupart des extensions PHP populaires sont compilées.

Pour réduire la taille du binaire et diminuer la surface d'attaque, vous pouvez choisir la liste des extensions à construire en utilisant l'argument Docker `PHP_EXTENSIONS`.

Par exemple, exécutez la commande suivante pour ne construire que l'extension `opcache` :

```console
docker buildx bake --load --set static-builder-musl.args.PHP_EXTENSIONS=opcache,pdo_sqlite static-builder-musl
# ...
```

Pour ajouter des bibliothèques permettant des fonctionnalités supplémentaires aux extensions que vous avez activées, vous pouvez utiliser l'argument Docker `PHP_EXTENSION_LIBS` :

```console
docker buildx bake \
  --load \
  --set static-builder-musl.args.PHP_EXTENSIONS=gd \
  --set static-builder-musl.args.PHP_EXTENSION_LIBS=libjpeg,libwebp \
  static-builder-musl
```

### Modules supplémentaires de Caddy

Pour ajouter des modules Caddy supplémentaires ou passer d'autres arguments à [xcaddy](https://github.com/caddyserver/xcaddy), utilisez l'argument Docker `XCADDY_ARGS` :

```console
docker buildx bake \
  --load \
  --set static-builder-musl.args.XCADDY_ARGS="--with github.com/darkweak/souin/plugins/caddy --with github.com/dunglas/caddy-cbrotli --with github.com/dunglas/mercure/caddy --with github.com/dunglas/vulcain/caddy" \
  static-builder-musl
```

Dans cet exemple, nous ajoutons le module de cache HTTP [Souin](https://souin.io) pour Caddy ainsi que les modules [cbrotli](https://github.com/dunglas/caddy-cbrotli), [Mercure](https://mercure.rocks) et [Vulcain](https://vulcain.rocks).

> [!TIP]
>
> Les modules cbrotli, Mercure et Vulcain sont inclus par défaut si `XCADDY_ARGS` est vide ou n'est pas défini.
> Si vous personnalisez la valeur de `XCADDY_ARGS`, vous devez les inclure explicitement si vous voulez qu'ils soient inclus.

Voir aussi comment [personnaliser la construction](#personnalisation-de-la-construction)

### Jeton GitHub

Si vous atteignez la limite de taux d'appels de l'API GitHub, définissez un jeton d'accès personnel GitHub dans une variable d'environnement nommée `GITHUB_TOKEN` :

```console
GITHUB_TOKEN="xxx" docker --load buildx bake static-builder-musl
# ...
```

## macOS

Exécutez le script suivant pour créer un binaire statique pour macOS (vous devez avoir [Homebrew](https://brew.sh/) d'installé) :

```console
git clone https://github.com/php/frankenphp
cd frankenphp
./build-static.sh
```

Note : ce script fonctionne également sur Linux (et probablement sur d'autres Unix) et est utilisé en interne par le builder statique basé sur Docker que nous fournissons.

## Personnalisation de la construction

Les variables d'environnement suivantes peuvent être transmises à `docker build` et au script `build-static.sh` pour personnaliser la construction statique :

- `FRANKENPHP_VERSION` : la version de FrankenPHP à utiliser
- `PHP_VERSION` : la version de PHP à utiliser
- `PHP_EXTENSIONS` : les extensions PHP à construire ([liste des extensions prises en charge](https://static-php.dev/en/guide/extensions.html))
- `PHP_EXTENSION_LIBS` : bibliothèques supplémentaires à construire qui ajoutent des fonctionnalités aux extensions
- `XCADDY_ARGS` : arguments à passer à [xcaddy](https://github.com/caddyserver/xcaddy), par exemple pour ajouter des modules Caddy supplémentaires
- `EMBED` : chemin de l'application PHP à intégrer dans le binaire
- `CLEAN` : lorsque défini, `libphp` et toutes ses dépendances sont construites à partir de zéro (pas de cache)
- `DEBUG_SYMBOLS` : lorsque défini, les symboles de débogage ne seront pas supprimés et seront ajoutés dans le binaire
- `NO_COMPRESS`: ne pas compresser le binaire avec UPX
- `MIMALLOC`: (expérimental, Linux seulement) remplace l'allocateur mallocng de musl par [mimalloc](https://github.com/microsoft/mimalloc) pour des performances améliorées
- `RELEASE` : (uniquement pour les mainteneurs) lorsque défini, le binaire résultant sera uploadé sur GitHub

## Extensions

Avec la glibc ou les binaires basés sur macOS, vous pouvez charger des extensions PHP dynamiquement. Cependant, ces extensions devront être compilées avec le support ZTS.
Comme la plupart des gestionnaires de paquets ne proposent pas de versions ZTS de leurs extensions, vous devrez les compiler vous-même.

Pour cela, vous pouvez construire et exécuter le conteneur Docker `static-builder-gnu`, vous y connecter à distance et compiler les extensions avec `./configure --with-php-config=/go/src/app/dist/static-php-cli/buildroot/bin/php-config`.

Exemple d'étapes pour [l'extension Xdebug](https://xdebug.org) :

```console
docker build -t gnu-ext -f static-builder-gnu.Dockerfile --build-arg FRANKENPHP_VERSION=1.0 .
docker create --name static-builder-gnu -it gnu-ext /bin/sh
docker start static-builder-gnu
docker exec -it static-builder-gnu /bin/sh
cd /go/src/app/dist/static-php-cli/buildroot/bin
git clone https://github.com/xdebug/xdebug.git && cd xdebug
source scl_source enable devtoolset-10
../phpize
./configure --with-php-config=/go/src/app/dist/static-php-cli/buildroot/bin/php-config
make
exit
docker cp static-builder-gnu:/go/src/app/dist/static-php-cli/buildroot/bin/xdebug/modules/xdebug.so xdebug-zts.so
docker cp static-builder-gnu:/go/src/app/dist/frankenphp-linux-$(uname -m) ./frankenphp
docker stop static-builder-gnu
docker rm static-builder-gnu
docker rmi gnu-ext
```

Cela aura créé `frankenphp` et `xdebug-zts.so` dans le répertoire courant.
Si vous déplacez `xdebug-zts.so` dans votre répertoire d'extension, ajoutez `zend_extension=xdebug-zts.so` à votre php.ini
et lancez FrankenPHP, il chargera Xdebug.
