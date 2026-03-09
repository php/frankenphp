# Hot Reload

FrankenPHP inclut une fonctionnalité de **hot reload** intégrée, conçue pour améliorer considérablement l'expérience développeur.

![Hot Reload](../hot-reload.png)

Cette fonctionnalité offre un workflow similaire au **Hot Module Replacement (HMR)** présent dans les outils JavaScript modernes (comme Vite ou webpack).
Au lieu de rafraîchir manuellement le navigateur après chaque modification de fichier (code PHP, templates, fichiers JavaScript et CSS...),
FrankenPHP met à jour le contenu de la page en temps réel.

Le Hot Reload fonctionne nativement avec WordPress, Laravel, Symfony et toute autre application ou framework PHP.

Lorsqu'il est activé, FrankenPHP surveille votre répertoire de travail actuel pour détecter les modifications du système de fichiers.
Quand un fichier est modifié, il envoie une mise à jour [Mercure](mercure.md) au navigateur.

Selon votre configuration, le navigateur va soit :

- **Transformer le DOM** (en préservant la position de défilement et l'état des champs de saisie) si [Idiomorph](https://github.com/bigskysoftware/idiomorph) est chargé.
- **Recharger la page** (rechargement standard) si Idiomorph n'est pas présent.

## Configuration

Pour activer le hot reload, activez Mercure, puis ajoutez la sous-directive `hot_reload` à la directive `php_server` dans votre `Caddyfile`.

> [!WARNING]

> Cette fonctionnalité est destinée **uniquement aux environnements de développement**.
> N'activez pas `hot_reload` en production, car cette fonctionnalité n'est pas sécurisée (expose des détails internes sensibles) et ralentit l'application.

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

Par défaut, FrankenPHP surveillera tous les fichiers du répertoire de travail actuel correspondant au motif glob suivant : `./**/*.{css,env,gif,htm,html,jpg,jpeg,js,mjs,php,png,svg,twig,webp,xml,yaml,yml}`

Il est possible de définir explicitement les fichiers à surveiller en utilisant un motif glob :

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

Utilisez la forme longue de `hot_reload` pour spécifier le *topic* Mercure à utiliser ainsi que les répertoires ou fichiers à surveiller :

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

Le serveur détecte les modifications et publie les modifications automatiquement. Le navigateur doit s'abonner à ces événements pour mettre à jour la page.
FrankenPHP expose l'URL du Hub Mercure à utiliser pour s'abonner aux modifications de fichiers via la variable d'environnement `$_SERVER['FRANKENPHP_HOT_RELOAD']`.

Une bibliothèque JavaScript pratique, [frankenphp-hot-reload](https://www.npmjs.com/package/frankenphp-hot-reload), est également disponible pour gérer la logique côté client.
Pour l'utiliser, ajoutez ce qui suit à votre gabarit (*layout*) principal :

```php
<!DOCTYPE html>
<title>FrankenPHP Hot Reload</title>
<?php if (isset($_SERVER['FRANKENPHP_HOT_RELOAD'])): ?>
<meta name="frankenphp-hot-reload:url" content="<?=$_SERVER['FRANKENPHP_HOT_RELOAD']?>">
<script src="https://cdn.jsdelivr.net/npm/idiomorph"></script>
<script src="https://cdn.jsdelivr.net/npm/frankenphp-hot-reload/+esm" type="module"></script>
<?php endif ?>
```

La bibliothèque s'abonnera automatiquement au hub Mercure, récupérera l'URL actuelle en arrière-plan lorsqu'une modification de fichier est détectée et transformera le DOM.
Elle est disponible en tant que package [npm](https://www.npmjs.com/package/frankenphp-hot-reload) et sur [GitHub](https://github.com/dunglas/frankenphp-hot-reload).

Alternativement, vous pouvez implémenter votre propre logique côté client en vous abonnant directement au hub Mercure en utilisant la classe JavaScript native `EventSource`.

### Conserver les nœuds DOM existants

Dans de rares cas, comme lors de l'utilisation d'outils de développement tels que [la *web debug toolbar* de Symfony](https://github.com/symfony/symfony/pull/62970),
vous pouvez souhaiter conserver des nœuds DOM spécifiques.
Pour ce faire, ajoutez l'attribut `data-frankenphp-hot-reload-preserve` à l'élément HTML concerné :

```html
<div data-frankenphp-hot-reload-preserve><!-- Ma barre de développement --></div>
```

## Mode Worker

Si vous exécutez votre application en [mode Worker](worker.md), le script de votre application reste en mémoire.
Cela signifie que les modifications de votre code PHP ne seront pas reflétées immédiatement, même si le navigateur recharge la page.

Pour une meilleure expérience de développement, vous devriez combiner `hot_reload` avec [la sous-directive `watch` dans la directive `worker`](config.md#surveillance-des-modifications-de-fichier).

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

## Comment ça fonctionne

1. **Surveillance** : FrankenPHP surveille le système de fichiers pour les modifications en utilisant [la bibliothèque `e-dant/watcher`](https://github.com/e-dant/watcher) en interne (nous avons contribué à son binding Go).
2. **Redémarrage (mode Worker)** : si `watch` est activé dans la configuration du worker, le worker PHP est redémarré pour charger le nouveau code.
3. **Envoi** : un payload JSON contenant la liste des fichiers modifiés est envoyé au [hub Mercure](https://mercure.rocks) intégré.
4. **Réception** : le navigateur, à l'écoute via la bibliothèque JavaScript, reçoit l'événement Mercure.
5. **Mise à jour** :

- Si **Idiomorph** est détecté, il récupère le contenu mis à jour et transforme le HTML actuel pour correspondre au nouvel état, appliquant les modifications instantanément sans perdre l'état.
- Sinon, `window.location.reload()` est appelé pour rafraîchir la page.
