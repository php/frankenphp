---
title: Migrando do Nginx e PHP-FPM para o FrankenPHP
description: Guia passo a passo para migrar uma aplicação PHP de uma pilha Nginx mais PHP-FPM para o FrankenPHP, cobrindo Caddyfile, php.ini, threads e Docker.
---

# Migrando do Nginx/PHP-FPM

FrankenPHP substitui seu servidor web (Nginx, Apache) e o PHP-FPM por um único binário.
Este guia abrange uma migração básica para uma aplicação PHP típica.

## Principais diferenças

| Configuração PHP-FPM              | Equivalente FrankenPHP                                   |
| --------------------------------- | -------------------------------------------------------- |
| Nginx/Apache + PHP-FPM            | Único binário `frankenphp`                               |
| Configurações de pool do `php-fpm.conf`      | [`frankenphp` opção global](config.md#caddyfile-config) |
| Bloco `server {}` do Nginx           | Bloco de site do `Caddyfile`                                   |
| `php_value` / `php_admin_value`   | [`php_ini` diretiva do Caddyfile](config.md#php-config)    |
| `pm = static` / `pm.max_children` | `num_threads`                                            |
| `pm = dynamic`                    | [`max_threads auto`](performance.md#max_threads)         |
| Filtragem de requisições do servidor web      | Rotas e matchers do Caddy                                |

## Filtragem de requisições HTTP

Ao migrar de um pacote Nginx ou Apache padrão, verifique qualquer filtragem de requisições que foi feita pelo servidor web antes que o PHP-FPM recebesse a requisição.
FrankenPHP é construído sobre o Caddy, então métodos e cabeçalhos HTTP válidos são passados através do roteamento normal do Caddy, a menos que seu `Caddyfile` ou outro proxy os rejeite primeiro.

Por exemplo, alguns servidores web rejeitam requisições `TRACE` por padrão.
Se sua aplicação deve manter esse comportamento, adicione um matcher explícito antes de `php_server`:

```caddyfile
example.com {
    @trace method TRACE
    respond @trace 405

    root /var/www/app/public
    php_server
}
```

Aplique a mesma abordagem a qualquer política mais rigorosa de método ou cabeçalho que seu servidor web anterior impôs.
O mesmo se aplica aos nomes dos cabeçalhos de requisição: alguns servidores web ou proxies rejeitam ou descartam nomes de campos HTTP válidos, mas incomuns, antes que o PHP-FPM os veja, enquanto o FrankenPHP pode passá-los para a aplicação PHP.

## Passo 1: substitua sua configuração de servidor web

Uma configuração típica de Nginx + PHP-FPM:

```nginx
# /etc/nginx/sites-available/example.com
server {
    listen 80;
    server_name example.com;
    root /var/www/app/public;
    index index.php;

    location / {
        try_files $uri $uri/ /index.php$is_args$args;
    }

    location ~ \.php$ {
        fastcgi_pass unix:/run/php/php-fpm.sock;
        fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
        include fastcgi_params;
    }
}
```

Torna-se um único `Caddyfile`:

```caddyfile
example.com {
    root /var/www/app/public
    php_server
}
```

É isso. A diretiva `php_server` lida com o roteamento PHP, comportamento semelhante ao `try_files` e o serviço de arquivos estáticos.

## Passo 2: migre a configuração do PHP

Seu `php.ini` existente funciona como está. Veja [Configuração](config.md) para saber onde colocá-lo, dependendo do seu método de instalação.

Uma configuração muda de significado: `max_execution_time` conta o tempo de relógio (wall time) no FrankenPHP, enquanto em builds não-ZTS do PHP-FPM no Linux ele conta o tempo de CPU.
Esperar por uma consulta de banco de dados, uma chamada HTTP, `sleep()` ou o bloqueio de um arquivo de sessão agora conta para o limite, então um valor ajustado para PHP-FPM pode ser muito baixo.

Você também pode definir diretivas diretamente no `Caddyfile`:

```caddyfile
{
    frankenphp {
        php_ini memory_limit 256M
        php_ini max_execution_time 30
    }
}

example.com {
    root /var/www/app/public
    php_server
}
```

## Passo 3: ajuste o tamanho do pool

No PHP-FPM, você ajusta `pm.max_children` para controlar o número de processos worker.
No FrankenPHP, o equivalente é `num_threads`:

```caddyfile
{
    frankenphp {
        num_threads 16
    }
}
```

Por padrão, o FrankenPHP inicia 2 threads por CPU. Para escalabilidade dinâmica semelhante ao `pm = dynamic` do PHP-FPM:

```caddyfile
{
    frankenphp {
        num_threads 4
        max_threads auto
    }
}
```

## Passo 4: Migração Docker

Uma configuração típica de PHP-FPM com Docker usando dois contêineres (Nginx + PHP-FPM) pode ser substituída por um único contêiner:

**Antes:**

```yaml
# compose.yaml
services:
  nginx:
    image: nginx:1
    volumes:
      # mount Nginx config into the container
      - ./config:/etc/nginx/conf.d
      - .:/var/www/app
    ports:
      - "80:80"
      - "443:443"

  php:
    image: php:8.5-fpm
    volumes:
      - .:/var/www/app
```

**Depois:**

```yaml
# compose.yaml
services:
  php:
    image: dunglas/frankenphp:1-php8.5
    volumes:
      - .:/var/www/app
      # mount the Caddyfile into the container
      - ./config:/etc/frankenphp
      - caddy_data:/data
      - caddy_config:/config
    ports:
      - "80:80"
      - "443:443"
      - "443:443/udp"

volumes:
  caddy_data:
  caddy_config:
```

Se precisar de extensões PHP adicionais, consulte [Construindo imagem Docker personalizada](docker.md#how-to-install-more-php-extensions).

Para configurações Docker específicas de frameworks, consulte [Symfony Docker](https://github.com/dunglas/symfony-docker) e [executando Laravel com a imagem Docker do FrankenPHP](laravel.md#running-laravel-with-the-frankenphp-docker-image).

## Passo 5: considere o modo worker (opcional)

No [modo clássico](classic.md), o FrankenPHP funciona como o PHP-FPM: cada requisição inicializa a aplicação do zero.
Este é um ponto de partida seguro para a migração.

Para melhor desempenho, você pode mudar para o [modo worker](worker.md), que inicializa sua aplicação uma vez e a mantém em memória:

```caddyfile
example.com {
    root /var/www/app/public
    php_server {
        root /var/www/app/public
        worker index.php 4
    }
}
```

> [!CAUTION]
>
> O modo worker mantém sua aplicação em memória entre as requisições. Certifique-se de que seu código não depende do estado global sendo redefinido entre as requisições. Frameworks como [Symfony](worker.md#worker-mode-for-symfony), [Laravel](laravel.md#laravel-octane) e [API Platform](https://api-platform.com) possuem suporte nativo para este modo.

## O que você pode remover

Após a migração, você não precisará mais de:

- Nginx ou Apache
- PHP-FPM (serviço/processo `php-fpm`)
- Configuração FastCGI
- Certificados TLS auto-gerenciados (o Caddy os gerencia automaticamente)
