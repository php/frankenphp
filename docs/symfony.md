# Symfony

## Docker

For [Symfony](https://symfony.com) projects, we recommend using [Symfony Docker](https://github.com/dunglas/symfony-docker), the official Symfony Docker setup maintained by FrankenPHP's author. It provides a complete Docker-based environment with FrankenPHP, automatic HTTPS, HTTP/2, HTTP/3, and worker mode support out of the box.

## Local Installation

Alternatively, you can run your Symfony projects with FrankenPHP from your local machine:

1. [Install FrankenPHP](../#getting-started)
2. Add the following configuration to a file named `Caddyfile` in the root directory of your Symfony project:

   ```caddyfile
   # The domain name of your server
   localhost

   root public/
   php_server {
   	# Optional: Enable worker mode for better performance
   	worker ./public/index.php
   }
   ```

   See the [performance documentation](performance.md) for further optimizations.

3. Start FrankenPHP from the root directory of your Symfony project: `frankenphp run`

## Worker Mode

Since Symfony 7.4, FrankenPHP worker mode is natively supported.

For older versions, install the FrankenPHP package of [PHP Runtime](https://github.com/php-runtime/runtime):

```console
composer require runtime/frankenphp-symfony
```

Start your app server by defining the `APP_RUNTIME` environment variable to use the FrankenPHP Symfony Runtime:

```console
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php" \
    -e APP_RUNTIME=Runtime\\FrankenPhpSymfony\\Runtime \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

Learn more about [the worker mode](worker.md).

### Auditing Worker Compatibility

[Igor PHP](https://github.com/igor-php/igor-php) is a static linter that scans Symfony projects for state leaks before they bite in production: services missing `ResetInterface`, stateful properties that aren't reset, mutable local statics, `exit()`/`die()` calls, and superglobal writes. It audits your application code as well as services declared in `vendor/`.

```console
composer require --dev igor-php/igor-php
vendor/bin/igor-php .
```

## Hot Reload

Hot reloading is enabled by default in [Symfony Docker](https://github.com/dunglas/symfony-docker).

To use the [hot reload](hot-reload.md) feature without Symfony Docker, enable [Mercure](mercure.md) and add the `hot_reload` sub-directive to the `php_server` directive in your `Caddyfile`:

```caddyfile
localhost

mercure {
	anonymous
}

root public/
php_server {
	hot_reload
	worker ./public/index.php
}
```

Then, add the following code to your `templates/base.html.twig` file:

```twig
{% if app.request.server.has('FRANKENPHP_HOT_RELOAD') %}
    <meta name="frankenphp-hot-reload:url" content="{{ app.request.server.get('FRANKENPHP_HOT_RELOAD') }}">
    <script src="https://cdn.jsdelivr.net/npm/idiomorph"></script>
    <script src="https://cdn.jsdelivr.net/npm/frankenphp-hot-reload/+esm" type="module"></script>
{% endif %}
```

Finally, run `frankenphp run` from the root directory of your Symfony project.

## Pre-Compressing Assets

Symfony's [AssetMapper component](https://symfony.com/doc/current/frontend/asset_mapper.html) can pre-compress assets with Brotli and Zstandard during deployment. FrankenPHP (through Caddy's `file_server`) can serve these pre-compressed files directly, avoiding on-the-fly compression overhead.

1. Compile and compress your assets:

   ```console
   php bin/console asset-map:compile
   ```

2. Update your `Caddyfile` to serve pre-compressed assets:

   ```caddyfile
   localhost

   @assets path /assets/*
   file_server @assets {
   	precompressed zstd br gzip
   }

   root public/
   php_server {
   	worker ./public/index.php
   }
   ```

The `precompressed` directive tells Caddy to look for pre-compressed versions of the requested file (e.g., `app.css.zst`, `app.css.br`) and serve them directly if the client supports it.

## Serving Large Static Files (`X-Sendfile`)

FrankenPHP supports [efficiently serving large static files](x-sendfile.md) after executing PHP code (for access control, statistics, etc.).

Symfony HttpFoundation [natively supports this feature](https://symfony.com/doc/current/components/http_foundation.html#serving-files).
After [configuring your `Caddyfile`](x-sendfile.md#configuration), it will automatically determine the correct value for the `X-Accel-Redirect` header and add it to the response:

```php
use Symfony\Component\HttpFoundation\BinaryFileResponse;

BinaryFileResponse::trustXSendfileTypeHeader();
$response = new BinaryFileResponse(__DIR__.'/../private-files/file.txt');

// ...
```

## Symfony Apps As Standalone Binaries

Using [FrankenPHP's application embedding feature](embed.md), it's possible to distribute Symfony
apps as standalone binaries.

Follow these steps to prepare and package your Symfony app:

1. Prepare your app:

   ```console
   # Export the project to get rid of .git/, etc
   mkdir $TMPDIR/my-prepared-app
   git archive HEAD | tar -x -C $TMPDIR/my-prepared-app
   cd $TMPDIR/my-prepared-app

   # Set proper environment variables
   echo APP_ENV=prod > .env.local
   echo APP_DEBUG=0 >> .env.local

   # Remove the tests and other unneeded files to save space
   # Alternatively, add these files with the export-ignore attribute in your .gitattributes file
   rm -Rf tests/

   # Install the dependencies
   composer install --ignore-platform-reqs --no-dev -a

   # Optimize .env
   composer dump-env prod
   ```

2. Create a file named `static-build.Dockerfile` in the repository of your app:

   ```dockerfile
   FROM --platform=linux/amd64 dunglas/frankenphp:static-builder-gnu
   # If you intend to run the binary on musl-libc systems, use static-builder-musl instead

   # Copy your app
   WORKDIR /go/src/app/dist/app
   COPY . .

   # Build the static binary
   WORKDIR /go/src/app/
   RUN EMBED=dist/app/ ./build-static.sh
   ```

   > [!CAUTION]
   >
   > Some `.dockerignore` files (e.g. default [Symfony Docker `.dockerignore`](https://github.com/dunglas/symfony-docker/blob/main/.dockerignore))
   > will ignore the `vendor/` directory and `.env` files. Be sure to adjust or remove the `.dockerignore` file before the build.

3. Build:

   ```console
   docker build -t static-symfony-app -f static-build.Dockerfile .
   ```

4. Extract the binary:

   ```console
   docker cp $(docker create --name static-symfony-app-tmp static-symfony-app):/go/src/app/dist/frankenphp-linux-x86_64 my-app ; docker rm static-symfony-app-tmp
   ```

5. Start the server:

   ```console
   ./my-app php-server
   ```

Learn more about the options available and how to build binaries for other OSes in the [applications embedding](embed.md)
documentation.
