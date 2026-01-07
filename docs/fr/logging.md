# Journalisation

FrankenPHP s'intègre parfaitement au [système de journalisation de Caddy](https://caddyserver.com/docs/logging). Vous pouvez journaliser des messages en utilisant les fonctions PHP standard ou exploiter la fonction dédiée `frankenphp_log()` pour des capacités de journalisation structurée avancées.

## `frankenphp_log()`

La fonction `frankenphp_log()` vous permet d'émettre des journaux structurés directement depuis votre application PHP, facilitant grandement leur ingestion dans des plateformes comme Datadog, Grafana Loki ou Elastic, ainsi que le support d'OpenTelemetry.

En interne, `frankenphp_log()` enveloppe le [package `log/slog` de Go](https://pkg.go.dev/log/slog) pour offrir des fonctionnalités de journalisation riches.

Ces journaux incluent le niveau de sévérité et des données de contexte optionnelles.

```php
function frankenphp_log(string $message, int $level = FRANKENPHP_LOG_LEVEL_INFO, array $context = []): void
```

### Paramètres

- **`message`** : La chaîne de caractères du message de journal.
- **`level`** : Le niveau de sévérité du journal. Peut être n'importe quel entier arbitraire. Des constantes de commodité sont fournies pour les niveaux courants : `FRANKENPHP_LOG_LEVEL_DEBUG` (`-4`), `FRANKENPHP_LOG_LEVEL_INFO` (`0`), `FRANKENPHP_LOG_LEVEL_WARN` (`4`) et `FRANKENPHP_LOG_LEVEL_ERROR` (`8`)). La valeur par défaut est `FRANKENPHP_LOG_LEVEL_INFO`.
- **`context`** : Un tableau associatif de données additionnelles à inclure dans l'entrée du journal.

### Exemple

```php
<?php

// Journalise un simple message d'information
frankenphp_log("Hello from FrankenPHP!");

// Journalise un avertissement avec des données de contexte
frankenphp_log(
    "Memory usage high",
    FRANKENPHP_LOG_LEVEL_WARN,
    [
        'current_usage' => memory_get_usage(),
        'peak_usage' => memory_get_peak_usage(),
    ],
);

```

Lorsque vous consultez les journaux (par exemple, via `docker compose logs`), la sortie apparaîtra sous forme de JSON structuré :

```json
{"level":"info","ts":1704067200,"logger":"frankenphp","msg":"Hello from FrankenPHP!"}
{"level":"warn","ts":1704067200,"logger":"frankenphp","msg":"Memory usage high","current_usage":10485760,"peak_usage":12582912}
```

## `error_log()`

FrankenPHP permet également la journalisation en utilisant la fonction standard `error_log()`. Si le paramètre `$message_type` est `4` (SAPI), ces messages sont acheminés vers le journaliseur de Caddy.

Par défaut, les messages envoyés via `error_log()` sont traités comme du texte non structuré. Ils sont utiles pour la compatibilité avec les applications ou bibliothèques existantes qui s'appuient sur la bibliothèque PHP standard.

### Exemple avec error_log()

```php
error_log("Database connection failed", 4);
```

Ceci apparaîtra dans les journaux de Caddy, souvent préfixé pour indiquer qu'il provient de PHP.

> [!TIP]
> Pour une meilleure observabilité dans les environnements de production, préférez `frankenphp_log()`
> car cela vous permet de filtrer les journaux par niveau (Débogage, Erreur, etc.)
> et d'interroger des champs spécifiques dans votre infrastructure de journalisation.
