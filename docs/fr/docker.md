# Création d'une image Docker personnalisée

Les images Docker de [FrankenPHP](https://hub.docker.com/r/dunglas/frankenphp) sont basées sur les [images PHP officielles](https://hub.docker.com/_/php/). Des variantes Debian et Alpine Linux sont fournies pour les architectures populaires. Les variantes Debian sont recommandées.

Des variantes pour PHP 8.2, 8.3 et 8.4 sont disponibles. [Parcourir les tags](https://hub.docker.com/r/dunglas/frankenphp/tags).

Les tags suivent le pattern suivant: `dunglas/frankenphp:<frankenphp-version>-php<php-version>-<os>`

- `<frankenphp-version>` et `<php-version>` sont repsectivement les numéros de version de FrankenPHP et PHP, allant de majeur (e.g. `1`), mineur (e.g. `1.2`) à des versions correctives (e.g. `1.2.3`).
- `<os>` est soit `trixie` (pour Debian Trixie), `bookworm` (pour Debian Bookworm) ou `alpine` (pour la dernière version stable d'Alpine).

[Parcourir les tags](https://hub.docker.com/r/dunglas/frankenphp/tags).

## Comment utiliser les images

Créez un `Dockerfile` dans votre projet :

```dockerfile
FROM dunglas/frankenphp

COPY . /app/public
```

Ensuite, exécutez ces commandes pour construire et exécuter l'image Docker :

```console
docker build -t my-php-app .
docker run -it --rm --name my-running-app my-php-app
```

## Comment installer plus d'extensions PHP

Le script [`docker-php-extension-installer`](https://github.com/mlocati/docker-php-extension-installer) est fourni dans l'image de base.
Il est facile d'ajouter des extensions PHP supplémentaires :

```dockerfile
FROM dunglas/frankenphp

# ajoutez des extensions supplémentaires ici :
RUN install-php-extensions \
	pdo_mysql \
	gd \
	intl \
	zip \
	opcache
```

## Comment installer plus de modules Caddy

FrankenPHP est construit sur Caddy, et tous les [modules Caddy](https://caddyserver.com/docs/modules/) peuvent être utilisés avec FrankenPHP.

La manière la plus simple d'installer des modules Caddy personnalisés est d'utiliser [xcaddy](https://github.com/caddyserver/xcaddy) :

```dockerfile
FROM dunglas/frankenphp:builder AS builder

# Copier xcaddy dans l'image du constructeur
COPY --from=caddy:builder /usr/bin/xcaddy /usr/bin/xcaddy

# CGO doit être activé pour construire FrankenPHP
RUN CGO_ENABLED=1 \
    XCADDY_SETCAP=1 \
    XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
    CGO_CFLAGS=$(php-config --includes) \
    CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
    xcaddy build \
        --output /usr/local/bin/frankenphp \
        --with github.com/dunglas/frankenphp=./ \
        --with github.com/dunglas/frankenphp/caddy=./caddy/ \
        --with github.com/dunglas/caddy-cbrotli \
        # Mercure et Vulcain sont inclus dans la construction officielle, mais n'hésitez pas à les retirer
        --with github.com/dunglas/mercure/caddy \
        --with github.com/dunglas/vulcain/caddy
        # Ajoutez des modules Caddy supplémentaires ici

FROM dunglas/frankenphp AS runner

# Remplacer le binaire officiel par celui contenant vos modules personnalisés
COPY --from=builder /usr/local/bin/frankenphp /usr/local/bin/frankenphp
```

L'image builder fournie par FrankenPHP contient une version compilée de `libphp`.
[Les images builder](https://hub.docker.com/r/dunglas/frankenphp/tags?name=builder) sont fournies pour toutes les versions de FrankenPHP et PHP, à la fois pour Debian et Alpine.

> [!TIP]
>
> Si vous utilisez Alpine Linux et Symfony,
> vous devrez peut-être [augmenter la taille de pile par défaut](compile.md#utiliser-xcaddy).

## Activer le mode Worker par défaut

Définissez la variable d'environnement `FRANKENPHP_CONFIG` pour démarrer FrankenPHP avec un script worker :

```dockerfile
FROM dunglas/frankenphp

# ...

ENV FRANKENPHP_CONFIG="worker ./public/index.php"
```

## Utiliser un volume en développement

Pour développer facilement avec FrankenPHP, montez le répertoire de l'hôte contenant le code source de l'application comme un volume dans le conteneur Docker :

```console
docker run -v $PWD:/app/public -p 80:80 -p 443:443 -p 443:443/udp --tty my-php-app
```

> [!TIP]
>
> L'option --tty permet d'avoir des logs lisibles par un humain au lieu de logs JSON.

Avec Docker Compose :

```yaml
# compose.yaml

services:
  php:
    image: dunglas/frankenphp
    # décommentez la ligne suivante si vous souhaitez utiliser un Dockerfile personnalisé
    #build: .
    # décommentez la ligne suivante si vous souhaitez exécuter ceci dans un environnement de production
    # restart: always
    ports:
      - "80:80" # HTTP
      - "443:443" # HTTPS
      - "443:443/udp" # HTTP/3
    volumes:
      - ./:/app/public
      - caddy_data:/data
      - caddy_config:/config
    # commentez la ligne suivante en production, elle permet d'avoir de beaux logs lisibles en dev
    tty: true

# Volumes nécessaires pour les certificats et la configuration de Caddy
volumes:
  caddy_data:
  caddy_config:
```

## Exécution en tant qu'utilisateur non-root

FrankenPHP peut s'exécuter en tant qu'utilisateur non-root dans Docker.

Voici un exemple de `Dockerfile` le permettant :

```dockerfile
FROM dunglas/frankenphp

ARG USER=appuser

RUN \
	# Utilisez "adduser -D ${USER}" pour les distributions basées sur Alpine
	useradd ${USER}; \
	# Ajouter la capacité supplémentaire de se lier aux ports 80 et 443
	setcap CAP_NET_BIND_SERVICE=+eip /usr/local/bin/frankenphp; \
	# Donner l'accès en écriture à /data/caddy et /config/caddy
	chown -R ${USER}:${USER} /data/caddy && chown -R ${USER}:${USER} /config/caddy

USER ${USER}
```

### Exécution sans capacité

Même lorsqu'il s'exécute en tant qu'utilisateur autre que root, FrankenPHP a besoin de la capacité `CAP_NET_BIND_SERVICE`
pour que son serveur utilise les ports privilégiés (80 et 443).

Si vous exposez FrankenPHP sur un port non privilégié (à partir de 1024), il est possible de faire fonctionner le serveur web avec un utilisateur qui n'est pas root, et sans avoir besoin d'aucune capacité.

```dockerfile
FROM dunglas/frankenphp

ARG USER=appuser

RUN \
	# Utiliser "adduser -D ${USER}" pour les distros basées sur Alpine
	useradd ${USER}; \
	# Supprimer la capacité par défaut \
	setcap -r /usr/local/bin/frankenphp; \
	# Donner un accès en écriture à /data/caddy et /config/caddy \
	chown -R ${USER}:${USER} /data/caddy && chown -R ${USER}:${USER} /config/caddy

USER ${USER}
```

Ensuite, définissez la variable d'environnement `SERVER_NAME` pour utiliser un port non privilégié.
Exemple `:8000`

## Mises à jour

Les images Docker sont construites :

- lorsqu'une nouvelle version est taguée
- tous les jours à 4h UTC, si de nouvelles versions des images officielles PHP sont disponibles

## Versions de développement

Les versions de développement sont disponibles dans le dépôt Docker [`dunglas/frankenphp-dev`](https://hub.docker.com/repository/docker/dunglas/frankenphp-dev). Un nouveau build est déclenché chaque fois qu'un commit est poussé sur la branche principale du dépôt GitHub.

Les tags `latest*` pointent vers la tête de la branche `main`.
Les tags sous la forme `sha-<hash-du-commit-git>` sont également disponibles.
