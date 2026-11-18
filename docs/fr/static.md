---
title: FrankenPHP statique : serveur d'application PHP en un seul binaire
description: Créez des binaires FrankenPHP statiques ou principalement statiques avec PHP et Caddy embarqués sur Linux et macOS, personnalisez les extensions et chargez des modules dynamiques avec glibc.
---

# Créer un binaire statique

Au lieu d'utiliser une installation locale de la bibliothèque PHP,
il est possible de créer une version statique ou principalement statique de FrankenPHP grâce à l'excellent projet [static-php-cli](https://github.com/crazywhalecc/static-php-cli) (malgré son nom, ce projet prend en charge tous les SAPIs, pas seulement CLI).

Avec cette méthode, un binaire unique et portable contiendra l'interpréteur PHP, le serveur web Caddy et FrankenPHP !

Les exécutables natifs entièrement statiques ne nécessitent aucune dépendance et peuvent même être exécutés sur une [image Docker `scratch`](https://docs.docker.com/build/building/base-images/#create-a-minimal-base-image-using-scratch).
Cependant, ils ne peuvent pas charger d'extensions PHP dynamiques (telles que Xdebug) et présentent certaines limitations car ils utilisent la libc musl.

Les binaires principalement statiques ne nécessitent que la `glibc` et peuvent charger des extensions dynamiques.

Dans la mesure du possible, nous recommandons d'utiliser des versions principalement statiques basées sur glibc.

FrankenPHP prend également en charge l'[intégration de l'application PHP dans le binaire statique](embed.md).

## Linux

Nous fournissons des images Docker pour construire des binaires Linux statiques :

### Build entièrement statique, basé sur musl

Pour un binaire entièrement statique qui s'exécute sur n'importe quelle distribution Linux sans dépendances, mais qui ne prend pas en charge le chargement dynamique d'extensions :

```console
docker buildx bake --load static-builder-musl
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log dunglas/frankenphp:static-builder-musl
cp "output/frankenphp-linux-$(uname -m)" frankenphp
```

