# Laravel

## Docker

Serving a [Laravel](https://laravel.com) web application with FrankenPHP is as easy as mounting the project in the `/app` directory of the official Docker image.

Run this command from the main directory of your Laravel app:

```console
docker run -p 80:80 -p 443:443 -p 443:443/udp -v $PWD:/app dunglas/frankenphp
```

And enjoy!

## Local Installation

Alternatively, you can run your Laravel projects with FrankenPHP from your local machine:

1. [Download the binary corresponding to your system](../#standalone-binary)
2. Add the following configuration to a file named `Caddyfile` in the root directory of your Laravel project:

   ```caddyfile
   {
   	frankenphp
   }

   # The domain name of your server
   localhost {
   	# Set the webroot to the public/ directory
   	root public/
   	# Enable compression (optional)
   	encode zstd br gzip
   	# Execute PHP files from the public/ directory and serve assets
   	php_server {
   		try_files {path} index.php
   	}
   }
   ```

3. Start FrankenPHP from the root directory of your Laravel project: `frankenphp run`

## Laravel Octane

Octane may be installed via the Composer package manager:

```console
composer require laravel/octane
```

After installing Octane, you may execute the `octane:install` Artisan command, which will install Octane's configuration file into your application:

```console
php artisan octane:install --server=frankenphp
```

The Octane server can be started via the `octane:frankenphp` Artisan command.

```console
php artisan octane:frankenphp
```

The `octane:frankenphp` command can take the following options:

- `--host`: The IP address the server should bind to (default: `127.0.0.1`)
- `--port`: The port the server should be available on (default: `8000`)
- `--admin-port`: The port the admin server should be available on (default: `2019`)
- `--workers`: The number of workers that should be available to handle requests (default: `auto`)
- `--max-requests`: The number of requests to process before reloading the server (default: `500`)
- `--caddyfile`: The path to the FrankenPHP `Caddyfile` file (default: [stubbed `Caddyfile` in Laravel Octane](https://github.com/laravel/octane/blob/2.x/src/Commands/stubs/Caddyfile))
- `--https`: Enable HTTPS, HTTP/2, and HTTP/3, and automatically generate and renew certificates
- `--http-redirect`: Enable HTTP to HTTPS redirection (only enabled if --https is passed)
- `--watch`: Automatically reload the server when the application is modified
- `--poll`: Use file system polling while watching in order to watch files over a network
- `--log-level`: Log messages at or above the specified log level, using the native Caddy logger

> [!TIP]
> To get structured JSON logs (useful when using log analytics solutions), explicitly the pass `--log-level` option.

See also [how to use Mercure with Octane](#mercure-support).

Learn more about [Laravel Octane in its official documentation](https://laravel.com/docs/octane).

## Laravel Apps As Standalone Binaries

Using [FrankenPHP's application embedding feature](embed.md), it's possible to distribute Laravel
apps as standalone binaries.

Follow these steps to package your Laravel app as a standalone binary for Linux:

1. Create a file named `static-build.Dockerfile` in the repository of your app:

   ```dockerfile
   FROM --platform=linux/amd64 dunglas/frankenphp:static-builder-gnu
   # If you intend to run the binary on musl-libc systems, use static-builder-musl instead
   
   # Copy your app
   WORKDIR /go/src/app/dist/app
   COPY . .

   # Remove the tests and other unneeded files to save space
   # Alternatively, add these files to a .dockerignore file
   RUN rm -Rf tests/

   # Copy .env file
   RUN cp .env.example .env
   # Change APP_ENV and APP_DEBUG to be production ready
   RUN sed -i'' -e 's/^APP_ENV=.*/APP_ENV=production/' -e 's/^APP_DEBUG=.*/APP_DEBUG=false/' .env

   # Make other changes to your .env file if needed

   # Install the dependencies
   RUN composer install --ignore-platform-reqs --no-dev -a

   # Build the static binary
   WORKDIR /go/src/app/
   RUN EMBED=dist/app/ ./build-static.sh
   ```

   > [!CAUTION]
   >
   > Some `.dockerignore` files
   > will ignore the `vendor/` directory and `.env` files. Be sure to adjust or remove the `.dockerignore` file before the build.

2. Build:

   ```console
   docker build -t static-laravel-app -f static-build.Dockerfile .
   ```

3. Extract the binary:

   ```console
   docker cp $(docker create --name static-laravel-app-tmp static-laravel-app):/go/src/app/dist/frankenphp-linux-x86_64 frankenphp ; docker rm static-laravel-app-tmp
   ```

4. Populate caches:

   ```console
   frankenphp php-cli artisan optimize
   ```

5. Run database migrations (if any):

   ```console
   frankenphp php-cli artisan migrate
   ```

6. Generate app's secret key:

   ```console
   frankenphp php-cli artisan key:generate
   ```

7. Start the server:

   ```console
   frankenphp php-server
   ```

Your app is now ready!

Learn more about the options available and how to build binaries for other OSes in the [applications embedding](embed.md)
documentation.

### Changing The Storage Path

By default, Laravel stores uploaded files, caches, logs, etc. in the application's `storage/` directory.
This is not suitable for embedded applications, as each new version will be extracted into a different temporary directory.

Set the `LARAVEL_STORAGE_PATH` environment variable (for example, in your `.env` file) or call the `Illuminate\Foundation\Application::useStoragePath()` method to use a directory outside the temporary directory.

### Mercure Support

[Mercure](https://mercure.rocks) is a great way to add real-time capabilities to your Laravel apps.
FrankenPHP includes [Mercure support out of the box](mercure.md).

If you are not using [Octane](#laravel-octane), see [the Mercure documentation entry](mercure.md).

If you are using Octane, you can use enable Mercure support by adding the following lines to your `config/octane.php` file:

```php
// ...

return [
    // ...

    'mercure' => [
        'anonymous' => true,
        'publisher_jwt' => '!ChangeThisMercureHubJWTSecretKey!',
        'subscriber_jwt' => '!ChangeThisMercureHubJWTSecretKey!',
    ],
];
```

You can use [all directives supported by Mercure](https://mercure.rocks/docs/hub/config#directives) in this array.

To publish and subscribe to updates, we recommend using the [Laravel Mercure Broadcaster](https://github.com/mvanduijker/laravel-mercure-broadcaster) library.
Alternatively, see [the Mercure documentation](mercure.md) to do it in pure PHP and JavaScript.

### Running Octane With Standalone Binaries

It's even possible to package Laravel Octane apps as standalone binaries!

To do so, [install Octane properly](#laravel-octane) and follow the steps described in [the previous section](#laravel-apps-as-standalone-binaries).

Then, to start FrankenPHP in worker mode through Octane, run:

```console
PATH="$PWD:$PATH" frankenphp php-cli artisan octane:frankenphp
```

> [!CAUTION]
>
> For the command to work, the standalone binary **must** be named `frankenphp`
> because Octane needs a program named `frankenphp` available in the path.
