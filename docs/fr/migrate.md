---
title: Migrer de Nginx et PHP-FPM vers FrankenPHP
description: Guide étape par étape pour migrer une application PHP d'une pile Nginx + PHP-FPM vers FrankenPHP, couvrant Caddyfile, php.ini, les threads et Docker.
---

# Migration depuis Nginx/PHP-FPM

FrankenPHP remplace à la fois votre serveur web (Nginx, Apache) et PHP-FPM par un unique binaire. Ce guide couvre une migration basique pour une application PHP typique.

## Différences clés

| Configuration PHP-FPM             | Équivalent FrankenPHP                                    |
| --------------------------------- | -------------------------------------------------------- |
| Nginx/Apache + PHP-FPM            | Binaire unique `frankenphp`                              |
| Paramètres de pool `php-fpm.conf` | [Option globale `frankenphp`](config.md#configuration-du-caddyfile) |
| Bloc `server {}` Nginx            | Bloc de site `Caddyfile`                                 |
| `php_value` / `php_admin_value`   | [Directive `php_ini` du Caddyfile](config.md#configuration-php)    |
| `pm = static` / `pm.max_children` | `num_threads`                                            |
| `pm = dynamic`                    | [`max_threads auto`](performance.md#max_threads)         |
| Filtrage des requêtes du serveur web      | Routes et matchers Caddy                                |

## Filtrage des requêtes HTTP

Lors de la migration depuis un paquet Nginx ou Apache standard, vérifiez tout filtrage de requêtes effectué par le serveur web avant que PHP-FPM ne reçoive la requête. FrankenPHP est construit sur Caddy, de sorte que les méthodes et en-têtes HTTP valides sont transmis via le routage normal de Caddy, à moins que votre `Caddyfile` ou un autre proxy ne les rejette en premier.

Par exemple, certains serveurs web rejettent les requêtes `TRACE` par défaut. Si votre application doit conserver ce comportement, ajoutez un matcher explicite avant `php_server` :

```caddyfile
example.com {
    @trace method TRACE
    respond @trace 405

    root /var/www/app/public
    php_server
}
```

Appliquez la même approche à toute politique de méthode ou d'en-tête plus stricte que votre précédent serveur web aurait imposée. Il en va de même pour les noms d'en-têtes de requête : certains serveurs web ou proxies rejettent ou suppriment des noms de champs HTTP valides mais peu communs avant que PHP-FPM ne les voie, tandis que FrankenPHP peut les transmettre à l'application PHP.

## Étape 1 : remplacer la configuration de votre serveur web

Une configuration Nginx + PHP-FPM typique :

```nginx
# /etc/nginx/sites-available/example.com
server {
    listen 80;
    server_name example.com;
    root /var/www/app/public;
    index index.php;

    location / {
        try_files $uri $uri/ /index.php$is_args$args;
    }

    location ~ \.php$ {
        fastcgi_pass unix:/run/php/php-fpm.sock;
        fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
        include fastcgi_params;
    }
}
```

Devient un seul `Caddyfile` :

```caddyfile
example.com {
    root /var/www/app/public
    php_server
}
```

C'est tout. La directive `php_server` gère le routage PHP, un comportement similaire à `try_files`, et la diffusion de fichiers statiques.

## Étape 2 : migrer la configuration PHP

Votre `php.ini` existant fonctionne tel quel. Voir [Configuration](config.md) pour savoir où le placer selon votre méthode d'installation.

Un paramètre change de signification : `max_execution_time` compte le temps réel (wall time) dans FrankenPHP, tandis que les builds non-ZTS de PHP-FPM sous Linux comptent le temps CPU. Attendre une requête de base de données, un appel HTTP, `sleep()` ou le verrou d'un fichier de session compte désormais dans la limite, de sorte qu'une valeur ajustée pour PHP-FPM peut être trop faible.

Vous pouvez également définir des directives directement dans le `Caddyfile` :

```caddyfile
{
    frankenphp {
        php_ini memory_limit 256M
        php_ini max_execution_time 30
    }
}

example.com {
    root /var/www/app/public
    php_server
}
```

## Étape 3 : ajuster la taille du pool

Dans PHP-FPM, vous ajustez `pm.max_children` pour contrôler le nombre de processus worker. Dans FrankenPHP, l'équivalent est `num_threads` :

```caddyfile
{
    frankenphp {
        num_threads 16
    }
}
```

Par défaut, FrankenPHP démarre 2 threads par CPU. Pour une mise à l'échelle dynamique similaire à `pm = dynamic` de PHP-FPM :

```caddyfile
{
    frankenphp {
        num_threads 4
        max_threads auto
    }
}
```

## Étape 4 : migration Docker

Une configuration Docker PHP-FPM typique utilisant deux conteneurs (Nginx + PHP-FPM) peut être remplacée par un seul conteneur :

**Avant :**

```yaml
# compose.yaml
services:
  nginx:
    image: nginx:1
    volumes:
      # monter la config Nginx dans le conteneur
      - ./config:/etc/nginx/conf.d
      - .:/var/www/app
    ports:
      - "80:80"
      - "443:443"

  php:
    image: php:8.5-fpm
    volumes:
      - .:/var/www/app
```

**Après :**

```yaml
# compose.yaml
services:
  php:
    image: dunglas/frankenphp:1-php8.5
    volumes:
      - .:/var/www/app
      # monter le Caddyfile dans le conteneur
      - ./config:/etc/frankenphp
      - caddy_data:/data
      - caddy_config:/config
    ports:
      - "80:80"
      - "443:443"
      - "443:443/udp"

volumes:
  caddy_data:
  caddy_config:
```

Si vous avez besoin d'extensions PHP supplémentaires, consultez [Construire une image Docker personnalisée](docker.md#comment-installer-plus-d'extensions-php).

Pour les configurations Docker spécifiques à un framework, consultez [Symfony Docker](https://github.com/dunglas/symfony-docker) et [exécuter Laravel avec l'image Docker FrankenPHP](laravel.md#exécuter-laravel-avec-l'image-docker-frankenphp).

## Étape 5 : considérer le mode worker (optionnel)

En [mode classique](classic.md), FrankenPHP fonctionne comme PHP-FPM : chaque requête démarre l'application à partir de zéro. C'est un point de départ sûr pour la migration.

Pour de meilleures performances, vous pouvez passer en [mode worker](worker.md), qui démarre votre application une seule fois et la maintient en mémoire :

```caddyfile
example.com {
    root /var/www/app/public
    php_server {
        root /var/www/app/public
        worker index.php 4
    }
}
```

> [!ATTENTION]
>
> Le mode worker maintient votre application en mémoire entre les requêtes. Assurez-vous que votre code ne dépend pas de la réinitialisation de l'état global entre les requêtes. Les frameworks comme [Symfony](worker.md#mode-worker-pour-symfony), [Laravel](laravel.md#laravel-octane) et [API Platform](https://api-platform.com) prennent en charge nativement ce mode.

## Ce que vous pouvez supprimer

Après la migration, vous n'avez plus besoin de :

- Nginx ou Apache
- PHP-FPM (service/processus `php-fpm`)
- Configuration FastCGI
- Certificats TLS auto-gérés (Caddy les gère automatiquement)
