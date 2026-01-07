# Performance

Par défaut, FrankenPHP essaie d'offrir un bon compromis entre performance et facilité d'utilisation.
Cependant, il est possible d'améliorer considérablement les performances en utilisant une configuration appropriée.

## Nombre de threads et de workers

Par défaut, FrankenPHP démarre deux fois plus de threads et de workers (en mode worker) que le nombre de CPU disponibles.

Les valeurs appropriées dépendent fortement de la manière dont votre application est écrite, de ce qu'elle fait et de votre matériel.
Nous recommandons vivement de modifier ces valeurs. Pour une stabilité optimale du système, il est recommandé d'avoir `num_threads` x `memory_limit` < `available_memory`.

Pour trouver les bonnes valeurs, il est souhaitable d'effectuer des tests de charge simulant le trafic réel.
[k6](https://k6.io) et [Gatling](https://gatling.io) sont de bons outils pour cela.

Pour configurer le nombre de threads, utilisez l'option `num_threads` des directives `php_server` et `php`.
Pour changer le nombre de workers, utilisez l'option `num` de la section `worker` de la directive `frankenphp`.

### `max_threads`

Bien qu'il soit toujours préférable de savoir exactement à quoi ressemblera votre trafic, les applications réelles
ont tendance à être plus imprévisibles. La [configuration](config.md#caddyfile-config) `max_threads` permet à FrankenPHP de créer automatiquement des threads supplémentaires au moment de l'exécution, jusqu'à la limite spécifiée.
`max_threads` peut vous aider à déterminer le nombre de threads dont vous avez besoin pour gérer votre trafic et peut rendre le serveur plus résistant aux pics de latence.
Si elle est fixée à `auto`, la limite sera estimée en fonction de la valeur de `memory_limit` dans votre `php.ini`. Si ce n'est pas possible,
`auto` prendra par défaut 2x `num_threads`. Gardez à l'esprit que `auto` peut fortement sous-estimer le nombre de threads nécessaires.
`max_threads` est similaire à [pm.max_children](https://www.php.net/manual/en/install.fpm.configuration.php#pm.max-children) de PHP FPM. La principale différence est que FrankenPHP utilise des threads au lieu de
processus et les délègue automatiquement à différents scripts de worker et au 'mode classique' selon les besoins.

## Mode worker

Activer [le mode worker](worker.md) améliore considérablement les performances,
mais votre application doit être adaptée pour être compatible avec ce mode :
vous devez créer un script worker et vous assurer que l'application n'a pas de fuite de mémoire.

## Ne pas utiliser musl

La variante Alpine Linux des images Docker officielles et les binaires par défaut que nous fournissons utilisent [la bibliothèque musl](https://musl.libc.org).

PHP est connu pour être [plus lent](https://gitlab.alpinelinux.org/alpine/aports/-/issues/14381) lorsqu'il utilise cette bibliothèque C alternative au lieu de la bibliothèque GNU traditionnelle,
surtout lorsqu'il est compilé en mode ZTS (thread-safe), ce qui est nécessaire pour FrankenPHP. La différence peut être significative dans un environnement fortement threadé.

De plus, [certains bogues ne se produisent que lors de l'utilisation de musl](https://github.com/php/php-src/issues?q=sort%3Aupdated-desc+is%3Aissue+is%3Aopen+label%3ABug+musl).

Dans les environnements de production, nous recommandons d'utiliser FrankenPHP lié à glibc, compilé avec un niveau d'optimisation approprié.

Cela peut être réalisé en utilisant les images Docker Debian, en utilisant nos paquets [.deb](https://debs.henderkes.com) ou [.rpm](https://rpms.henderkes.com) fournis par nos mainteneurs, ou en [compilant FrankenPHP à partir des sources](compile.md).

## Configuration du runtime Go

FrankenPHP est écrit en Go.

En général, le runtime Go ne nécessite pas de configuration particulière, mais dans certaines circonstances,
une configuration spécifique améliore les performances.

Vous voudrez probablement définir la variable d'environnement `GODEBUG` à `cgocheck=0` (la valeur par défaut dans les images Docker de FrankenPHP).

Si vous exécutez FrankenPHP dans des conteneurs (Docker, Kubernetes, LXC...) et que vous limitez la mémoire disponible pour les conteneurs,
définissez la variable d'environnement `GOMEMLIMIT` à la quantité de mémoire disponible.

Pour plus de détails, [la page de documentation Go dédiée à ce sujet](https://pkg.go.dev/runtime#hdr-Environment_Variables) est une lecture indispensable pour tirer le meilleur parti du runtime.

## `file_server`

Par défaut, la directive `php_server` met automatiquement en place un serveur de fichiers
pour servir les fichiers statiques (assets) stockés dans le répertoire racine.

Cette fonctionnalité est pratique, mais a un coût.
Pour la désactiver, utilisez la configuration suivante :

```caddyfile
php_server {
    file_server off
}
```

## `try_files`

Outre les fichiers statiques et les fichiers PHP, `php_server` essaiera également de servir les fichiers d'index
et d'index de répertoire de votre application (`/path/` -> `/path/index.php`). Si vous n'avez pas besoin d'indices de répertoire,
vous pouvez les désactiver en définissant explicitement `try_files` comme ceci :

```caddyfile
php_server {
    try_files {path} index.php
    root /root/to/your/app # l'ajout explicite de la racine ici permet une meilleure mise en cache
}
```

Cela permet de réduire considérablement le nombre d'opérations inutiles sur les fichiers.

Une approche alternative avec 0 opération inutile sur le système de fichiers serait d'utiliser la directive `php`
et de séparer les fichiers de PHP par chemin. Cette approche fonctionne bien si votre application entière est servie par un seul fichier d'entrée.
Un exemple de [configuration](config.md#caddyfile-config) qui sert des fichiers statiques derrière un dossier `/assets` pourrait ressembler à ceci :

```caddyfile
route {
    @assets {
        path /assets/*
    }

    # tout ce qui se trouve derrière /assets est géré par le serveur de fichiers
    file_server @assets {
        root /root/to/your/app
    }

    # tout ce qui n'est pas dans /assets est géré par votre index ou votre fichier PHP worker
    rewrite index.php
    php {
        root /root/to/your/app # l'ajout explicite de la racine ici permet une meilleure mise en cache
    }
}
```

## Placeholders

Vous pouvez utiliser des [placeholders](https://caddyserver.com/docs/conventions#placeholders) dans les directives `root` et `env`.
Cependant, cela empêche la mise en cache de ces valeurs et a un coût important en termes de performances.

Si possible, évitez les placeholders dans ces directives.

## `resolve_root_symlink`

Par défaut, si le document root est un lien symbolique, il est automatiquement résolu par FrankenPHP (c'est nécessaire pour que PHP fonctionne correctement).
Si le document root n'est pas un lien symbolique, vous pouvez désactiver cette fonctionnalité.

```caddyfile
php_server {
    resolve_root_symlink false
}
```

Cela améliorera les performances si la directive `root` contient des [placeholders](https://caddyserver.com/docs/conventions#placeholders).
Le gain sera négligeable dans les autres cas.

## Journaux

La journalisation est évidemment très utile, mais, par définition, elle nécessite des opérations d'I/O et des allocations de mémoire,
ce qui réduit considérablement les performances.
Assurez-vous de [définir le niveau de journalisation](https://caddyserver.com/docs/caddyfile/options#log) correctement,
et de ne journaliser que ce qui est nécessaire.

## Performances de PHP

FrankenPHP utilise l'interpréteur PHP officiel.
Toutes les optimisations de performances habituelles liées à PHP s'appliquent à FrankenPHP.

En particulier :

- vérifiez que [OPcache](https://www.php.net/manual/en/book.opcache.php) est installé, activé et correctement configuré
- activez [les optimisations de l'autoloader de Composer](https://getcomposer.org/doc/articles/autoloader-optimization.md)
- assurez-vous que le cache `realpath` est suffisamment grand pour les besoins de votre application
- utilisez le [pré-chargement](https://www.php.net/manual/en/opcache.preloading.php)

Pour plus de détails, lisez [l'entrée de la documentation dédiée de Symfony](https://symfony.com/doc/current/performance.html)
(la plupart des conseils sont utiles même si vous n'utilisez pas Symfony).

## Division du pool de threads

Il est courant que les applications interagissent avec des services externes lents, comme une
API qui a tendance à être peu fiable sous une forte charge ou qui met constamment plus de 10 secondes à répondre.
Dans de tels cas, il peut être bénéfique de diviser le pool de threads pour avoir des pools "lents" dédiés.
Cela empêche les points d'accès lents de consommer toutes les ressources/threads du serveur et
limite la concurrence des requêtes allant vers le point d'accès lent, à l'instar d'un pool de connexions.

```caddyfile
{
    frankenphp {
        max_threads 100 # 100 threads maximum partagés par tous les workers
    }
}

example.com {
    php_server {
        root /app/public # la racine de votre application
        worker index.php {
            match /slow-endpoint/* # toutes les requêtes avec le chemin /slow-endpoint/* sont gérées par ce pool de threads
            num 10 # minimum 10 threads pour les requêtes correspondant à /slow-endpoint/*
        }
        worker index.php {
            match * # toutes les autres requêtes sont gérées séparément
            num 20 # minimum 20 threads pour les autres requêtes, même si les points d'accès lents commencent à bloquer
        }
    }
}
```

De manière générale, il est également conseillé de gérer les points d'accès très lents de manière asynchrone, en utilisant des mécanismes pertinents tels que les files de messages.
