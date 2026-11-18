---
title: "Modèle de sécurité de FrankenPHP : limites de confiance entre Go et PHP"
description: "Où se situe la limite de confiance de FrankenPHP : quels sont les inputs non fiables, ce dont le runtime Go/C est responsable, et ce qui incombe à l'application PHP ou aux projets en amont."
---

# Modèle de sécurité

Ce document décrit le modèle de confiance de FrankenPHP : quelles sont les entrées fiables, quelles ne le sont pas, et où se situe la limite entre elles.
Il est destiné à aider les audits de sécurité et les scanners automatisés à raisonner sur **FrankenPHP lui-même**, et non sur les applications PHP qu'il sert.

Pour les mécanismes internes référencés ici (threads, la limite CGO, le sandboxing d'environnement), voir [Fonctionnement interne](internals.md).
Pour la persistance d'état dans les processus de longue durée, voir [Mode Travailleur](worker.md).

## Limites de confiance

FrankenPHP s'exécute dans une pile avec quatre acteurs distincts :

| Acteur                   | Confiance             | Notes                                                                                                                                                |
| ------------------------ | --------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Client distant**       | Non fiable             | La requête HTTP (méthode, URI, en-têtes, cookies, corps, uploads) est la principale source d'entrée contaminée.                                         |
| **Opérateur**            | Fiable                | Fournit la configuration de déploiement : le `Caddyfile`, les variables d'environnement, `php.ini`, les extensions PHP et modules Caddy installés, et le code de l'application lui-même. |
| **Code d'application PHP** | Fiable *par provenance* | Déployé par l'opérateur, donc FrankenPHP n'exécute jamais de code fourni par un attaquant, mais ce code *consomme* des données de requête non fiables.                    |
| **FrankenPHP (Go + C)**  | Base de calcul fiable  | Intègre PHP, transporte les données entrantes et sortantes, et isole les requêtes et les threads. Ce document couvre ses propres défauts.                            |

## Provenance du code vs. contamination des données

C'est la distinction la plus importante, et celle qui résout la plupart des confusions "faut-il faire confiance à PHP ou non ?" :

- **La provenance du code est fiable.** FrankenPHP n'exécute que les fichiers PHP déployés par l'opérateur : le script résolu sous la racine du document, ou le script de travailleur configuré. Il n'évalue jamais de code transporté dans la requête elle-même (corps, chaîne de requête, en-têtes) : la limite SAPI transporte des *données*, jamais du *code* non fiable.
- **Les données de requête sont contaminées.** Tout ce que le code PHP lit depuis la requête (`$_GET`, `$_POST`, `$_COOKIE`, `$_FILES`, `$_SERVER`, `php://input`) est non fiable, exactement comme dans toute SAPI PHP. La désinfection de ces données est le travail de l'application.

Ainsi, "nous faisons confiance à ce qui vient de PHP" est vrai pour le *code*, tandis que "la SAPI transporte des entrées non fiables" est vrai pour les *données* : les deux ne sont pas en conflit.
Le rôle de FrankenPHP est de transporter fidèlement ces données contaminées et d'empêcher les données d'une requête de fuir vers une autre.

## Ce dont FrankenPHP est responsable

La base de calcul fiable a trois tâches. Les défauts de sécurité dans FrankenPHP se situent dans l'une d'entre elles :

1. **Transport fidèle** : mapper la requête dans les superglobales PHP et `php://input`, et transporter la sortie et les en-têtes de PHP vers le client, sans introduire d'injection (injection d'en-tête/CRLF, contrebande de requête, exécution du mauvais fichier).
2. **Isolation** : empêcher l'état à l'échelle de la requête de traverser entre les requêtes, entre les threads PHP et entre les itérations des travailleurs.
3. **Sécurité mémoire** : gérer la limite CGO (Go ↔ C/PHP) sans corrompre la mémoire.

## Dans le périmètre : la surface d'attaque propre à FrankenPHP

Voici les surfaces que FrankenPHP gère. Une vulnérabilité ici est une vulnérabilité de FrankenPHP :

