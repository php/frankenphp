# Laravel

## Docker

Déployer une application web [Laravel](https://laravel.com) avec FrankenPHP est très facile. Il suffit de monter le projet dans le répertoire `/app` de l'image Docker officielle.

Exécutez cette commande depuis le répertoire principal de votre application Laravel :

```console
docker run -p 80:80 -p 443:443 -p 443:443/udp -v $PWD:/app dunglas/frankenphp
```

Et profitez !

## Installation Locale

Vous pouvez également exécuter vos projets Laravel avec FrankenPHP depuis votre machine locale :

1. [Téléchargez le binaire correspondant à votre système](README.md#binaire-autonome)
2. Ajoutez la configuration suivante dans un fichier nommé `Caddyfile` placé dans le répertoire racine de votre projet Laravel :

   ```caddyfile
   {
   	frankenphp
   }

   # Le nom de domaine de votre serveur
   localhost {
   	# Définir le répertoire racine sur le dossier public/
   	root public/
   	# Autoriser la compression (optionnel)
   	encode zstd br gzip
   	# Exécuter les scripts PHP du dossier public/ et servir les assets
   	php_server {
   		try_files {path} index.php
   	}
   }
   ```

3. Démarrez FrankenPHP depuis le répertoire racine de votre projet Laravel : `frankenphp run`

## Laravel Octane

Octane peut être installé via le gestionnaire de paquets Composer :

```console
composer require laravel/octane
```

Après avoir installé Octane, vous pouvez exécuter la commande Artisan `octane:install`, qui installera le fichier de configuration d'Octane dans votre application :

```console
php artisan octane:install --server=frankenphp
```

Le serveur Octane peut être démarré via la commande Artisan `octane:frankenphp`.

```console
php artisan octane:frankenphp
```

La commande `octane:frankenphp` peut prendre les options suivantes :

- `--host` : L'adresse IP à laquelle le serveur doit se lier (par défaut : `127.0.0.1`)
- `--port` : Le port sur lequel le serveur doit être disponible (par défaut : `8000`)
- `--admin-port` : Le port sur lequel le serveur administratif doit être disponible (par défaut : `2019`)
- `--workers` : Le nombre de workers qui doivent être disponibles pour traiter les requêtes (par défaut : `auto`)
- `--max-requests` : Le nombre de requêtes à traiter avant de recharger le serveur (par défaut : `500`)
- `--caddyfile` : Le chemin vers le fichier `Caddyfile` de FrankenPHP
- `--https` : Activer HTTPS, HTTP/2, et HTTP/3, et générer automatiquement et renouveler les certificats
- `--http-redirect` : Activer la redirection HTTP vers HTTPS (uniquement activé si --https est passé)
- `--watch` : Recharger automatiquement le serveur lorsque l'application est modifiée
- `--poll` : Utiliser le sondage du système de fichiers pendant la surveillance pour surveiller les fichiers sur un réseau
- `--log-level` : Enregistrer les messages au niveau de journalisation spécifié ou au-dessus, en utilisant le logger natif de Caddy

> [!TIP]
> Pour obtenir des logs structurés en JSON logs (utile quand vous utilisez des solutions d'analyse de logs), passez explicitement l'option `--log-level`.

En savoir plus sur Laravel Octane [dans sa documentation officielle](https://laravel.com/docs/octane).

## Les Applications Laravel En Tant Que Binaires Autonomes

En utilisant la [fonctionnalité d'intégration d'applications de FrankenPHP](embed.md), il est possible de distribuer
les applications Laravel sous forme de binaires autonomes.

Suivez ces étapes pour empaqueter votre application Laravel en tant que binaire autonome pour Linux :

1. Créez un fichier nommé `static-build.Dockerfile` dans le dépôt de votre application :

   ```dockerfile
   FROM --platform=linux/amd64 dunglas/frankenphp:static-builder-gnu
   # Si vous avez l'intention d'exécuter le binaire sur des systèmes musl-libc, utilisez plutôt static-builder-musl

   # Copiez votre application
   WORKDIR /go/src/app/dist/app
   COPY . .

   # Supprimez les tests et autres fichiers inutiles pour gagner de la place
   # Alternativement, ajoutez ces fichiers à un fichier .dockerignore
   RUN rm -Rf tests/

   # Copiez le fichier .env
   RUN cp .env.example .env
   # Modifier APP_ENV et APP_DEBUG pour qu'ils soient prêts pour la production
   RUN sed -i'' -e 's/^APP_ENV=.*/APP_ENV=production/' -e 's/^APP_DEBUG=.*/APP_DEBUG=false/' .env

   # Apportez d'autres modifications à votre fichier .env si nécessaire

   # Installez les dépendances
   RUN composer install --ignore-platform-reqs --no-dev -a

   # Construire le binaire statique
   WORKDIR /go/src/app/
   RUN EMBED=/go/src/app/dist/app/ ./build-static.sh
   ```

   > [!CAUTION]
   >
   > Certains fichiers `.dockerignore` ignoreront le répertoire `vendor/`
   > et les fichiers `.env`. Assurez-vous d'ajuster ou de supprimer le fichier `.dockerignore` avant la construction.

2. Build:

   ```console
   docker build -t static-laravel-app -f static-build.Dockerfile .
   ```

3. Extraire le binaire

   ```console
   docker cp $(docker create --name static-laravel-app-tmp static-laravel-app):/go/src/app/dist/frankenphp-linux-x86_64 frankenphp ; docker rm static-laravel-app-tmp
   ```

4. Remplir les caches :

   ```console
   frankenphp php-cli artisan optimize
   ```

5. Exécutez les migrations de base de données (s'il y en a) :

   ```console
   frankenphp php-cli artisan migrate
   ```

6. Générer la clé secrète de l'application :

   ```console
   frankenphp php-cli artisan key:generate
   ```

7. Démarrez le serveur:

   ```console
   frankenphp php-server
   ```

Votre application est maintenant prête !

Pour en savoir plus sur les options disponibles et sur la construction de binaires pour d'autres systèmes d'exploitation,
consultez la documentation [Applications PHP en tant que binaires autonomes](embed.md).

### Changer le chemin de stockage

Par défaut, Laravel stocke les fichiers téléchargés, les caches, les logs, etc. dans le répertoire `storage/` de l'application.
Ceci n'est pas adapté aux applications embarquées, car chaque nouvelle version sera extraite dans un répertoire temporaire différent.

Définissez la variable d'environnement `LARAVEL_STORAGE_PATH` (par exemple, dans votre fichier `.env`) ou appelez la méthode `Illuminate\Foundation\Application::useStoragePath()` pour utiliser un répertoire en dehors du répertoire temporaire.

### Exécuter Octane avec des binaires autonomes

Il est même possible d'empaqueter les applications Laravel Octane en tant que binaires autonomes !

Pour ce faire, [installez Octane correctement](#laravel-octane) et suivez les étapes décrites dans [la section précédente](#les-applications-laravel-en-tant-que-binaires-autonomes).

Ensuite, pour démarrer FrankenPHP en mode worker via Octane, exécutez :

```console
PATH="$PWD:$PATH" frankenphp php-cli artisan octane:frankenphp
```

> [!CAUTION]
>
> Pour que la commande fonctionne, le binaire autonome **doit** être nommé `frankenphp`
> car Octane a besoin d'un programme nommé `frankenphp` disponible dans le chemin
