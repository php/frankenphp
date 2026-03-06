# Création d'une image Docker personnalisée

Les images Docker de [FrankenPHP](https://hub.docker.com/r/dunglas/frankenphp) sont basées sur les [images PHP officielles](https://hub.docker.com/_/php/). Des variantes Debian et Alpine Linux sont fournies pour les architectures populaires. Les variantes Debian sont recommandées.

Des variantes pour PHP 8.2, 8.3, 8.4 et 8.5 sont disponibles. [Parcourir les tags](https://hub.docker.com/r/dunglas/frankenphp/tags).

Les tags suivent le pattern suivant : `dunglas/frankenphp:<frankenphp-version>-php<php-version>-<os>`

- `<frankenphp-version>` et `<php-version>` sont respectivement les numéros de version de FrankenPHP et PHP, allant de majeur (e.g. `1`), mineur (e.g. `1.2`) à des versions correctives (e.g. `1.2.3`).
- `<os>` est soit `trixie` (pour Debian Trixie), `bookworm` (pour Debian Bookworm), ou `alpine` (pour la dernière version stable d'Alpine).

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

## Comment ajuster la configuration

Pour une meilleure expérience initiale, un [`Caddyfile` par défaut](https://github.com/php/frankenphp/blob/main/caddy/frankenphp/Caddyfile) contenant des variables d'environnement communément utilisées est fourni dans l'image.

## Comment installer plus d'extensions PHP

Le script [`docker-php-extension-installer`](https://github.com/mlocati/docker-php-extension-installer) est fourni dans l'image de base. L'ajout d'extensions PHP supplémentaires se fait de cette manière :

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

L'image `builder` fournie par FrankenPHP contient une version compilée de `libphp`.
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
	# Donner l'accès en écriture à /config/caddy et /data/caddy
	chown -R ${USER}:${USER} /config/caddy /data/caddy

USER ${USER}
```

### Exécution sans capacité

Même lorsqu'il s'exécute en tant qu'utilisateur non-root, FrankenPHP a besoin de la capacité `CAP_NET_BIND_SERVICE` pour lier le serveur web sur les ports privilégiés (80 et 443).

Si vous exposez FrankenPHP sur un port non privilégié (1024 et au-delà), il est possible d'exécuter le serveur web en tant qu'utilisateur non-root, et sans avoir besoin d'aucune capacité :

```dockerfile
FROM dunglas/frankenphp

ARG USER=appuser

RUN \
	# Utiliser "adduser -D ${USER}" pour les distros basées sur Alpine
	useradd ${USER}; \
	# Supprimer la capacité par défaut
	setcap -r /usr/local/bin/frankenphp; \
	# Donner un accès en écriture à /config/caddy et /data/caddy
	chown -R ${USER}:${USER} /config/caddy /data/caddy

USER ${USER}
```

Ensuite, définissez la variable d'environnement `SERVER_NAME` pour utiliser un port non privilégié.
Exemple : `:8000`

## Mises à jour

Les images Docker sont construites :

- lorsqu'une nouvelle version est taguée
- tous les jours à 4h UTC, si de nouvelles versions des images officielles PHP sont disponibles

## Durcir la sécurité des images

Pour réduire davantage la surface d'attaque et la taille de vos images Docker FrankenPHP, il est également possible de les construire sur une image [Google distroless](https://github.com/GoogleContainerTools/distroless) ou [Docker hardened](https://www.docker.com/products/hardened-images).

> [!WARNING]
> Ces images de base minimales n'incluent pas de shell ou de gestionnaire de paquets, ce qui rend le débogage plus difficile. Elles sont donc recommandées uniquement pour la production si la sécurité est une priorité.

Lors de l'ajout d'extensions PHP supplémentaires, vous aurez besoin d'une étape de build intermédiaire :

```dockerfile
FROM dunglas/frankenphp AS builder

# Ajoutez ici des extensions PHP supplémentaires
RUN install-php-extensions pdo_mysql pdo_pgsql #...

# Copiez les bibliothèques partagées de frankenphp et toutes les extensions installées vers un emplacement temporaire
# Vous pouvez également effectuer cette étape manuellement en analysant la sortie ldd du binaire frankenphp et de chaque fichier .so d'extension
RUN apt-get update && apt-get install -y libtree && \
    EXT_DIR="$(php -r 'echo ini_get("extension_dir");')" && \
    FRANKENPHP_BIN="$(which frankenphp)"; \
    LIBS_TMP_DIR="/tmp/libs"; \
    mkdir -p "$LIBS_TMP_DIR"; \
    for target in "$FRANKENPHP_BIN" $(find "$EXT_DIR" -maxdepth 2 -type f -name "*.so"); do \
        libtree -pv "$target" | sed 's/.*── \(.*\) \[.*/\1/' | grep -v "^$target" | while IFS= read -r lib; do \
            [ -z "$lib" ] && continue; \
            base=$(basename "$lib"); \
            destfile="$LIBS_TMP_DIR/$base"; \
            if [ ! -f "$destfile" ]; then \
                cp "$lib" "$destfile"; \
            fi; \
        done; \
    done


# Image de base Debian distroless, assurez-vous que c'est la même version de Debian que l'image de base
FROM gcr.io/distroless/base-debian13
# Alternative d'image Docker renforcée
# FROM dhi.io/debian:13

# Emplacement de votre application et du Caddyfile à copier dans le conteneur
ARG PATH_TO_APP="."
ARG PATH_TO_CADDYFILE="./Caddyfile"

# Copiez votre application dans /app
# Pour un durcissement supplémentaire, assurez-vous que seuls les chemins accessibles en écriture sont détenus par l'utilisateur non-root
COPY --chown=nonroot:nonroot "$PATH_TO_APP" /app
COPY "$PATH_TO_CADDYFILE" /etc/caddy/Caddyfile

# Copiez frankenphp et les bibliothèques nécessaires
COPY --from=builder /usr/local/bin/frankenphp /usr/local/bin/frankenphp
COPY --from=builder /usr/local/lib/php/extensions /usr/local/lib/php/extensions
COPY --from=builder /tmp/libs /usr/lib

# Copiez les fichiers de configuration php.ini
COPY --from=builder /usr/local/etc/php/conf.d /usr/local/etc/php/conf.d
COPY --from=builder /usr/local/etc/php/php.ini-production /usr/local/etc/php/php.ini

# Répertoires de données Caddy — doivent être accessibles en écriture pour l'utilisateur non-root, même sur un système de fichiers racine en lecture seule
ENV XDG_CONFIG_HOME=/config \
    XDG_DATA_HOME=/data
COPY --from=builder --chown=nonroot:nonroot /data/caddy /data/caddy
COPY --from=builder --chown=nonroot:nonroot /config/caddy /config/caddy

USER nonroot

WORKDIR /app

# Point d'entrée pour exécuter frankenphp avec le Caddyfile fourni
ENTRYPOINT ["/usr/local/bin/frankenphp", "run", "-c", "/etc/caddy/Caddyfile"]
```

## Versions de développement

Les versions de développement sont disponibles dans le dépôt Docker [`dunglas/frankenphp-dev`](https://hub.docker.com/repository/docker/dunglas/frankenphp-dev). Un nouveau build est déclenché chaque fois qu'un commit est poussé sur la branche principale du dépôt GitHub.

Les tags `latest*` pointent vers la tête de la branche `main`.
Les tags sous la forme `sha-<git-commit-hash>` sont également disponibles.
