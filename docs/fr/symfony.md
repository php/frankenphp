# Symfony

## Exécuter Symfony avec Symfony Docker

Pour les projets [Symfony](https://symfony.com), nous recommandons d'utiliser [Symfony Docker](https://github.com/dunglas/symfony-docker), la configuration Docker officielle de Symfony maintenue par l'auteur de FrankenPHP. Elle fournit un environnement complet basé sur Docker avec FrankenPHP, HTTPS automatique, HTTP/2, HTTP/3 et le support du mode worker prêts à l'emploi.

## Installer Symfony avec FrankenPHP en local

Vous pouvez également exécuter vos projets Symfony avec FrankenPHP depuis votre machine locale :

1. [Installez FrankenPHP](../#getting-started)
2. Ajoutez la configuration suivante dans un fichier nommé `Caddyfile` à la racine de votre projet Symfony :

   ```caddyfile
   # Caddyfile
   # Le nom de domaine de votre serveur
   localhost

   root public/
   php_server {
   	# Optionnel : activer le mode worker pour de meilleures performances
   	worker ./public/index.php
   }
   ```

   Consultez la [documentation sur les performances](performance.md) pour d'autres optimisations.

3. Démarrez FrankenPHP depuis la racine de votre projet Symfony : `frankenphp run`

## Le mode worker de Symfony avec FrankenPHP

Depuis Symfony 7.4, le mode worker de FrankenPHP est pris en charge nativement.

Pour les versions antérieures, installez le package FrankenPHP de [PHP Runtime](https://github.com/php-runtime/runtime) :

```console
composer require runtime/frankenphp-symfony
```

Démarrez votre serveur d'application en définissant la variable d'environnement `APP_RUNTIME` pour utiliser le Runtime Symfony de FrankenPHP :

```console
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php" \
    -e APP_RUNTIME=Runtime\\FrankenPhpSymfony\\Runtime \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

En savoir plus sur [le mode worker](worker.md).

### Vérifier la compatibilité avec le mode worker

[Igor PHP](https://github.com/igor-php/igor-php) est un linter statique qui analyse les projets Symfony pour détecter les fuites d'état avant qu'elles ne posent problème en production : services n'implémentant pas `ResetInterface`, propriétés avec état qui ne sont pas réinitialisées, variables statiques locales mutables, appels à `exit()`/`die()` et écritures dans les superglobales. Il audite le code de votre application ainsi que les services déclarés dans `vendor/`.

```console
composer require --dev igor-php/igor-php
vendor/bin/igor-php .
```

## Rechargement à chaud pour Symfony

Le rechargement à chaud est activé par défaut dans [Symfony Docker](https://github.com/dunglas/symfony-docker).

Pour utiliser la fonctionnalité de [rechargement à chaud](hot-reload.md) sans Symfony Docker, activez [Mercure](mercure.md) et ajoutez la sous-directive `hot_reload` à la directive `php_server` de votre `Caddyfile` :

```caddyfile
localhost

mercure {
	anonymous
}

root public/
php_server {
	hot_reload
	worker ./public/index.php
}
```

Ajoutez ensuite le code suivant à votre fichier `templates/base.html.twig` :

```twig
{# templates/base.html.twig #}
{% if app.request.server.has('FRANKENPHP_HOT_RELOAD') %}
    <meta name="frankenphp-hot-reload:url" content="{{ app.request.server.get('FRANKENPHP_HOT_RELOAD') }}">
    <script src="https://cdn.jsdelivr.net/npm/idiomorph/dist/idiomorph.min.js"></script>
    <script src="https://cdn.jsdelivr.net/npm/frankenphp-hot-reload/+esm" type="module"></script>
{% endif %}
```

Enfin, lancez `frankenphp run` depuis la racine de votre projet Symfony.

## Pré-compression des assets

Le [composant AssetMapper](https://symfony.com/doc/current/frontend/asset_mapper.html) de Symfony peut pré-compresser les assets avec Brotli et Zstandard lors du déploiement. FrankenPHP (via le `file_server` de Caddy) peut servir directement ces fichiers pré-compressés, ce qui évite le coût de la compression à la volée.

1. Compilez et compressez vos assets :

   ```console
   php bin/console asset-map:compile
   ```

2. Mettez à jour votre `Caddyfile` pour servir les assets pré-compressés :

   ```caddyfile
   # Caddyfile
   localhost

   @assets path /assets/*
   file_server @assets {
   	precompressed zstd br gzip
   }

   root public/
   php_server {
   	worker ./public/index.php
   }
   ```

La directive `precompressed` indique à Caddy de rechercher les versions pré-compressées du fichier demandé (par exemple `app.css.zst`, `app.css.br`) et de les servir directement si le client les prend en charge.

## Servir des fichiers statiques volumineux (`X-Sendfile`)

FrankenPHP permet de [servir efficacement des fichiers statiques volumineux](x-sendfile.md) après l'exécution de code PHP (pour le contrôle d'accès, les statistiques, etc.).

Symfony HttpFoundation [prend en charge nativement cette fonctionnalité](https://symfony.com/doc/current/components/http_foundation.html#serving-files).
Après avoir [configuré votre `Caddyfile`](x-sendfile.md#configuration), il déterminera automatiquement la bonne valeur de l'en-tête `X-Accel-Redirect` et l'ajoutera à la réponse :

```php
use Symfony\Component\HttpFoundation\BinaryFileResponse;

BinaryFileResponse::trustXSendfileTypeHeader();
$response = new BinaryFileResponse(__DIR__.'/../private-files/file.txt');

// ...
```

## Applications Symfony sous forme de binaires autonomes

Grâce à la [fonctionnalité d'intégration d'applications de FrankenPHP](embed.md), il est possible de distribuer des applications Symfony
sous forme de binaires autonomes.

Suivez ces étapes pour préparer et empaqueter votre application Symfony :

1. Préparez votre application :

   ```console
   # Exporter le projet pour se débarrasser de .git/, etc.
   mkdir $TMPDIR/my-prepared-app
   git archive HEAD | tar -x -C $TMPDIR/my-prepared-app
   cd $TMPDIR/my-prepared-app

   # Définir les variables d'environnement appropriées
   echo APP_ENV=prod > .env.local
   echo APP_DEBUG=0 >> .env.local

   # Supprimer les tests et autres fichiers inutiles pour gagner de la place
   # Vous pouvez aussi ajouter ces fichiers avec l'attribut export-ignore dans votre fichier .gitattributes
   rm -Rf tests/

   # Installer les dépendances
   composer install --ignore-platform-reqs --no-dev -a

   # Optimiser .env
   composer dump-env prod
   ```

2. Créez un fichier nommé `static-build.Dockerfile` dans le dépôt de votre application :

   ```dockerfile
   # static-build.Dockerfile
   FROM --platform=linux/amd64 dunglas/frankenphp:static-builder-gnu
   # Si vous prévoyez d'exécuter le binaire sur des systèmes musl-libc, utilisez plutôt static-builder-musl

   # Copier votre application
   WORKDIR /go/src/app/dist/app
   COPY . .

   # Construire le binaire statique
   WORKDIR /go/src/app/
   RUN EMBED=dist/app/ ./build-static.sh
   ```

   > [!CAUTION]
   >
   > Certains fichiers `.dockerignore` (par exemple le [`.dockerignore` par défaut de Symfony Docker](https://github.com/dunglas/symfony-docker/blob/main/.dockerignore))
   > ignorent le répertoire `vendor/` et les fichiers `.env`. Pensez à adapter ou supprimer le fichier `.dockerignore` avant le build.

3. Construisez :

   ```console
   docker build -t static-symfony-app -f static-build.Dockerfile .
   ```

4. Extrayez le binaire :

   ```console
   docker cp $(docker create --name static-symfony-app-tmp static-symfony-app):/go/src/app/dist/frankenphp-linux-x86_64 my-app ; docker rm static-symfony-app-tmp
   ```

5. Démarrez le serveur :

   ```console
   ./my-app php-server
   ```

Pour en savoir plus sur les options disponibles et sur la compilation de binaires pour d'autres systèmes d'exploitation, consultez la documentation sur
l'[intégration d'applications](embed.md).
