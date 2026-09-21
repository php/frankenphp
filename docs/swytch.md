# Swytch

FrankenPHP includes the [Swytch Caddy module](https://github.com/swytchdb/caddy-swytch),
which runs a Redis-compatible server in the same process as your PHP application.
Enable it with the `swytch` option in the **global block** of your `Caddyfile`:

```caddyfile
{
    frankenphp
    swytch {
        listen 127.0.0.1:6379
        password {env.REDIS_PASSWORD}
        max_memory 80%
    }
}

localhost {
    root public/
    php_server
}
```

Set `REDIS_PASSWORD` before starting FrankenPHP. The bundled Caddyfile also
contains a commented Swytch configuration, including `SWYTCH_EXTRA_DIRECTIVES`
for additional options. Swytch starts only when configured.

> [!TIP]
>
> Because Swytch runs in the same process as your PHP application, we recommend
> setting `max_memory` to a percentage, such as `80%`, instead of a fixed size.
> This targets total process memory at approximately 80% of the memory available
> to the process (the container memory limit when set, otherwise system RAM on Linux).
> Swytch adjusts its cache capacity as your application's memory use changes:
> it shrinks under memory pressure and grows when there is room. This lets you
> use the spare memory already provisioned for your application's peak usage.

Connect from PHP using a Redis client such as [phpredis](https://github.com/phpredis/phpredis)
(requires the `redis` PHP extension) or [Predis](https://github.com/predis/predis):

```php
<?php
$redis = new Redis();
$redis->connect('127.0.0.1', 6379);
$redis->auth(getenv('REDIS_PASSWORD'));
$redis->set('greeting', 'Hello from FrankenPHP!');
echo $redis->get('greeting');
```

Standalone data is an in-memory cache and is discarded when the server stops.
For peer replication, configure `cluster_passphrase` and DNS discovery with
`join`. For durable storage across restarts and node loss with zero-knowledge
encryption, configure Swytch Cloud using `connection_secret` instead.

See the [module documentation](https://github.com/swytchdb/caddy-swytch#caddyfile)
for ACLs, Unix sockets, TLS, clustering, Cloud, and supported options. Unchanged
Caddy reloads retain the running server; configuration changes require a restart.

## Custom builds

Swytch is included in the bundled binary and default static builds. Include
`--with github.com/swytchdb/caddy-swytch` when using xcaddy or overriding
`XCADDY_ARGS`.

Swytch is licensed under AGPL-3.0-or-later.
