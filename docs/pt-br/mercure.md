# Tempo real

O FrankenPHP vem com um hub [Mercure](https://mercure.rocks) integrado!
O Mercure permite que você envie eventos em tempo real para todos os dispositivos conectados: eles receberão um evento JavaScript instantaneamente.

É uma alternativa conveniente aos WebSockets, simples de usar e com suporte nativo em todos os navegadores web modernos!

![Mercure](mercure-hub.png)

## Habilitando o Mercure

O suporte ao Mercure é desativado por padrão.
Aqui está um exemplo mínimo de um `Caddyfile` habilitando tanto o FrankenPHP quanto o hub Mercure:

```caddyfile
# O nome do host para responder
localhost

mercure {
    # A chave secreta usada para assinar os tokens JWT para publicadores
    publisher_jwt !ChangeThisMercureHubJWTSecretKey!
    # Permite assinantes anônimos (sem JWT)
    anonymous
}

root public/
php_server
```

> [!TIP]
>
> O [exemplo de `Caddyfile`](https://github.com/php/frankenphp/blob/main/caddy/frankenphp/Caddyfile)
> fornecido pelas [imagens Docker](docker.md) já inclui uma configuração Mercure comentada
> com variáveis de ambiente convenientes para configurá-lo.
>
> Descomente a seção Mercure em `/etc/frankenphp/Caddyfile` para habilitá-lo.

## Assinando Atualizações

Por padrão, o hub Mercure está disponível no caminho `/.well-known/mercure` do seu servidor FrankenPHP.
Para assinar atualizações, use a classe JavaScript nativa [`EventSource`](https://developer.mozilla.org/docs/Web/API/EventSource):

```html
<!-- public/index.html -->
<!doctype html>
<title>Exemplo Mercure</title>
<script>
  const eventSource = new EventSource("/.well-known/mercure?topic=my-topic");
  eventSource.onmessage = function (event) {
    console.log("Nova mensagem:", event.data);
  };
</script>
```

## Publicando Atualizações

### Usando `mercure_publish()`

FrankenPHP fornece uma função conveniente `mercure_publish()` para publicar atualizações no hub Mercure integrado:

```php
<?php
// public/publish.php

$updateID = mercure_publish('my-topic',  json_encode(['key' => 'value']));

// Escreve nos logs do FrankenPHP
error_log("update $updateID published", 4);
```

A assinatura completa da função é:

```php
/**
 * @param string|string[] $topics
 */
function mercure_publish(string|array $topics, string $data = '', bool $private = false, ?string $id = null, ?string $type = null, ?int $retry = null): string {}
```

### Usando `file_get_contents()`

Para despachar uma atualização para os assinantes conectados, envie uma requisição POST autenticada para o hub Mercure com os parâmetros `topic` e `data`:

```php
<?php
// public/publish.php

const JWT = 'eyJhbGciOiJIUzI1NiJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiKiJdfX0.PXwpfIGng6KObfZlcOXvcnWCJOWTFLtswGI5DZuWSK4';

$updateID = file_get_contents('https://localhost/.well-known/mercure', context: stream_context_create(['http' => [
    'method'  => 'POST',
    'header'  => "Content-type: application/x-www-form-urlencoded\r\nAuthorization: Bearer " . JWT,
    'content' => http_build_query([
        'topic' => 'my-topic',
        'data' => json_encode(['key' => 'value']),
    ]),
]]));

// Escreve nos logs do FrankenPHP
error_log("update $updateID published", 4);
```

A chave passada como parâmetro da opção `mercure.publisher_jwt` no `Caddyfile` deve ser usada para assinar o token JWT usado no cabeçalho `Authorization`.

O JWT deve incluir uma reivindicação `mercure` com permissão `publish` para os tópicos nos quais você deseja publicar.
Consulte [a documentação do Mercure](https://mercure.rocks/spec#publishers) sobre autorização.

Para gerar seus próprios tokens, você pode usar [este link do jwt.io](https://www.jwt.io/#token=eyJhbGciOiJIUzI1NiJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiKiJdfX0.PXwpfIGng6KObfZlcOXvcnWCJOWTFLtswGI5DZuWSK4),
mas para aplicações de produção, é recomendado usar tokens de curta duração gerados dinamicamente com uma [biblioteca JWT](https://www.jwt.io/libraries?programming_language=php) confiável.

### Usando o Symfony Mercure

Alternativamente, você pode usar o [Componente Symfony Mercure](https://symfony.com/components/Mercure), uma biblioteca PHP autônoma.

Esta biblioteca lida com a geração de JWT, publicação de atualizações, bem como autorização baseada em cookies para assinantes.

Primeiro, instale a biblioteca usando o Composer:

```console
composer require symfony/mercure lcobucci/jwt
```

Então, você pode usá-lo assim:

```php
<?php
// public/publish.php

require __DIR__ . '/../vendor/autoload.php';

const JWT_SECRET = '!ChangeThisMercureHubJWTSecretKey!'; // Deve ser a mesma de mercure.publisher_jwt no Caddyfile

// Configura o provedor de token JWT
$jwFactory = new \Symfony\Component\Mercure\Jwt\LcobucciFactory(JWT_SECRET);
$provider = new \Symfony\Component\Mercure\Jwt\FactoryTokenProvider($jwFactory, publish: ['*']);

$hub = new \Symfony\Component\Mercure\Hub('https://localhost/.well-known/mercure', $provider);
// Serializa a atualização e a despacha para o hub, que a transmitirá para os clientes
$updateID = $hub->publish(new \Symfony\Component\Mercure\Update('my-topic', json_encode(['key' => 'value'])));

// Escreve nos logs do FrankenPHP
error_log("update $updateID published", 4);
```

O Mercure também é nativamente suportado por:

- [Laravel](laravel.md#mercure-support)
- [Symfony](https://symfony.com/doc/current/mercure.html)
- [API Platform](https://api-platform.com/docs/core/mercure/)