Pour de meilleures performances dans des scénarios fortement concurrents, envisagez d'utiliser l'allocateur [mimalloc](https://github.com/microsoft/mimalloc).

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log -e MIMALLOC=1 dunglas/frankenphp:static-builder-musl
```

### Construction principalement statique (avec prise en charge des extensions dynamiques), basé sur glibc

Pour un binaire qui prend en charge le chargement dynamique des extensions PHP tout en ayant les extensions sélectionnées compilées statiquement :

```console
docker buildx bake --load static-builder-gnu
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log dunglas/frankenphp:static-builder-gnu
cp "output/frankenphp-linux-$(uname -m)" frankenphp
```

Ce binaire prend en charge toutes les versions de glibc 2.28 et supérieures, mais ne s'exécute pas sur les systèmes basés sur musl (comme Alpine Linux).

Le binaire résultant, principalement statique (à l'exception de `glibc`), est nommé `frankenphp` et est disponible dans le répertoire courant.

Si la construction échoue, les logs de static-php-cli se trouvent dans `output/log/`.

Si vous souhaitez construire le binaire statique sans Docker, consultez les instructions macOS, qui fonctionnent également pour Linux.

### Extensions PHP personnalisées dans la construction statique

Par défaut, les extensions PHP les plus populaires sont compilées.
Les valeurs par défaut se trouvent dans [`craft.yml`](https://github.com/php/frankenphp/blob/main/craft.yml).

Pour réduire la taille du binaire et la surface d'attaque, vous pouvez choisir la liste des extensions à construire en utilisant la variable `PHP_EXTENSIONS`.

Par exemple, exécutez la commande suivante pour ne construire que l'extension `opcache` :

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log -e PHP_EXTENSIONS=opcache,pdo_sqlite dunglas/frankenphp:static-builder-musl
# ...
```

Pour ajouter des bibliothèques activant des fonctionnalités supplémentaires aux extensions que vous avez activées, vous pouvez passer la variable `PHP_EXTENSION_LIBS` :

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log \
  -e PHP_EXTENSIONS=gd \
  -e PHP_EXTENSION_LIBS=libjpeg,libwebp \
  dunglas/frankenphp:static-builder-musl
```

### Modules supplémentaires de Caddy

Pour ajouter des modules Caddy supplémentaires ou passer d'autres arguments à [xcaddy](https://github.com/caddyserver/xcaddy), utilisez la variable `XCADDY_ARGS` :

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log \
  -e XCADDY_ARGS="--with github.com/darkweak/souin/plugins/caddy --with github.com/dunglas/caddy-cbrotli --with github.com/dunglas/mercure/caddy --with github.com/dunglas/vulcain/caddy" \
  dunglas/frankenphp:static-builder-musl
```

Dans cet exemple, nous ajoutons le module de cache HTTP [Souin](https://souin.io) pour Caddy ainsi que les modules [cbrotli](https://github.com/dunglas/caddy-cbrotli), [Mercure](https://mercure.rocks) et [Vulcain](https://vulcain.rocks).

> [!TIP]
>
> Les modules cbrotli, Mercure et Vulcain sont inclus par défaut si `XCADDY_ARGS` est vide ou n'est pas défini.
> Si vous personnalisez la valeur de `XCADDY_ARGS`, vous devez les inclure explicitement si vous voulez qu'ils soient inclus.

Voir aussi comment [personnaliser la construction statique de FrankenPHP](#personnalisation-de-la-construction-statique-de-frankenphp)

### Jeton GitHub

Si vous atteignez la limite de taux de l'API GitHub, définissez un jeton d'accès personnel GitHub dans une variable d'environnement nommée `GITHUB_TOKEN` :

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log -e GITHUB_TOKEN="xxx" dunglas/frankenphp:static-builder-musl
# ...
```

## macOS

Exécutez le script suivant pour créer un binaire statique pour macOS (vous devez avoir [Homebrew](https://brew.sh/) installé) :

```console
git clone https://github.com/php/frankenphp
cd frankenphp
./build-static.sh
```

Note : ce script fonctionne également sur Linux (et probablement sur d'autres Unix), et est utilisé en interne par les images Docker que nous fournissons.

Nous ne publions que des binaires macOS pour Apple Silicon. Sur les Mac Intel, construisez à partir des sources avec ce script.

## Personnalisation de la construction statique de FrankenPHP

Les variables d'environnement suivantes peuvent être transmises à `docker run` et au script `build-static.sh`
pour personnaliser la construction statique :

- `FRANKENPHP_VERSION` : la version de FrankenPHP à utiliser
- `PHP_VERSION` : la version de PHP à utiliser
- `PHP_EXTENSIONS` : les extensions PHP à construire ([liste des extensions prises en charge](https://static-php.dev/en/guide/extensions.html))
- `PHP_EXTENSION_LIBS` : bibliothèques supplémentaires à construire qui ajoutent des fonctionnalités aux extensions
- `XCADDY_ARGS` : arguments à passer à [xcaddy](https://github.com/caddyserver/xcaddy), par exemple pour ajouter des modules Caddy supplémentaires
- `EMBED` : chemin de l'application PHP à intégrer dans le binaire
- `CLEAN` : lorsque défini, `libphp` et toutes ses dépendances sont construites à partir de zéro (pas de cache)
- `COMPRESS` : lorsque défini à `1`, compacte le binaire résultant avec UPX (Linux uniquement ; ignoré lorsque `DEBUG_SYMBOLS` est défini)
- `DEBUG_SYMBOLS` : lorsque défini, les symboles de débogage ne seront pas supprimés et seront ajoutés au binaire
- `MIMALLOC` : (expérimental, Linux seulement) remplace l'allocateur mallocng de musl par [mimalloc](https://github.com/microsoft/mimalloc) pour des performances améliorées. Nous recommandons uniquement de l'utiliser pour les constructions ciblant musl ; pour glibc, préférez désactiver cette option et utiliser [`LD_PRELOAD`](https://microsoft.github.io/mimalloc/overrides.html) lorsque vous exécutez votre binaire à la place.
- `RELEASE` : (uniquement pour les mainteneurs) lorsque défini, le binaire résultant sera mis en ligne sur GitHub
- `LIBC` : (Linux uniquement) `musl` pour un binaire entièrement statique (par défaut), ou `gnu` pour lier
  dynamiquement à la glibc du système. C'est un raccourci pour `SPC_TARGET`
  et `SPC_TOOLCHAIN` de static-php-cli, qui peuvent toujours être définis directement ; ce doivent être de vraies variables
  d'environnement car static-php-cli résout sa chaîne d'outils avant de lire `craft.yml`.
  Le réglage par défaut `musl` cible le CPU de base de l'architecture (`x86_64-linux-musl` ou `aarch64-linux-musl`)
  afin que le binaire s'exécute sur n'importe quel CPU ; définissez `SPC_TARGET=native-native-musl` pour optimiser pour la machine de construction.

## Chargement dynamique d'extensions PHP dans le binaire statique

Avec les binaires basés sur glibc ou macOS, vous pouvez charger des extensions PHP dynamiquement. Cependant, ces extensions devront être compilées avec le support ZTS.
Étant donné que la plupart des gestionnaires de paquets n'offrent pas actuellement de versions ZTS de leurs extensions, vous devrez les compiler vous-même.

Pour cela, vous pouvez exécuter le conteneur Docker `static-builder-gnu`, vous y connecter à distance, et compiler les extensions avec `./configure --with-php-config=/go/src/app/dist/buildroot/bin/php-config`.

Exemple d'étapes pour [l'extension Xdebug](https://xdebug.org) :

```console
docker buildx bake --load static-builder-gnu
docker run --rm -v "$PWD/dist:/go/src/app/dist" -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log dunglas/frankenphp:static-builder-gnu
docker run --rm -it -v "$PWD/dist:/go/src/app/dist" dunglas/frankenphp:static-builder-gnu /bin/bash
cd /go/src/app/dist/buildroot/bin
git clone https://github.com/xdebug/xdebug.git && cd xdebug
../phpize
./configure --with-php-config=/go/src/app/dist/buildroot/bin/php-config
make
exit
cp dist/buildroot/bin/xdebug/modules/xdebug.so xdebug-zts.so
cp "output/frankenphp-linux-$(uname -m)" frankenphp
```

Cela aura créé `frankenphp` et `xdebug-zts.so` dans le répertoire courant.
Si vous déplacez le fichier `xdebug-zts.so` dans votre répertoire d'extensions, ajoutez `zend_extension=xdebug-zts.so` à votre php.ini
et exécutez FrankenPHP, il chargera Xdebug.
