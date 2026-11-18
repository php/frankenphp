---
title: Mode worker de FrankenPHP : gardez votre application PHP en mémoire
description: Exécutez FrankenPHP en mode worker pour maintenir votre application PHP bootstrappée entre les requêtes, réduire la surcharge de démarrage et servir les réponses en quelques millisecondes.
---

# Utilisation des workers FrankenPHP

Démarrez votre application une fois et gardez-la en mémoire.
FrankenPHP traitera les requêtes entrantes en quelques millisecondes.

## Démarrage des scripts workers FrankenPHP

### Exécuter un worker FrankenPHP avec Docker

Définissez la valeur de la variable d'environnement `FRANKENPHP_CONFIG` à `worker /path/to/your/worker/script.php` :

```bash
docker run \
    -e FRANKENPHP_CONFIG="worker /app/path/to/your/worker/script.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### Exécuter un worker FrankenPHP avec le binaire autonome

Utilisez l'option `--worker` de la commande `php-server` pour servir le contenu du répertoire courant en utilisant un worker :

```bash
frankenphp php-server --worker /path/to/your/worker/script.php
```

Si votre application PHP est [intégrée dans le binaire](embed.md), vous pouvez ajouter un `Caddyfile` personnalisé dans le répertoire racine de l'application.
Il sera utilisé automatiquement.

Il est également possible de [redémarrer le worker en cas de changement de fichier](config.md#watching-for-file-changes) avec l'option `--watch`.
La commande suivante déclenchera un redémarrage si un fichier se terminant par `.php` dans le répertoire `/path/to/your/app/` ou ses sous-répertoires est modifié :

```bash
frankenphp php-server --worker /path/to/your/worker/script.php --watch="/path/to/your/app/**/*.php"
```

Cette fonctionnalité est souvent utilisée en combinaison avec le [rechargement à chaud](hot-reload.md).

## Mode worker pour Symfony

Voir [la documentation du mode worker de Symfony avec FrankenPHP](symfony.md#symfony-worker-mode-with-frankenphp).

## Mode worker pour Laravel Octane

Voir [la documentation FrankenPHP Laravel Octane](laravel.md#laravel-octane).

## Écrire un script worker FrankenPHP personnalisé

L'exemple suivant montre comment créer votre propre script worker sans dépendre d'une bibliothèque tierce :

```php
<?php
// public/index.php

// Démarrer votre application
require __DIR__.'/vendor/autoload.php';

$myApp = new \App\Kernel();
$myApp->boot();

// Handler en dehors de la boucle pour de meilleures performances (moins de travail)
$handler = static function () use ($myApp) {
    try {
        // Appelé lorsqu'une requête est reçue,
        // les superglobales, php://input, etc., sont réinitialisés
        echo $myApp->handle($_GET, $_POST, $_COOKIE, $_FILES, $_SERVER);
    } catch (\Throwable $exception) {
        // `set_exception_handler` est appelé uniquement lorsque le script worker se termine,
        // ce qui peut ne pas être ce que vous attendez, alors interceptez et gérez les exceptions ici
        (new \MyCustomExceptionHandler)->handleException($exception);
    }
};

$maxRequests = (int)($_SERVER['MAX_REQUESTS'] ?? 0);
for ($nbRequests = 0; !$maxRequests || $nbRequests < $maxRequests; ++$nbRequests) {
    $keepRunning = \frankenphp_handle_request($handler);

    // Faire quelque chose après l'envoi de la réponse HTTP
    $myApp->terminate();

    // Appeler le ramasse-miettes pour réduire les chances qu'il soit déclenché au milieu de la génération d'une page
    gc_collect_cycles();

    if (!$keepRunning) break;
}

// Nettoyage
$myApp->shutdown();
```

Ensuite, démarrez votre application et utilisez la variable d'environnement `FRANKENPHP_CONFIG` pour configurer votre worker :

```bash
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

Par défaut, 2 workers par CPU sont démarrés.
Vous pouvez également configurer le nombre de workers à démarrer :

