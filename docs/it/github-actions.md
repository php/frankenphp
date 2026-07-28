# Usare GitHub Actions

Questo repository crea e distribuisce l'immagine Docker su [Docker Hub](https://hub.docker.com/r/dunglas/frankenphp) su
ogni pull request approvata o sul proprio fork, una volta configurato.

## Configurazione di GitHub Actions

Nelle impostazioni del repository, sotto Secrets, aggiungere i seguenti valori:

- `REGISTRY_LOGIN_SERVER`: il registro Docker da utilizzare (ad esempio `docker.io`).
- `REGISTRY_USERNAME`: Il nome utente da utilizzare per accedere al registro (es. `dunglas`).
- `REGISTRY_PASSWORD`: La password da utilizzare per accedere al registro (ad esempio una chiave di accesso).
- `IMAGE_NAME`: il nome dell'immagine (es. `dunglas/frankenphp`).

## Build e push dell'immagine

1. Aprire una pull request o fare un push nel proprio fork.
2. GitHub Actions creerà l'immagine ed eseguirà eventuali test.
3. Se la creazione ha esito positivo, l'immagine verrà inviata al registro utilizzando `pr-x`, dove `x` è il numero PR, come tag.

## Deploy dell'immagine

1. Dopo il merge della pull request, GitHub Actions eseguirà nuovamente i test e creerà una nuova immagine.
2. Se la compilazione ha esito positivo, il tag `main` verrà aggiornato nel registro Docker.

## Rilasci

1. Creare un nuovo tag nel repository.
2. GitHub Actions creerà l'immagine ed eseguirà eventuali test.
3. Se la creazione ha esito positivo, l'immagine verrà inviata al registro utilizzando il nome del tag come tag (ad esempio, verranno creati `v1.2.3` e `v1.2`).
4. Verrà aggiornato anche il tag `latest`.
