# Utilisation de GitHub Actions

Ce dépôt construit et déploie l'image Docker sur [le Hub Docker](https://hub.docker.com/r/dunglas/frankenphp) pour
chaque pull request approuvée ou sur votre propre fork une fois configuré.

## Configuration de GitHub Actions

Dans les paramètres du dépôt, sous "secrets", ajoutez les secrets suivants :

- `REGISTRY_LOGIN_SERVER` : Le registre Docker à utiliser (par exemple, `docker.io`).
- `REGISTRY_USERNAME` : Le nom d'utilisateur à utiliser pour se connecter au registre (par exemple, `dunglas`).
- `REGISTRY_PASSWORD` : Le mot de passe à utiliser pour se connecter au registre (par exemple, une clé d'accès).
- `IMAGE_NAME` : Le nom de l'image (par exemple, `dunglas/frankenphp`).

## Construction et push de l'image

1. Créez une Pull Request ou poussez vers votre fork.
2. GitHub Actions va construire l'image et exécuter tous les tests.
3. Si la construction est réussie, l'image sera poussée vers le registre en utilisant le tag `pr-x`, où `x` est le numéro de la PR.

## Déploiement de l'image

1. Une fois la Pull Request fusionnée, GitHub Actions exécutera à nouveau les tests et construira une nouvelle image.
2. Si la construction est réussie, le tag `main` sera mis à jour dans le registre Docker.

## Releases

1. Créez un nouveau tag dans le dépôt.
2. GitHub Actions va construire l'image et exécuter tous les tests.
3. Si la construction est réussie, l'image sera poussée vers le registre en utilisant le nom du tag comme tag (par exemple, `v1.2.3` et `v1.2` seront créés).
4. Le tag `latest` sera également mis à jour.