```bash
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php 42" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### Utiliser PSR-15

Si votre application utilise [PSR-15](https://www.php-fig.org/psr/psr-15/) au lieu des superglobales, convertissez la requête/réponse aux limites du handler avec [`nyholm/psr7`](https://github.com/Nyholm/psr7) et [`nyholm/psr7-server`](https://github.com/Nyholm/psr7-server) :

```console
composer require nyholm/psr7 nyholm/psr7-server psr/http-server-handler
```

```php
<?php
// public/index.php

require __DIR__.'/vendor/autoload.php';

use Nyholm\Psr7\Factory\Psr17Factory;
use Nyholm\Psr7Server\ServerRequestCreator;

$myApp = new \App\Kernel(); // implémente Psr\Http\Server\RequestHandlerInterface
$myApp->boot();

$psr17Factory = new Psr17Factory();
$creator = new ServerRequestCreator(
    $psr17Factory, // ServerRequestFactory
    $psr17Factory, // UriFactory
    $psr17Factory, // UploadedFileFactory
    $psr17Factory, // StreamFactory
);

$handler = static function () use ($myApp, $creator) {
    $response = $myApp->handle($creator->fromGlobals());

    http_response_code($response->getStatusCode());
    foreach ($response->getHeaders() as $name => $values) {
        foreach ($values as $value) {
            header("$name: $value", false);
        }
    }
    echo $response->getBody();
};

$maxRequests = (int)($_SERVER['MAX_REQUESTS'] ?? 0);
for ($nbRequests = 0; !$maxRequests || $nbRequests < $maxRequests; ++$nbRequests) {
    $keepRunning = \frankenphp_handle_request($handler);
    gc_collect_cycles();
    if (!$keepRunning) break;
}
```

### Redémarrer le worker après un certain nombre de requêtes

Comme PHP n'a pas été initialement conçu pour des processus de longue durée, de nombreuses bibliothèques et codes anciens présentent encore des fuites de mémoire.
Une solution pour utiliser ce type de code en mode worker est de redémarrer le script worker après avoir traité un certain nombre de requêtes :

Le code du worker précédent permet de configurer un nombre maximal de requêtes à traiter en définissant une variable d'environnement nommée `MAX_REQUESTS`.

### Redémarrer les workers manuellement

Bien qu'il soit possible de redémarrer les workers [en cas de changement de fichier](config.md#watching-for-file-changes),
il est également possible de redémarrer tous les workers de manière élégante via l'[API Admin de Caddy](https://caddyserver.com/docs/api).
Si l'administration est activée dans votre [Caddyfile](config.md#caddyfile-config), vous pouvez envoyer un ping
à l'endpoint de redémarrage avec une simple requête POST comme celle-ci :

```bash
curl -X POST http://localhost:2019/frankenphp/workers/restart
```

### Échecs des workers

Si un script worker crash avec un code de sortie non nul, FrankenPHP le redémarrera avec une stratégie de backoff exponentielle.
Si le script worker reste en place plus longtemps que le dernier backoff × 2,
il ne pénalisera pas le script worker et le redémarrera à nouveau.
Toutefois, si le script worker continue d'échouer avec un code de sortie non nul dans un court laps de temps
(par exemple, une faute de frappe dans un script), FrankenPHP plantera avec l'erreur : `too many consecutive failures`.

Le nombre d'échecs consécutifs peut être configuré dans votre [Caddyfile](config.md#caddyfile-config) avec l'option `max_consecutive_failures` :

```caddyfile
frankenphp {
    worker {
        # ...
        max_consecutive_failures 10
    }
}
```

### Nombre de threads

Lorsque le mode worker est activé, un thread supplémentaire est nécessaire pour gérer les requêtes entrantes,
donc la valeur par défaut de `num_threads` est `1 + 2x(nombre de CPUs disponibles)`.

Ainsi, dans ce mode, si `num_threads` et `num` sont explicitement définis, `num_threads` doit être strictement supérieur à `num`.

## Comportement des superglobales

[Les superglobales PHP](https://www.php.net/manual/language.variables.superglobals.php) (`$_SERVER`, `$_ENV`, `$_GET`...)
se comportent comme suit :

- avant le premier appel à `frankenphp_handle_request()`, les superglobales contiennent des valeurs liées au script worker lui-même
- pendant et après l'appel à `frankenphp_handle_request()`, les superglobales contiennent des valeurs générées à partir de la requête HTTP traitée, chaque appel à `frankenphp_handle_request()` change les valeurs des superglobales

Pour accéder aux superglobales du script worker à l'intérieur de la fonction de rappel, vous devez les copier et importer la copie dans la portée de la fonction de rappel :

```php
<?php
// Copier la superglobale $_SERVER du worker avant le premier appel à frankenphp_handle_request()
$workerServer = $_SERVER;

