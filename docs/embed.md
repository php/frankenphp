# PHP Apps As Standalone Binaries

FrankenPHP has the ability to embed the source code and assets of PHP applications in a static, self-contained binary.

Thanks to this feature, PHP applications can be distributed as standalone binaries that include the application itself, the PHP interpreter, and Caddy, a production-level web server.

Learn more about this feature [in the presentation made by Kévin at SymfonyCon 2023](https://dunglas.dev/2023/12/php-and-symfony-apps-as-standalone-binaries/).

For embedding Laravel applications, [read this specific documentation entry](laravel.md#laravel-apps-as-standalone-binaries).

## Preparing Your App

Before creating the self-contained binary be sure that your app is ready for embedding.

For instance, you likely want to:

- Install the production dependencies of the app
- Dump the autoloader
- Enable the production mode of your application (if any)
- Strip unneeded files such as `.git` or tests to reduce the size of your final binary

For instance, for a Symfony app, you can use the following commands:

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

### Customizing the Configuration

To customize [the configuration](config.md), you can put a `Caddyfile` as well as a `php.ini` file
in the main directory of the app to be embedded (`$TMPDIR/my-prepared-app` in the previous example).

## Creating a Linux Binary

The easiest way to create a Linux binary is to use the Docker-based builder we provide.

1. Create a file named `static-build.Dockerfile` in the repository of your app:

   ```dockerfile
   FROM dunglas/frankenphp:static-builder-gnu
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

2. Build:

   ```console
   docker build -t static-app -f static-build.Dockerfile .
   ```

3. Extract the binary:

   ```console
   docker cp $(docker create --name static-app-tmp static-app):/go/src/app/dist/frankenphp-linux-x86_64 my-app ; docker rm static-app-tmp
   ```

The resulting binary is the file named `my-app` in the current directory.

## Creating a Binary for Other OSes

If you don't want to use Docker, or want to build a macOS binary, use the shell script we provide:

```console
git clone https://github.com/php/frankenphp
cd frankenphp
EMBED=/path/to/your/app ./build-static.sh
```

The resulting binary is the file named `frankenphp-<os>-<arch>` in the `dist/` directory.

## Using The Binary

This is it! The `my-app` file (or `dist/frankenphp-<os>-<arch>` on other OSes) contains your self-contained app!

To start the web app run:

```console
./my-app php-server
```

If your app contains a [worker script](worker.md), start the worker with something like:

```console
./my-app php-server --worker public/index.php
```

To enable HTTPS (a Let's Encrypt certificate is automatically created), HTTP/2, and HTTP/3, specify the domain name to use:

```console
./my-app php-server --domain localhost
```

You can also run the PHP CLI scripts embedded in your binary:

```console
./my-app php-cli bin/console
```

## PHP Extensions

By default, the script will build extensions required by the `composer.json` file of your project, if any.
If the `composer.json` file doesn't exist, the default extensions are built, as documented in [the static builds entry](static.md).

To customize the extensions, use the `PHP_EXTENSIONS` environment variable.

## Customizing The Build

[Read the static build documentation](static.md) to see how to customize the binary (extensions, PHP version...).

## Distributing The Binary

On Linux, the created binary is compressed using [UPX](https://upx.github.io).

On Mac, to reduce the size of the file before sending it, you can compress it.
We recommend `xz`.
