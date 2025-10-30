# Applications PHP en tant que binaires autonomes

FrankenPHP a la capacité d'incorporer le code source et les assets des applications PHP dans un binaire statique et autonome.

Grâce à cette fonctionnalité, les applications PHP peuvent être distribuées en tant que binaires autonomes qui incluent l'application elle-même, l'interpréteur PHP et Caddy, un serveur web de qualité production.

Pour en savoir plus sur cette fonctionnalité, consultez [la présentation faite par Kévin à la SymfonyCon 2023](https://dunglas.dev/2023/12/php-and-symfony-apps-as-standalone-binaries/).

Pour embarquer des applications Laravel, [lisez ce point spécifique de la documentation](laravel.md#les-applications-laravel-en-tant-que-binaires-autonomes).

## Préparer votre application

Avant de créer le binaire autonome, assurez-vous que votre application est prête à être intégrée.

Vous devrez probablement :

- Installer les dépendances de production de l'application
- Dumper l'autoloader
- Activer le mode production de votre application (si disponible)
- Supprimer les fichiers inutiles tels que `.git` ou les tests pour réduire la taille de votre binaire final

Par exemple, pour une application Symfony, lancez les commandes suivantes :

```console
# Exporter le projet pour se débarrasser de .git/, etc.
mkdir $TMPDIR/my-prepared-app
git archive HEAD | tar -x -C $TMPDIR/my-prepared-app
cd $TMPDIR/my-prepared-app

# Définir les variables d'environnement appropriées
echo APP_ENV=prod > .env.local
echo APP_DEBUG=0 >> .env.local

# Supprimer les tests et autres fichiers inutiles pour économiser de l'espace
# Alternativement, ajoutez ces fichiers avec l'attribut export-ignore dans votre fichier .gitattributes
rm -Rf tests/

# Installer les dépendances
composer install --ignore-platform-reqs --no-dev -a

# Optimiser le .env
composer dump-env prod
```

### Personnaliser la configuration

Pour personnaliser [la configuration](config.md),
vous pouvez mettre un fichier `Caddyfile` ainsi qu'un fichier `php.ini`
dans le répertoire principal de l'application à intégrer
(`$TMPDIR/my-prepared-app` dans l'exemple précédent).

## Créer un binaire Linux

La manière la plus simple de créer un binaire Linux est d'utiliser le builder basé sur Docker que nous fournissons.

1. Créez un fichier nommé `static-build.Dockerfile` dans le répertoire de votre application préparée :

   ```dockerfile
   FROM dunglas/frankenphp:static-builder-gnu
   # Si vous envisagez d'exécuter le binaire sur des systèmes musl-libc, utilisez plutôt static-builder-musl

   # Copy your app
   WORKDIR /go/src/app/dist/app
   COPY . .

   WORKDIR /go/src/app/
   RUN EMBED=dist/app/ ./build-static.sh
   ```

   > [!CAUTION]
   >
   > Certains fichiers `.dockerignore` (par exemple celui fourni par défaut par [Symfony Docker](https://github.com/dunglas/symfony-docker/blob/main/.dockerignore))
   > empêchent la copie du dossier `vendor/` et des fichiers `.env`. Assurez-vous d'ajuster ou de supprimer le fichier `.dockerignore` avant le build.

2. Construisez:

   ```console
   docker build -t static-app -f static-build.Dockerfile .
   ```

3. Extrayez le binaire :

   ```console
   docker cp $(docker create --name static-app-tmp static-app):/go/src/app/dist/frankenphp-linux-x86_64 my-app ; docker rm static-app-tmp
   ```

Le binaire généré sera nommé `my-app` dans le répertoire courant.

## Créer un binaire pour d'autres systèmes d'exploitation

Si vous ne souhaitez pas utiliser Docker, ou souhaitez construire un binaire macOS, utilisez le script shell que nous fournissons :

```console
git clone https://github.com/php/frankenphp
cd frankenphp
EMBED=/path/to/your/app ./build-static.sh
```

Le binaire obtenu est le fichier nommé `frankenphp-<os>-<arch>` dans le répertoire `dist/`.

## Utiliser le binaire

C'est tout ! Le fichier `my-app` (ou `dist/frankenphp-<os>-<arch>` sur d'autres systèmes d'exploitation) contient votre application autonome !

Pour démarrer l'application web, exécutez :

```console
./my-app php-server
```

Si votre application contient un [script worker](worker.md), démarrez le worker avec quelque chose comme :

```console
./my-app php-server --worker public/index.php
```

Pour activer HTTPS (un certificat Let's Encrypt est automatiquement créé), HTTP/2 et HTTP/3, spécifiez le nom de domaine à utiliser :

```console
./my-app php-server --domain localhost
```

Vous pouvez également exécuter les scripts CLI PHP incorporés dans votre binaire :

```console
./my-app php-cli bin/console
```

## Extensions PHP

Par défaut, le script construira les extensions requises par le fichier `composer.json` de votre projet, s'il y en a.
Si le fichier `composer.json` n'existe pas, les extensions par défaut sont construites, comme documenté dans [Créer un binaire statique](static.md).

Pour personnaliser les extensions, utilisez la variable d'environnement `PHP_EXTENSIONS`.

```console
EMBED=/path/to/your/app \
PHP_EXTENSIONS=ctype,iconv,pdo_sqlite \
./build-static.sh
```

## Personnaliser la compilation

[Consultez la documentation sur la compilation statique](static.md) pour voir comment personnaliser le binaire (extensions, version PHP...).

## Distribuer le binaire

Sous Linux, le binaire est compressé par défaut à l'aide de [UPX](https://upx.github.io).

Sous Mac, pour réduire la taille du fichier avant de l'envoyer, vous pouvez le compresser.
Nous recommandons `xz`.