$handler = static function () use ($workerServer) {
    var_dump($_SERVER); // $_SERVER lié à la requête
    var_dump($workerServer); // $_SERVER du script worker
};

// ...
```

La plupart des superglobales (`$_GET`, `$_POST`, `$_COOKIE`, `$_FILES`, `$_SERVER`, `$_REQUEST`) sont automatiquement réinitialisées entre les requêtes.
Cependant, **`$_ENV` n'est actuellement pas réinitialisée entre les requêtes**.
Cela signifie que toute modification apportée à `$_ENV` pendant une requête persistera et sera visible pour les requêtes ultérieures traitées par le même thread worker.
Évitez de stocker des données spécifiques à la requête ou sensibles dans `$_ENV`.

## Persistance d'état

Parce que le mode worker maintient le processus PHP en vie entre les requêtes, l'état suivant persiste à travers les requêtes :

- **Variables statiques** : Les variables déclarées avec `static` à l'intérieur des fonctions ou des méthodes conservent leurs valeurs entre les requêtes.
- **Propriétés statiques de classe** : Les propriétés statiques des classes persistent entre les requêtes.
- **Variables globales** : Les variables dans la portée globale du script worker persistent entre les requêtes.
- **Caches en mémoire** : Toute donnée stockée en mémoire (tableaux, objets) en dehors du gestionnaire de requêtes persiste.
- **Paramètres du moteur d'exécution** : Les modifications effectuées avec `ini_set()`, `stream_context_set_default()`, `date_default_timezone_set()`, `chdir()`, `stream_wrapper_register()`, `set_error_handler()` ou `set_exception_handler()` restent en vigueur pour les requêtes ultérieures. Ne stockez jamais de valeurs spécifiques à la requête ou à l'utilisateur, telles que des identifiants dans le contexte de flux par défaut : passez plutôt un contexte explicite à chaque appel.
- **Paramètres à l'échelle du processus** : `umask()` et `setlocale()` modifient l'ensemble du processus FrankenPHP, pas seulement le thread actuel. Le mode worker ne les restaure pas entre les requêtes, et dans tous les modes, ils affectent également les requêtes exécutées simultanément. Définissez-les une fois au démarrage, ou appelez `chmod()` sur les fichiers créés au lieu de modifier l'umask.

C'est un comportement intentionnel et c'est ce qui rend le mode worker rapide. Cependant, cela demande de l'attention pour éviter les effets secondaires involontaires :

```php
<?php
function getCounter(): int {
    static $count = 0;
    return ++$count; // S'incrémente à travers les requêtes !
}

$handler = static function () {
    echo getCounter(); // 1, 2, 3, ... pour chaque requête sur ce thread
};

while (\frankenphp_handle_request($handler)) {
    // ...
}
```

Lorsque vous écrivez des scripts worker, assurez-vous de réinitialiser tout état spécifique à la requête entre les requêtes.
Les frameworks comme [Symfony](symfony.md) et [Laravel Octane](laravel.md) se chargent de réinitialiser la plupart des états pour vous, mais vous pourriez toujours avoir besoin de réinitialiser vos propres services. Avec Symfony, les services qui maintiennent un état spécifique à la requête doivent implémenter [`Symfony\Contracts\Service\ResetInterface`](https://github.com/symfony/contracts/blob/main/Service/ResetInterface.php) afin qu'ils soient réinitialisés par le noyau entre les requêtes.
