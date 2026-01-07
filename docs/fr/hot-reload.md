# Rechargement à chaud

FrankenPHP inclut une fonctionnalité de **rechargement à chaud** intégrée, conçue pour améliorer considérablement l'expérience développeur.

![Mercure](hot-reload.png)

Cette fonctionnalité offre un flux de travail similaire au **Remplacement de Module à Chaud (HMR)** que l'on trouve dans les outils JavaScript modernes (comme Vite ou webpack).
Au lieu de rafraîchir manuellement le navigateur après chaque modification de fichier (code PHP, templates, fichiers JavaScript et CSS...),
FrankenPHP met à jour le contenu en temps réel.

Le rechargement à chaud fonctionne nativement avec WordPress, Laravel, Symfony, et toute autre application ou framework PHP.

Lorsqu'il est activé, FrankenPHP surveille votre répertoire de travail actuel pour les modifications du système de fichiers.
Lorsqu'un fichier est modifié, il envoie une mise à jour [Mercure](mercure.md) au navigateur.

Selon votre configuration, le navigateur :

- **Transformera le DOM** (en préservant la position de défilement et l'état des entrées) si [Idiomorph](https://github.com/bigskysoftware/idiomorph) est chargé.
- **Rechargera la page** (rechargement en direct standard) si Idiomorph n'est pas présent.

## Configuration

Pour activer le rechargement à chaud, activez Mercure, puis ajoutez la sous-directive `hot_reload` à la directive `php_server` dans votre `Caddyfile`.

> [!WARNING]
> Cette fonctionnalité est destinée aux **environnements de développement uniquement**.
> N'activez pas `hot_reload` en production, car la surveillance du système de fichiers entraîne une surcharge de performance et expose des points d'accès internes.

```caddyfile
localhost

mercure {
    anonymous
}

root public/
php_server {
    hot_reload
}
```

Par défaut, FrankenPHP surveillera tous les fichiers dans le répertoire de travail actuel correspondant à ce modèle glob : `./**/*.{css,env,gif,htm,html,jpg,jpeg,js,mjs,php,png,svg,twig,webp,xml,yaml,yml}`

Il est possible de définir explicitement les fichiers à surveiller en utilisant la syntaxe glob :

```caddyfile
localhost

mercure {
    anonymous
}

root public/
php_server {
    hot_reload src/**/*{.php,.js} config/**/*.yaml
}
```

Utilisez la forme longue pour spécifier le sujet Mercure à utiliser ainsi que les répertoires ou fichiers à surveiller en fournissant des chemins à l'option `hot_reload` :

```caddyfile
localhost

mercure {
    anonymous
}

root public/
php_server {
    hot_reload {
        topic hot-reload-topic
        watch src/**/*.php
        watch assets/**/*.{ts,json}
        watch templates/
        watch public/css/
    }
}
```

## Intégration côté client

Alors que le serveur détecte les changements, le navigateur doit s'abonner à ces événements pour mettre à jour la page.
FrankenPHP expose l'URL du Hub Mercure à utiliser pour s'abonner aux changements de fichiers via la variable d'environnement `$_SERVER['FRANKENPHP_HOT_RELOAD']`.

Une bibliothèque JavaScript pratique, [frankenphp-hot-reload](https://www.npmjs.com/package/frankenphp-hot-reload), est également disponible pour gérer la logique côté client.
Pour l'utiliser, ajoutez ce qui suit à votre mise en page principale :

```php
<!DOCTYPE html>
<title>Rechargement à chaud FrankenPHP</title>
<?php if (isset($_SERVER['FRANKENPHP_HOT_RELOAD'])): ?>
<meta name="frankenphp-hot-reload:url" content="<?=$_SERVER['FRANKENPHP_HOT_RELOAD']?>">
<script src="https://cdn.jsdelivr.net/npm/idiomorph"></script>
<script src="https://cdn.jsdelivr.net/npm/frankenphp-hot-reload/+esm" type="module"></script>
<?php endif ?>
```

La bibliothèque s'abonnera automatiquement au hub Mercure, récupérera l'URL actuelle en arrière-plan lorsqu'un changement de fichier est détecté et transformera le DOM.
Elle est disponible en tant que package [npm](https://www.npmjs.com/package/frankenphp-hot-reload) et sur [GitHub](https://github.com/dunglas/frankenphp-hot-reload).

Alternativement, vous pouvez implémenter votre propre logique côté client en vous abonnant directement au hub Mercure en utilisant la classe JavaScript native `EventSource`.

### Mode Worker

Si vous exécutez votre application en [Mode Worker](https://frankenphp.dev/docs/worker/), le script de votre application reste en mémoire.
Cela signifie que les modifications apportées à votre code PHP ne seront pas reflétées immédiatement, même si le navigateur se recharge.

Pour la meilleure expérience développeur, vous devez combiner `hot_reload` avec [la sous-directive `watch` dans la directive `worker`](config.md#watching-for-file-changes).

- `hot_reload` : rafraîchit le **navigateur** lorsque les fichiers changent
- `worker.watch` : redémarre le worker lorsque les fichiers changent

```caddy
localhost

mercure {
    anonymous
}

root public/
php_server {
    hot_reload
    worker {
        file /path/to/my_worker.php
        watch
    }
}
```

### Comment ça marche

1. **Surveillance** : FrankenPHP surveille le système de fichiers pour les modifications en utilisant la bibliothèque [`e-dant/watcher`](https://github.com/e-dant/watcher) en coulisses (nous avons contribué au binding Go).
2. **Redémarrage (Mode Worker)** : si `watch` est activé dans la configuration du worker, le worker PHP est redémarré pour charger le nouveau code.
3. **Poussée** : une charge utile JSON contenant la liste des fichiers modifiés est envoyée au [hub Mercure](https://mercure.rocks) intégré.
4. **Réception** : le navigateur, écoutant via la bibliothèque JavaScript, reçoit l'événement Mercure.
5. **Mise à jour** :

- Si **Idiomorph** est détecté, il récupère le contenu mis à jour et transforme le HTML actuel pour correspondre au nouvel état, appliquant les changements instantanément sans perdre l'état.
- Sinon, `window.location.reload()` est appelé pour rafraîchir la page.
