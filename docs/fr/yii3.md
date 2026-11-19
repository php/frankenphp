---
title: Exécuter Yii3 avec FrankenPHP (Docker, mode worker)
description: Comment exécuter une application Yii3 avec FrankenPHP en utilisant l'image Docker, une installation locale ou le mode worker avec le paquet yii-runner-frankenphp.
---

# Yii3

## Exécuter Yii3 avec l'image Docker de FrankenPHP

Servir une application web [Yii](https://www.yiiframework.com/) avec FrankenPHP est aussi simple que de monter le projet dans le répertoire `/app` de l'image Docker officielle.

Exécutez cette commande depuis le répertoire principal de votre application Yii :

```console
docker run -p 80:80 -p 443:443 -p 443:443/udp -v $PWD:/app dunglas/frankenphp
```

Et profitez-en !

## Installer Yii3 avec FrankenPHP en local

Alternativement, vous pouvez exécuter vos projets Yii avec FrankenPHP depuis votre machine locale :

1. [Téléchargez l'exécutable correspondant à votre système](../#executable-autonome)
2. Ajoutez la configuration suivante dans un fichier nommé `Caddyfile` à la racine de votre projet Yii :

   ```caddyfile
   {
   	frankenphp
   }

   # Le nom de domaine de votre serveur
   localhost {
   	# Activer la compression (facultatif)
   	encode zstd br gzip
   	# Exécute les fichiers PHP depuis le répertoire public/ et sert les ressources
   	php_server {
   		root public/
   		try_files {path} index.php
   	}
   }
   ```

3. Démarrez FrankenPHP depuis le répertoire racine de votre projet Yii : `frankenphp run`

## Mode worker Yii3

Pour exécuter votre application Yii en [mode worker](worker.md), installez le [runner Yii FrankenPHP](https://github.com/yiisoft/yii-runner-frankenphp) :

```console
composer require yiisoft/yii-runner-frankenphp
```

Ensuite, créez un fichier nommé `worker.php` à la racine de votre application :

```php
<?php

declare(strict_types=1);

use Yiisoft\Yii\Runner\FrankenPHP\FrankenPHPApplicationRunner;

require_once __DIR__ . '/vendor/autoload.php';

(new FrankenPHPApplicationRunner(rootPath: __DIR__))->run();
```

Mettez à jour votre `Caddyfile` pour démarrer l'application en mode worker :

```caddyfile
{
	frankenphp
}

localhost {
	encode zstd br gzip
	php_server {
		root public/
		worker ./worker.php {
			# Envoyer toutes les requêtes au worker
			match *
			# Recharger les workers lorsque les fichiers PHP changent (uniquement en développement)
			watch ./**/*.php
		}
	}
}
```

Si votre application est basée sur le [modèle d'application officiel de Yii](https://github.com/yiisoft/app), consultez le [README du paquet](https://github.com/yiisoft/yii-runner-frankenphp) pour un exemple complet de `worker.php` avec la configuration de débogage, d'environnement et de gestionnaire d'erreurs.

Pour limiter le nombre de requêtes qu'un worker traite avant d'être redémarré (utile pour atténuer les fuites de mémoire), définissez la variable d'environnement `MAX_REQUESTS`. Par défaut, les workers traitent les requêtes indéfiniment.

Lorsque vous utilisez le mode worker, assurez-vous que les services avec état sont réinitialisés après chaque requête. Consultez la [documentation du mode worker](worker.md) et la [documentation de `StateResetter` de Yii DI](https://github.com/yiisoft/di#resetting-services-state) pour plus de détails.
