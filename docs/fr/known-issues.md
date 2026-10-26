---
title: Problèmes connus, extensions incompatibles et solutions de contournement de FrankenPHP
description: Référence des limitations de FrankenPHP, y compris les extensions PHP non prises en charge, les mises en garde de musl libc, la configuration TLS de Docker avec 127.0.0.1, et les scripts Composer @php.
---

# Problèmes connus

## Extensions PHP non prises en charge

Les extensions suivantes sont connues pour ne pas être compatibles avec FrankenPHP :

| Nom                                                                                                         | Raison          | Alternatives                                                                                                                                                                    |
| :---------------------------------------------------------------------------------------------------------- | :-------------- | :------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| [imap](https://www.php.net/manual/imap.installation.php)                                                    | Non thread-safe | [javanile/php-imap2](https://github.com/javanile/php-imap2), [webklex/php-imap](https://github.com/Webklex/php-imap), [ImapEngine](https://github.com/DirectoryTree/ImapEngine) |
| [newrelic](https://docs.newrelic.com/docs/apm/agents/php-agent/getting-started/introduction-new-relic-php/) | Non thread-safe | -                                                                                                                                                                               |
| [pcov](https://github.com/krakjoe/pcov)                                                                     | Non thread-safe ([SIGSEGV en mode classique](https://github.com/krakjoe/pcov/issues/132)) | Collecter la couverture avec le SAPI CLI (exécuter PHPUnit/Pest en dehors de FrankenPHP) |

## Extensions PHP boguées

Les extensions suivantes ont des bugs connus et des comportements inattendus lorsqu'elles sont utilisées avec FrankenPHP :

| Nom                                     | Problème                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| :-------------------------------------- | :----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [datadog](https://github.com/DataDog)   | Instabilité lors du profilage de FrankenPHP. Ce bug est [suivi par DataDog](https://github.com/DataDog/dd-trace-php/issues/3729).                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| [blackfire](https://blackfire.io/)      | [Le support de FrankenPHP est en bêta et n'est pas encore complet](https://docs.blackfire.io/php/integrations/frankenphp).                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| [imagick](https://github.com/Imagick/imagick) | Les threads OpenMP d'ImageMagick entrent en conflit avec les threads de FrankenPHP, causant instabilité et plantages. Atténuez cela en désactivant le threading d'ImageMagick via ``\Imagick::setResourceLimit(\Imagick::RESOURCETYPE_THREAD, 1)`` ou en compilant ImageMagick avec ``--disable-openmp``. OpenMP est déjà désactivé dans les binaires statiques et les paquets officiels ``apt``/``apk``/``rpm``, donc seules les images Docker et les installations Homebrew sont affectées. |

## Dépasser la limite de mémoire dure de PHP arrête l'ensemble du serveur

Depuis PHP 8.2.35, 8.3.36, 8.4.27 et 8.5.12, dépasser `memory_limit` n'arrête plus le script immédiatement : l'erreur fatale "Allowed memory size exhausted" est déclenchée lors du prochain appel de fonction ou itération de boucle.
Si le script alloue au-delà de la limite de mémoire dure avant cela, PHP quitte le processus.
La limite dure est par défaut le double de `memory_limit` (ou `memory_limit` plus 1 GiB au-dessus de 1 GiB) et peut être modifiée avec le paramètre INI `memory_hard_limit_percent`.

Le code qui continue de s'exécuter au-delà de `memory_limit` peut l'atteindre, par exemple une seule allocation plus grande que `memory_limit` (comme `array_merge()` sur de grands tableaux) ou des concaténations de chaînes consécutives.

PHP-FPM ne perd qu'un seul processus enfant.
FrankenPHP sert toutes les requêtes à partir de threads d'un seul processus, de sorte que chaque requête en cours est perdue et le serveur s'arrête jusqu'à ce qu'il soit redémarré.
Exécutez FrankenPHP sous un superviseur qui le redémarre, comme une politique de redémarrage Docker ou `Restart=` de systemd (déjà configuré dans les paquets `deb` et `rpm`).

## get_browser

La fonction [get_browser()](https://www.php.net/manual/function.get-browser.php) semble avoir de mauvaises performances après un certain temps. Une solution est de mettre en cache (par exemple, avec [APCu](https://www.php.net/manual/book.apcu.php)) les résultats par agent utilisateur, car ils sont statiques.

## Binaire autonome et images Docker basées sur Alpine

Le binaire entièrement statique et les images Docker basées sur Alpine (`dunglas/frankenphp:*-alpine`) utilisent [musl libc](https://musl.libc.org/) au lieu de [glibc et ses amis](https://www.etalabs.net/compare_libcs.html), pour garder une taille de binaire plus petite.
Cela peut entraîner des problèmes de compatibilité.
En particulier, le drapeau glob `GLOB_BRACE` n'est [pas disponible](https://www.php.net/manual/function.glob.php).

Préférez utiliser la variante GNU du binaire statique et les images Docker basées sur Debian si vous rencontrez des problèmes.

## Utilisation de `https://127.0.0.1` avec Docker

Par défaut, FrankenPHP génère un certificat TLS pour `localhost`.
C'est l'option la plus simple et recommandée pour le développement local.

Si vous voulez vraiment utiliser `127.0.0.1` comme hôte, il est possible de configurer FrankenPHP pour générer un certificat pour cela en définissant le nom du serveur à `127.0.0.1`.

Malheureusement, cela ne suffit pas lors de l'utilisation de Docker à cause de [son système de gestion des réseaux](https://docs.docker.com/network/).
Vous obtiendrez une erreur TLS similaire à `curl: (35) LibreSSL/3.3.6: error:1404B438:SSL routines:ST_CONNECT:tlsv1 alert internal error`.

Si vous utilisez Linux, une solution est d'utiliser [le pilote de réseau "hôte"](https://docs.docker.com/network/network-tutorial-host/) :

```console
docker run \
    -e SERVER_NAME="127.0.0.1" \
    -v $PWD:/app/public \
    --network host \
    dunglas/frankenphp
```

Le pilote de réseau "hôte" n'est pas pris en charge sur Mac et Windows. Sur ces plateformes, vous devrez deviner l'adresse IP du conteneur et l'inclure dans les noms de serveur.

Exécutez la commande `docker network inspect bridge` et inspectez la clé `Containers` pour identifier la dernière adresse IP attribuée sous la clé `IPv4Address`, puis incrémentez-la d'un. Si aucun conteneur n'est en cours d'exécution, la première adresse IP attribuée est généralement `172.17.0.2`.

Ensuite, incluez ceci dans la variable d'environnement `SERVER_NAME` :

```console
docker run \
    -e SERVER_NAME="127.0.0.1, 172.17.0.3" \
    -v $PWD:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

> [!CAUTION]
>
> Assurez-vous de remplacer `172.17.0.3` par l'IP qui sera attribuée à votre conteneur.

Vous devriez maintenant pouvoir accéder à `https://127.0.0.1` depuis la machine hôte.

Si ce n'est pas le cas, lancez FrankenPHP en mode debug pour essayer de comprendre le problème :

```console
docker run \
    -e CADDY_GLOBAL_OPTIONS="debug" \
    -e SERVER_NAME="127.0.0.1" \
    -v $PWD:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

## Scripts Composer faisant référence à `@php`

Les [scripts Composer](https://getcomposer.org/doc/articles/scripts.md) peuvent vouloir exécuter un binaire PHP pour certaines tâches, par exemple dans [un projet Laravel](laravel.md) pour exécuter `@php artisan package:discover --ansi`. Cela [échoue actuellement](https://github.com/php/frankenphp/issues/483#issuecomment-1899890915) pour deux raisons :

- Composer ne sait pas comment appeler le binaire FrankenPHP ;
- Composer peut ajouter des paramètres PHP en utilisant le paramètre `-d` dans la commande, ce que FrankenPHP ne supporte pas encore.

Comme solution de contournement, nous pouvons créer un script shell dans `/usr/local/bin/php` qui supprime les paramètres non supportés et appelle ensuite FrankenPHP :

```bash
#!/usr/bin/env bash
# /usr/local/bin/php
args=("$@")
index=0
for i in "$@"
do
    if [ "$i" == "-d" ]; then
        unset 'args[$index]'
        unset 'args[$index+1]'
    fi
    index=$((index+1))
done

/usr/local/bin/frankenphp php-cli ${args[@]}
```

Ensuite, définissez la variable d'environnement `PHP_BINARY` au chemin de notre script `php` et lancez Composer :

```console
export PHP_BINARY=/usr/local/bin/php
composer install
```

## Résolution des problèmes TLS/SSL avec les binaires statiques

Lorsque vous utilisez les binaires statiques, vous pouvez rencontrer les erreurs suivantes liées à TLS, par exemple lors de l'envoi de courriels utilisant STARTTLS :

```text
Unable to connect with STARTTLS: stream_socket_enable_crypto(): SSL operation failed with code 5. OpenSSL Error messages:
error:80000002:system library::No such file or directory
error:80000002:system library::No such file or directory
error:80000002:system library::No such file or directory
error:0A000086:SSL routines::certificate verify failed
```

Comme le binaire statique ne contient pas de certificats TLS, vous devez indiquer à OpenSSL l'installation de vos certificats CA locaux.

Inspectez la sortie de [`openssl_get_cert_locations()`](https://www.php.net/manual/function.openssl-get-cert-locations.php),
pour trouver l'endroit où les certificats CA doivent être installés et stockez-les à cet endroit.

> [!WARNING]
>
> Les contextes Web et CLI peuvent avoir des paramètres différents.
> Assurez-vous d'exécuter `openssl_get_cert_locations()` dans le bon contexte.

[Les certificats CA extraits de Mozilla peuvent être téléchargés sur le site de cURL](https://curl.se/docs/caextract.html).

Alternativement, de nombreuses distributions, y compris Debian, Ubuntu, et Alpine fournissent des paquets nommés `ca-certificates` qui contiennent ces certificats.

Il est également possible d'utiliser `SSL_CERT_FILE` et `SSL_CERT_DIR` pour indiquer à OpenSSL où chercher les certificats CA :

```console
# Définir les variables d'environnement des certificats TLS
export SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
export SSL_CERT_DIR=/etc/ssl/certs
```
