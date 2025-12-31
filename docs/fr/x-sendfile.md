# Servir efficacement les gros fichiers statiques (`X-Sendfile`/`X-Accel-Redirect`)

Habituellement, les fichiers statiques peuvent être servis directement par le serveur web,
mais parfois, il est nécessaire d'exécuter du code PHP avant de les envoyer :
contrôle d'accès, statistiques, en-têtes HTTP personnalisés...

Malheureusement, utiliser PHP pour servir de gros fichiers statiques est inefficace comparé à
l'utilisation directe du serveur web (surcharge mémoire, diminution des performances...).

FrankenPHP permet de déléguer l'envoi des fichiers statiques au serveur web
**après** avoir exécuté du code PHP personnalisé.

Pour ce faire, votre application PHP n'a qu'à définir un en-tête HTTP personnalisé
contenant le chemin du fichier à servir. FrankenPHP se chargera du reste.

Cette fonctionnalité est connue sous le nom de **`X-Sendfile`** pour Apache, et **`X-Accel-Redirect`** pour NGINX.

Dans les exemples suivants, nous supposons que le "document root" du projet est le répertoire `public/`
et que nous voulons utiliser PHP pour servir des fichiers stockés en dehors du dossier `public/`,
depuis un répertoire nommé `private-files/`.

## Configuration

Tout d'abord, ajoutez la configuration suivante à votre `Caddyfile` pour activer cette fonctionnalité :

```patch
	root public/
	# ...

+	# Needed for Symfony, Laravel and other projects using the Symfony HttpFoundation component
+	request_header X-Sendfile-Type x-accel-redirect
+	request_header X-Accel-Mapping ../private-files=/private-files
+
+	intercept {
+		@accel header X-Accel-Redirect *
+		handle_response @accel {
+			root private-files/
+			rewrite * {resp.header.X-Accel-Redirect}
+			method * GET
+
+			# Remove the X-Accel-Redirect header set by PHP for increased security
+			header -X-Accel-Redirect
+
+			file_server
+		}
+	}

	php_server
```

## PHP simple

Définissez le chemin relatif du fichier (à partir de `private-files/`) comme valeur de l'en-tête `X-Accel-Redirect` :

```php
header('X-Accel-Redirect: file.txt') ;
```

## Projets utilisant le composant Symfony HttpFoundation (Symfony, Laravel, Drupal...)

Symfony HttpFoundation [supporte nativement cette fonctionnalité](https://symfony.com/doc/current/components/http_foundation.html#serving-files).
Il va automatiquement déterminer la bonne valeur pour l'en-tête `X-Accel-Redirect` et l'ajoutera à la réponse.

```php
use Symfony\Component\HttpFoundation\BinaryFileResponse;

BinaryFileResponse::trustXSendfileTypeHeader();
$response = new BinaryFileResponse(__DIR__.'/../private-files/file.txt');

// ...
```
