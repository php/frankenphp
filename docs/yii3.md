---
title: Running Yii 3 with FrankenPHP (Docker, worker mode)
description: How to run a Yii 3 application with FrankenPHP using the Docker image, a local install, or worker mode with the yii-runner-frankenphp package.
---

# Yii 3

## Running Yii 3 with the FrankenPHP Docker image

Serving a [Yii](https://www.yiiframework.com/) web application with FrankenPHP is as easy as mounting the project in the `/app` directory of the official Docker image.

Run this command from the main directory of your Yii app:

```console
docker run -p 80:80 -p 443:443 -p 443:443/udp -v $PWD:/app dunglas/frankenphp
```

And enjoy!

## Installing Yii 3 with FrankenPHP locally

Alternatively, you can run your Yii projects with FrankenPHP from your local machine:

1. [Download the binary corresponding to your system](../#standalone-binary)
2. Add the following configuration to a file named `Caddyfile` in the root directory of your Yii project:

   ```caddyfile
   {
   	frankenphp
   }

   # The domain name of your server
   localhost {
   	# Enable compression (optional)
   	encode zstd br gzip
   	# Execute PHP files from the public/ directory and serve assets
   	php_server {
   		root public/
   		try_files {path} index.php
   	}
   }
   ```

3. Start FrankenPHP from the root directory of your Yii project: `frankenphp run`

## Yii 3 worker mode

To run your Yii application in [worker mode](worker.md), install the [Yii FrankenPHP runner](https://github.com/yiisoft/yii-runner-frankenphp):

```console
composer require yiisoft/yii-runner-frankenphp
```

Then create a file named `worker.php` in the root directory of your application:

```php
<?php

declare(strict_types=1);

use Yiisoft\Yii\Runner\FrankenPHP\FrankenPHPApplicationRunner;

require_once __DIR__ . '/vendor/autoload.php';

(new FrankenPHPApplicationRunner(rootPath: __DIR__))->run();
```

Update your `Caddyfile` to start the application in worker mode:

```caddyfile
{
	frankenphp
}

localhost {
	encode zstd br gzip
	php_server {
		root public/
		worker ./worker.php {
			# Send all requests to the worker
			match *
			# Reload workers when PHP files change (development only)
			watch ./**/*.php
		}
	}
}
```

If your application is based on the [official Yii app template](https://github.com/yiisoft/app), see the [package readme](https://github.com/yiisoft/yii-runner-frankenphp) for a complete `worker.php` example with debug, environment, and error handler configuration.

To limit the number of requests a worker handles before being restarted (useful to mitigate memory leaks), set the `MAX_REQUESTS` environment variable. By default, workers handle requests indefinitely.

When using worker mode, make sure stateful services are reset after each request. See the [worker mode documentation](worker.md) and the [Yii DI `StateResetter` documentation](https://github.com/yiisoft/di#resetting-services-state) for details.
