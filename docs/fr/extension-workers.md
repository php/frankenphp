# Workers d'extension

Les Workers d'extension permettent à votre [extension FrankenPHP](https://frankenphp.dev/docs/extensions/) de gérer un pool dédié de threads PHP pour exécuter des tâches en arrière-plan, gérer des événements asynchrones ou implémenter des protocoles personnalisés. Utile pour les systèmes de files d'attente, les écouteurs d'événements, les planificateurs, etc.

## Enregistrement du Worker

### Enregistrement statique

Si vous n'avez pas besoin de rendre le worker configurable par l'utilisateur (chemin de script fixe, nombre de threads fixe), vous pouvez simplement enregistrer le worker dans la fonction `init()`.

```go
package myextension

import (
	"github.com/dunglas/frankenphp"
	"github.com/dunglas/frankenphp/caddy"
)

// Handle global pour communiquer avec le pool de workers
var worker frankenphp.Workers

func init() {
	// Enregistre le worker lorsque le module est chargé.
	worker = caddy.RegisterWorkers(
		"my-internal-worker", // Nom unique
		"worker.php",         // Chemin du script (relatif à l'exécution ou absolu)
		2,                    // Nombre de threads fixe
		// Hooks de cycle de vie optionnels
		frankenphp.WithWorkerOnServerStartup(func() {
			// Logique de configuration globale...
		}),
	)
}
```

### Dans un module Caddy (configurable par l'utilisateur)

Si vous prévoyez de partager votre extension (comme une file d'attente générique ou un écouteur d'événements), vous devriez l'envelopper dans un module Caddy. Cela permet aux utilisateurs de configurer le chemin du script et le nombre de threads via leur `Caddyfile`. Cela nécessite d'implémenter l'interface `caddy.Provisioner` et de parser le Caddyfile ([voir un exemple](https://github.com/dunglas/frankenphp-queue/blob/989120d394d66dd6c8e2101cac73dd622fade334/caddy.go)).

### Dans une application Go pure (intégration)

Si vous [intégrez FrankenPHP dans une application Go standard sans Caddy](https://pkg.go.dev/github.com/dunglas/frankenphp#example-ServeHTTP), vous pouvez enregistrer des workers d'extension en utilisant `frankenphp.WithExtensionWorkers` lors de l'initialisation des options.

## Interaction avec les Workers

Une fois le pool de workers actif, vous pouvez lui envoyer des tâches. Cela peut être fait à l'intérieur de [fonctions natives exportées vers PHP](https://frankenphp.dev/docs/extensions/#writing-the-extension), ou à partir de toute logique Go telle qu'un planificateur cron, un écouteur d'événements (MQTT, Kafka), ou toute autre goroutine.

### Mode sans tête : `SendMessage`

Utilisez `SendMessage` pour passer des données brutes directement à votre script worker. C'est idéal pour les files d'attente ou les commandes simples.

#### Exemple : Une extension de file d'attente asynchrone

```go
// #include <Zend/zend_types.h>
import "C"
import (
	"context"
	"unsafe"
	"github.com/dunglas/frankenphp"
)

//export_php:function my_queue_push(mixed $data): bool
func my_queue_push(data *C.zval) bool {
	// 1. S'assurer que le worker est prêt
	if worker == nil {
		return false
	}

	// 2. Envoyer au worker en arrière-plan
	_, err := worker.SendMessage(
		context.Background(), // Contexte Go standard
		unsafe.Pointer(data), // Données à passer au worker
		nil, // http.ResponseWriter optionnel
	)

	return err == nil
}
```

### Émulation HTTP : `SendRequest`

Utilisez `SendRequest` si votre extension doit invoquer un script PHP qui s'attend à un environnement web standard (remplir `$_SERVER`, `$_GET`, etc.).

```go
// #include <Zend/zend_types.h>
import "C"
import (
	"net/http"
	"net/http/httptest"
	"unsafe"
	"github.com/dunglas/frankenphp"
)

//export_php:function my_worker_http_request(string $path): string
func my_worker_http_request(path *C.zend_string) unsafe.Pointer {
	// 1. Préparer la requête et l'enregistreur
	url := frankenphp.GoString(unsafe.Pointer(path))
	req, _ := http.NewRequest("GET", url, http.NoBody)
	rr := httptest.NewRecorder()

	// 2. Envoyer au worker
	if err := worker.SendRequest(rr, req); err != nil {
		return nil
	}

	// 3. Retourner la réponse capturée
	return frankenphp.PHPString(rr.Body.String(), false)
}
```

## Script Worker

Le script worker PHP s'exécute dans une boucle et peut gérer à la fois les messages bruts et les requêtes HTTP.

```php
<?php
// Gérer à la fois les messages bruts et les requêtes HTTP dans la même boucle
$handler = function ($payload = null) {
    // Cas 1 : Mode Message
    if ($payload !== null) {
        return "Received payload: " . $payload;
    }

    // Cas 2 : Mode HTTP (les superglobales PHP standards sont peuplées)
    echo "Hello from page: " . $_SERVER['REQUEST_URI'];
};

while (frankenphp_handle_request($handler)) {
    gc_collect_cycles();
}
```

## Hooks de Cycle de Vie

FrankenPHP fournit des hooks pour exécuter du code Go à des points spécifiques du cycle de vie.

| Type de Hook | Nom de l'Option                  | Signature            | Contexte et Cas d'Utilisation                                          |
| :--------- | :--------------------------- | :------------------- | :--------------------------------------------------------------------- |
| **Serveur** | `WithWorkerOnServerStartup`  | `func()`             | Configuration globale. Exécuté **Une fois**. Exemple : Connexion à NATS/Redis. |
| **Serveur** | `WithWorkerOnServerShutdown` | `func()`             | Nettoyage global. Exécuté **Une fois**. Exemple : Fermeture des connexions partagées. |
| **Thread** | `WithWorkerOnReady`          | `func(threadID int)` | Configuration par thread. Appelé lorsqu'un thread démarre. Reçoit l'ID du Thread. |
| **Thread** | `WithWorkerOnShutdown`       | `func(threadID int)` | Nettoyage par thread. Reçoit l'ID du Thread.                           |

### Exemple

```go
package myextension

import (
    "fmt"
    "github.com/dunglas/frankenphp"
    frankenphpCaddy "github.com/dunglas/frankenphp/caddy"
)

func init() {
    workerHandle = frankenphpCaddy.RegisterWorkers(
        "my-worker", "worker.php", 2,

        // Démarrage du Serveur (Global)
        frankenphp.WithWorkerOnServerStartup(func() {
            fmt.Println("Extension : Démarrage du serveur...")
        }),

        // Thread Prêt (Par Thread)
        // Note : La fonction accepte un entier représentant l'ID du Thread
        frankenphp.WithWorkerOnReady(func(id int) {
            fmt.Printf("Extension : Le thread worker #%d est prêt.\n", id)
        }),
    )
}
```
