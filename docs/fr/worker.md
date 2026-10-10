# Utilisation des workers FrankenPHP

Démarrez votre application une fois et gardez-la en mémoire.
FrankenPHP traitera les requêtes entrantes en quelques millisecondes.

## Démarrer des scripts workers FrankenPHP

### Exécuter un worker FrankenPHP avec Docker

Définissez la valeur de la variable d'environnement `FRANKENPHP_CONFIG` à `worker /path/to/your/worker/script.php` :

```console
docker run \
    -e FRANKENPHP_CONFIG="worker /app/path/to/your/worker/script.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### Exécuter un worker FrankenPHP avec le binaire autonome

Utilisez l'option `--worker` de la commande `php-server` pour servir le contenu du répertoire courant en utilisant un worker :

```console
frankenphp php-server --worker /path/to/your/worker/script.php
```

Si votre application PHP est [intégrée dans le binaire](embed.md), vous pouvez ajouter un `Caddyfile` personnalisé dans le répertoire racine de l'application.
Il sera utilisé automatiquement.

Il est également possible de [redémarrer le worker en cas de changement de fichier](config.md#watching-for-file-changes) avec l'option `--watch`.
La commande suivante déclenchera un redémarrage si un fichier se terminant par `.php` dans le répertoire `/path/to/your/app/` ou ses sous-répertoires est modifié :

```console
frankenphp php-server --worker /path/to/your/worker/script.php --watch="/path/to/your/app/**/*.php"
```

Cette fonctionnalité se combine très bien avec le [rechargement à chaud](hot-reload.md).

## Le mode worker pour Symfony

Consultez [la documentation du mode worker de FrankenPHP pour Symfony](symfony.md#le-mode-worker-de-symfony-avec-frankenphp).

## Le mode worker pour Laravel Octane

Consultez [la documentation de FrankenPHP pour Laravel Octane](laravel.md#laravel-octane).

## Écrire un script worker FrankenPHP personnalisé

L'exemple suivant montre comment créer votre propre script worker sans dépendre d'une bibliothèque tierce :

```php
<?php
// public/index.php

// Démarrer votre application
require __DIR__.'/vendor/autoload.php';

$myApp = new \App\Kernel();
$myApp->boot();

// Déclarer le handler en dehors de la boucle pour de meilleures performances (moins de travail effectué)
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

    // Exécuter le ramasse-miettes pour réduire les chances qu'il soit déclenché au milieu de la génération d'une page
    gc_collect_cycles();

    if (!$keepRunning) break;
}

// Nettoyage
$myApp->shutdown();
```

Ensuite, démarrez votre application et utilisez la variable d'environnement `FRANKENPHP_CONFIG` pour configurer votre worker :

```console
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

Par défaut, 2 workers par CPU sont démarrés.
Vous pouvez également configurer le nombre de workers à démarrer :

```console
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php 42" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### Utiliser PSR-15

Si votre application utilise [PSR-15](https://www.php-fig.org/psr/psr-15/) plutôt que les superglobales, convertissez la requête et la réponse aux extrémités du handler avec [`nyholm/psr7`](https://github.com/Nyholm/psr7) et [`nyholm/psr7-server`](https://github.com/Nyholm/psr7-server) :

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

```console
curl -X POST http://localhost:2019/frankenphp/workers/restart
```

### Échecs des workers

Si un script de worker se plante avec un code de sortie non nul, FrankenPHP le redémarre avec une stratégie de backoff exponentielle.
Si le script worker reste en place plus longtemps que le dernier backoff × 2, FrankenPHP ne pénalisera pas le script et le redémarrera à nouveau.
Toutefois, si le script de worker continue d'échouer avec un code de sortie non nul dans un court laps de temps
(par exemple, une faute de frappe dans un script), FrankenPHP plantera avec l'erreur : `too many consecutive failures` (trop d'échecs consécutifs).

Le nombre d'échecs consécutifs peut être configuré dans votre [Caddyfile](config.md#caddyfile-config) avec l'option `max_consecutive_failures` :

```caddyfile
frankenphp {
    worker {
        # ...
        max_consecutive_failures 10
    }
}
```

## Comportement des superglobales

[Les superglobales PHP](https://www.php.net/manual/language.variables.superglobals.php) (`$_SERVER`, `$_ENV`, `$_GET`...)
se comportent comme suit :

- avant le premier appel à `frankenphp_handle_request()`, les superglobales contiennent des valeurs liées au script worker lui-même
- pendant et après l'appel à `frankenphp_handle_request()`, les superglobales contiennent des valeurs générées à partir de la requête HTTP traitée, chaque appel à `frankenphp_handle_request()` change les valeurs des superglobales

Pour accéder aux superglobales du script worker à l'intérieur de la fonction de rappel, vous devez les copier et importer la copie dans le scope de la fonction :

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
Cela signifie que toute modification apportée à `$_ENV` pendant une requête persistera et sera visible par les requêtes suivantes traitées par le même thread worker.
Évitez de stocker des données spécifiques à une requête ou sensibles dans `$_ENV`.

## Persistance de l'état

Comme le mode worker garde le processus PHP en vie entre les requêtes, l'état suivant persiste d'une requête à l'autre :

- **Variables statiques** : les variables déclarées avec `static` dans des fonctions ou des méthodes conservent leur valeur entre les requêtes.
- **Propriétés statiques de classe** : les propriétés statiques des classes persistent entre les requêtes.
- **Variables globales** : les variables de la portée globale du script worker persistent entre les requêtes.
- **Caches en mémoire** : toute donnée stockée en mémoire (tableaux, objets) en dehors du handler de requête persiste.
- **Paramètres du moteur d'exécution** : les modifications effectuées avec `ini_set()`, `stream_context_set_default()`, `date_default_timezone_set()`, `chdir()`, `stream_wrapper_register()`, `set_error_handler()` ou `set_exception_handler()` restent en vigueur pour les requêtes suivantes. N'y stockez jamais de valeurs spécifiques à une requête ou à un utilisateur, comme des identifiants dans le contexte de flux par défaut : passez plutôt un contexte explicite à chaque appel.

C'est voulu, et c'est ce qui rend le mode worker rapide. Cela demande toutefois de l'attention pour éviter des effets de bord indésirables :

```php
<?php
function getCounter(): int {
    static $count = 0;
    return ++$count; // S'incrémente d'une requête à l'autre !
}

$handler = static function () {
    echo getCounter(); // 1, 2, 3, ... pour chaque requête sur ce thread
};

while (\frankenphp_handle_request($handler)) {
    // ...
}
```

Lorsque vous écrivez des scripts workers, veillez à réinitialiser tout état spécifique à une requête entre les requêtes.
Les frameworks comme [Symfony](symfony.md) et [Laravel Octane](laravel.md) se chargent de réinitialiser la plupart de l'état pour vous, mais vous devrez peut-être encore réinitialiser vos propres services. Avec Symfony, les services qui conservent un état spécifique à la requête doivent implémenter [`Symfony\Contracts\Service\ResetInterface`](https://github.com/symfony/contracts/blob/main/Service/ResetInterface.php) afin d'être réinitialisés par le kernel entre les requêtes.
