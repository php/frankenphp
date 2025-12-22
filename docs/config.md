# Configuration

FrankenPHP, Caddy as well as the [Mercure](mercure.md) and [Vulcain](https://vulcain.rocks) modules can be configured using [the formats supported by Caddy](https://caddyserver.com/docs/getting-started#your-first-config).

The most common format is the `Caddyfile`, which is a simple, human-readable text format.
By default, FrankenPHP will look for a `Caddyfile` in the current directory.
You can specify a custom path with the `-c` or `--config` option.

A minimal `Caddyfile` to serve a PHP application is shown below:

```caddyfile
# The hostname to respond to
localhost

# Optionaly, the directory to serve files from, otherwise defaults to the current directory
#root public/
php_server
```

A more advanced `Caddyfile` enabling more features and providing convenient environment variables is provided [in the FrankenPHP repository](https://github.com/php/frankenphp/blob/main/caddy/frankenphp/Caddyfile),
and with Docker images.

PHP itself can be configured [using a `php.ini` file](https://www.php.net/manual/en/configuration.file.php).

Depending on your installation method, FrankenPHP and the PHP interpreter will look for configuration files in locations described below.

## Docker

FrankenPHP:

- `/etc/frankenphp/Caddyfile`: the main configuration file
- `/etc/frankenphp/Caddyfile.d/*.caddyfile`: additional configuration files that are loaded automatically

PHP:

- `php.ini`: `/usr/local/etc/php/php.ini` (no `php.ini` is provided by default)
- additional configuration files: `/usr/local/etc/php/conf.d/*.ini`
- PHP extensions: `/usr/local/lib/php/extensions/no-debug-zts-<YYYYMMDD>/`
- You should copy an official template provided by the PHP project:

```dockerfile
FROM dunglas/frankenphp

# Production:
RUN cp $PHP_INI_DIR/php.ini-production $PHP_INI_DIR/php.ini

# Or development:
RUN cp $PHP_INI_DIR/php.ini-development $PHP_INI_DIR/php.ini
```

## RPM and Debian packages

FrankenPHP:

- `/etc/frankenphp/Caddyfile`: the main configuration file
- `/etc/frankenphp/Caddyfile.d/*.caddyfile`: additional configuration files that are loaded automatically

PHP:

- `php.ini`: `/etc/php-zts/php.ini` (a `php.ini` file with production presets is provided by default)
- additional configuration files: `/etc/php-zts/conf.d/*.ini`

## Static binary

FrankenPHP:

- In the current working directory: `Caddyfile`

PHP:

- `php.ini`: The directory in which `frankenphp run` or `frankenphp php-server` is executed, then `/etc/frankenphp/php.ini`
- additional configuration files: `/etc/frankenphp/php.d/*.ini`
- PHP extensions: cannot be loaded, bundle them in the binary itself
- copy one of `php.ini-production` or `php.ini-development` provided [in the PHP sources](https://github.com/php/php-src/).

## Caddyfile Config

The `php_server` or the `php` [HTTP directives](https://caddyserver.com/docs/caddyfile/concepts#directives) may be used within the site blocks to serve your PHP app.

Minimal example:

```caddyfile
localhost {
	# Enable compression (optional)
	encode zstd br gzip
	# Execute PHP files in the current directory and serve assets
	php_server
}
```

You can also explicitly configure FrankenPHP using the [global option](https://caddyserver.com/docs/caddyfile/concepts#global-options) `frankenphp`:

```caddyfile
{
	frankenphp {
		num_threads <num_threads> # Sets the number of PHP threads to start. Default: 2x the number of available CPUs.
		max_threads <num_threads> # Limits the number of additional PHP threads that can be started at runtime. Default: num_threads. Can be set to 'auto'.
		max_wait_time <duration> # Sets the maximum time a request may wait for a free PHP thread before timing out. Default: disabled.
		php_ini <key> <value> # Set a php.ini directive. Can be used several times to set multiple directives.
		worker {
			file <path> # Sets the path to the worker script.
			num <num> # Sets the number of PHP threads to start, defaults to 2x the number of available CPUs.
			env <key> <value> # Sets an extra environment variable to the given value. Can be specified more than once for multiple environment variables.
			watch <path> # Sets the path to watch for file changes. Can be specified more than once for multiple paths.
			name <name> # Sets the name of the worker, used in logs and metrics. Default: absolute path of worker file
			max_consecutive_failures <num> # Sets the maximum number of consecutive failures before the worker is considered unhealthy, -1 means the worker will always restart. Default: 6.
		}
	}
}

# ...
```

Alternatively, you may use the one-line short form of the `worker` option:

```caddyfile
{
	frankenphp {
		worker <file> <num>
	}
}

# ...
```

You can also define multiple workers if you serve multiple apps on the same server:

```caddyfile
app.example.com {
    root /path/to/app/public
	php_server {
		root /path/to/app/public # allows for better caching
		worker index.php <num>
	}
}

other.example.com {
    root /path/to/other/public
	php_server {
		root /path/to/other/public
		worker index.php <num>
	}
}

# ...
```

Using the `php_server` directive is generally what you need,
but if you need full control, you can use the lower-level `php` directive.
The `php` directive passes all input to PHP, instead of first checking whether
it's a PHP file or not. Read more about it in the [performance page](performance.md#try_files).

Using the `php_server` directive is equivalent to this configuration:

```caddyfile
route {
	# Add trailing slash for directory requests
	@canonicalPath {
		file {path}/index.php
		not path */
	}
	redir @canonicalPath {path}/ 308
	# If the requested file does not exist, try index files
	@indexFiles file {
		try_files {path} {path}/index.php index.php
		split_path .php
	}
	rewrite @indexFiles {http.matchers.file.relative}
	# FrankenPHP!
	@phpFiles path *.php
	php @phpFiles
	file_server
}
```

The `php_server` and the `php` directives have the following options:

```caddyfile
php_server [<matcher>] {
	root <directory> # Sets the root folder to the site. Default: `root` directive.
	split_path <delim...> # Sets the substrings for splitting the URI into two parts. The first matching substring will be used to split the "path info" from the path. The first piece is suffixed with the matching substring and will be assumed as the actual resource (CGI script) name. The second piece will be set to PATH_INFO for the script to use. Default: `.php`
	resolve_root_symlink false # Disables resolving the `root` directory to its actual value by evaluating a symbolic link, if one exists (enabled by default).
	env <key> <value> # Sets an extra environment variable to the given value. Can be specified more than once for multiple environment variables.
	file_server off # Disables the built-in file_server directive.
	worker { # Creates a worker specific to this server. Can be specified more than once for multiple workers.
		file <path> # Sets the path to the worker script, can be relative to the php_server root
		num <num> # Sets the number of PHP threads to start, defaults to 2x the number of available
		name <name> # Sets the name for the worker, used in logs and metrics. Default: absolute path of worker file. Always starts with m# when defined in a php_server block.
		watch <path> # Sets the path to watch for file changes. Can be specified more than once for multiple paths.
		env <key> <value> # Sets an extra environment variable to the given value. Can be specified more than once for multiple environment variables. Environment variables for this worker are also inherited from the php_server parent, but can be overwritten here.
		match <path> # match the worker to a path pattern. Overrides try_files and can only be used in the php_server directive.
	}
	worker <other_file> <num> # Can also use the short form like in the global frankenphp block.
}
```

### Watching for File Changes

Since workers only boot your application once and keep it in memory, any changes
to your PHP files will not be reflected immediately.

Workers can instead be restarted on file changes via the `watch` directive.
This is useful for development environments.

```caddyfile
{
	frankenphp {
		worker {
			file  /path/to/app/public/worker.php
			watch
		}
	}
}
```

This feature is often used in combination with [hot reload](hot-reload.md)

If the `watch` directory is not specified, it will fall back to `./**/*.{env,php,twig,yaml,yml}`,
which watches all `.php`, `.yaml`, `.yml`, `.twig` and `.env` files in the directory and subdirectories
where the FrankenPHP process was started. You can instead also specify one or more directories via a
[shell filename pattern](https://pkg.go.dev/path/filepath#Match):

```caddyfile
{
	frankenphp {
		worker {
			file  /path/to/app/public/worker.php
			watch /path/to/app # watches all files in all subdirectories of /path/to/app
			watch /path/to/app/*.php # watches files ending in .php in /path/to/app
			watch /path/to/app/**/*.php # watches PHP files in /path/to/app and subdirectories
			watch /path/to/app/**/*.{php,twig} # watches PHP and Twig files in /path/to/app and subdirectories
		}
	}
}
```

- The `**` pattern signifies recursive watching
- Directories can also be relative (to where the FrankenPHP process is started from)
- If you have multiple workers defined, all of them will be restarted when a file changes
- Be wary about watching files that are created at runtime (like logs) since they might cause unwanted worker restarts.

The file watcher is based on [e-dant/watcher](https://github.com/e-dant/watcher).

## Matching the Worker To a Path

In traditional PHP applications, scripts are always placed in the public directory.
This is also true for worker scripts, which are treated like any other PHP script.
If you want to instead put the worker script outside the public directory, you can do so via the `match` directive.

The `match` directive is an optimized alternative to `try_files` only available inside `php_server` and `php`.
The following example will always serve a file in the public directory if present
and otherwise forward the request to the worker matching the path pattern.

```caddyfile
{
	frankenphp {
		php_server {
			worker {
				file /path/to/worker.php # file can be outside of public path
				match /api/* # all requests starting with /api/ will be handled by this worker
			}
		}
	}
}
```

## Environment Variables

The following environment variables can be used to inject Caddy directives in the `Caddyfile` without modifying it:

- `SERVER_NAME`: change [the addresses on which to listen](https://caddyserver.com/docs/caddyfile/concepts#addresses), the provided hostnames will also be used for the generated TLS certificate
- `SERVER_ROOT`: change the root directory of the site, defaults to `public/`
- `CADDY_GLOBAL_OPTIONS`: inject [global options](https://caddyserver.com/docs/caddyfile/options)
- `FRANKENPHP_CONFIG`: inject config under the `frankenphp` directive

As for FPM and CLI SAPIs, environment variables are exposed by default in the `$_SERVER` superglobal.

The `S` value of [the `variables_order` PHP directive](https://www.php.net/manual/en/ini.core.php#ini.variables-order) is always equivalent to `ES` regardless of the placement of `E` elsewhere in this directive.

## PHP config

To load [additional PHP configuration files](https://www.php.net/manual/en/configuration.file.php#configuration.file.scan),
the `PHP_INI_SCAN_DIR` environment variable can be used.
When set, PHP will load all the file with the `.ini` extension present in the given directories.

You can also change the PHP configuration using the `php_ini` directive in the `Caddyfile`:

```caddyfile
{
    frankenphp {
        php_ini memory_limit 256M

        # or

        php_ini {
            memory_limit 256M
            max_execution_time 15
        }
    }
}
```

### Disabling HTTPS

By default, FrankenPHP will automatically enable HTTPS using for all the hostnames, including `localhost`.
If you want to disable HTTPS (for example in a development environment), you can set the `SERVER_NAME` environment variable to `http://` or `:80`:

Alternatively, you can use all other methods described in the [Caddy documentation](https://caddyserver.com/docs/automatic-https#activation).

If you want to use HTTPS with the `127.0.0.1` IP address instead of the `localhost` hostname, please read the [known issues](known-issues.md#using-https127001-with-docker) section.

### Full Duplex (HTTP/1)

When using HTTP/1.x, it may be desirable to enable full-duplex mode to allow writing a response before the entire body
has been read. (for example: [Mercure](mercure.md), WebSocket, Server-Sent Events, etc.)

This is an opt-in configuration that needs to be added to the global options in the `Caddyfile`:

```caddyfile
{
  servers {
    enable_full_duplex
  }
}
```

> [!CAUTION]
>
> Enabling this option may cause old HTTP/1.x clients that don't support full-duplex to deadlock.
> This can also be configured using the `CADDY_GLOBAL_OPTIONS` environment config:

```sh
CADDY_GLOBAL_OPTIONS="servers {
  enable_full_duplex
}"
```

You can find more information about this setting in the [Caddy documentation](https://caddyserver.com/docs/caddyfile/options#enable-full-duplex).

## Enable the Debug Mode

When using the Docker image, set the `CADDY_GLOBAL_OPTIONS` environment variable to `debug` to enable the debug mode:

```console
docker run -v $PWD:/app/public \
    -e CADDY_GLOBAL_OPTIONS=debug \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```