- **Mappage de la requête aux superglobales** (`cgi.go`, `frankenphp_register_server_vars`) : construction de `$_SERVER`, `REMOTE_ADDR`, `SCRIPT_NAME`, `PATH_INFO` et des autres variables CGI à partir de la requête.
- **Résolution du chemin du script PHP** : le chemin de la requête est divisé sur `split_path` (`.php` par défaut) en `SCRIPT_NAME` / `PATH_INFO`, puis joint à la racine du document avec `sanitizedPathJoin` (`filepath.Join(root, filepath.Clean("/"+reqPath))`), ce qui empêche `SCRIPT_FILENAME` de s'échapper de la racine du document (traversée de chemin). La directive `php_server` définit en outre une réécriture `try_files` par défaut qui achemine les requêtes vers les fichiers existants ou le contrôleur frontal, atténuant ainsi le piège classique de PHP-FPM d'exécuter le mauvais fichier.
- **Isolation de l'état en mode travailleur** : FrankenPHP réinitialise `$_GET`, `$_POST`, `$_COOKIE`, `$_FILES`, `$_SERVER` et `$_REQUEST` entre les requêtes, efface explicitement `$_SESSION` (qui autrement fuirait entre les requêtes) et les erreurs retournées par `error_get_last()` et `libxml_get_errors()`, ainsi que les en-têtes retournés par `http_get_last_response_headers()`, l'état de `strtok()`, et les variables ajoutées par `output_add_rewrite_var()`, mais **`$_ENV` n'est pas réinitialisé**, et les écritures de `putenv()`, les variables `static`, les propriétés statiques de classe, les globales et les paramètres du moteur d'exécution (`ini_set()`, le contexte de flux par défaut, le fuseau horaire par défaut, le répertoire de travail, ...) persistent entre les requêtes sur le même thread. Les données spécifiques à la requête ou à l'utilisateur laissées dans cet état peuvent fuir vers une requête ultérieure (voir [Mode Travailleur](worker.md#state-persistence)).
- **Bac à sable d'environnement par thread** : `frankenphp_putenv()` / `frankenphp_getenv()` opèrent sur un `sandboxed_env` local au thread. Les écritures de `putenv()` restent dans ce bac à sable et n'atteignent jamais l'environnement OS global du processus, de sorte qu'un secret défini par une requête ne peut pas fuir vers les processus enfants (`proc_open`, `exec`, ...), vers le côté Go/Caddy, ou vers d'autres requêtes. Un processus enfant qui doit voir une telle variable doit la recevoir explicitement, via l'argument d'environnement de `proc_open()` ou `exec()` (voir [Fonctionnement interne](internals.md#per-thread-environment-sandboxing)).
- **Limite mémoire CGO** : l'épinglage des chaînes Go et les durées de vie de `C.CString()` / `free()` à travers la limite Go ↔ C.
- **API d'administration Caddy** : les points de terminaison `/frankenphp/workers/restart` et `/frankenphp/threads`, exposés via l'API d'administration de Caddy (qui écoute sur `localhost:2019` par défaut). L'exposition de ce point de terminaison au-delà de localhost est une décision de l'opérateur.
- **Gestion des proxys de confiance** : les en-têtes `X-Forwarded-*` entrants parviennent toujours à PHP en tant que valeurs `$_SERVER['HTTP_X_FORWARDED_*']` contaminées ; ils ne sont fiables pour dériver l'IP et le schéma du client réel que lorsque `trusted_proxies` est configuré ([`proxies_de_confiance`](production.md#running-behind-a-reverse-proxy)).
- **Corps de requête lents** : un client qui annonce un corps puis le goutte-à-goutte ou le bloque retient le thread de traitement pendant toute la durée. Avec un pool de threads limité, un nombre suffisant de telles connexions l'épuisent (DoS par slow-POST). FrankenPHP applique un délai d'inactivité de 60s sur les lectures de corps par défaut ([`delai_d_attente_du_corps_de_la_requete`](config.md#caddyfile-config)), réinitialisant la date limite avant chaque lecture afin qu'un téléchargement stable de toute taille réussisse tandis qu'un téléchargement bloqué est interrompu et le thread libéré.

## Hors périmètre

- **Vulnérabilités dans le code PHP de l'application** (injection SQL, XSS, désérialisation non sécurisée, etc.). FrankenPHP livre les données de requête non fiables à l'application sans les modifier ; la défense contre celles-ci relève de la responsabilité de l'application, comme avec toute SAPI.
- **Défauts dans les composants en amont** utilisés par FrankenPHP (PHP, Caddy, Go) ou dans les projets construits sur lui (Laravel Octane, Symfony Runtime). Veuillez les signaler au projet concerné.

## Signaler une vulnérabilité

Consultez [`SECURITY.md`](../SECURITY.md) pour savoir comment signaler un problème de sécurité affectant FrankenPHP.
