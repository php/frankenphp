# Usando workers do FrankenPHP

Inicialize sua aplicação uma vez e mantenha-a na memória.
O FrankenPHP processará as requisições recebidas em poucos milissegundos.

## Iniciando worker scripts

### Docker

Defina o valor da variável de ambiente `FRANKENPHP_CONFIG` como
`worker /caminho/para/seu/worker/script.php`:

```console
docker run \
    -e FRANKENPHP_CONFIG="worker /app/caminho/para/seu/worker/script.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### Binário independente

Use a opção `--worker` do comando `php-server` para servir o conteúdo do
diretório atual usando um worker:

```console
frankenphp php-server --worker /caminho/para/seu/worker/script.php
```

Se a sua aplicação PHP estiver [embutida no binário](embed.md), você pode
adicionar um `Caddyfile` personalizado no diretório raiz da aplicação.
Ele será usado automaticamente.

Também é possível
[reiniciar o worker em caso de alterações em arquivos](config.md#monitorando-alteracoes-em-arquivos)
com a opção `--watch`.
O comando a seguir acionará uma reinicialização se qualquer arquivo terminado em
`.php` no diretório `/caminho/para/sua/aplicacao/` ou subdiretórios for
modificado:

```console
frankenphp php-server --worker /caminho/para/seu/worker/script.php --watch="/caminho/para/sua/aplicacao/**/*.php"
```

## Symfony Runtime

O modo worker do FrankenPHP é suportado pelo
[Componente Symfony Runtime](https://symfony.com/doc/current/components/runtime.html).
Para iniciar qualquer aplicação Symfony em um worker, instale o pacote
FrankenPHP do [PHP Runtime](https://github.com/php-runtime/runtime):

```console
composer require runtime/frankenphp-symfony
```

Inicie seu servidor de aplicações definindo a variável de ambiente `APP_RUNTIME`
para usar o Symfony Runtime do FrankenPHP:

```console
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php" \
    -e APP_RUNTIME=Runtime\\FrankenPhpSymfony\\Runtime \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

## Laravel Octane

Consulte [a documentação dedicada](laravel.md#laravel-octane).

## Aplicações personalizadas

O exemplo a seguir mostra como criar seu próprio worker script sem depender de
uma biblioteca de terceiros:

```php
<?php
// public/index.php

// Impede o encerramento do worker script quando uma conexão do cliente for
// interrompida
ignore_user_abort(true);

// Inicializa a aplicação
require __DIR__.'/vendor/autoload.php';

$myApp = new \App\Kernel();
$myApp->boot();

// Manipulador fora do loop para melhor desempenho (fazendo menos trabalho)
$handler = static function () use ($myApp) {
    // Chamado quando uma requisição é recebida,
    // superglobals, php://input e similares são redefinidos
    echo $myApp->handle($_GET, $_POST, $_COOKIE, $_FILES, $_SERVER);
};

$maxRequests = (int)($_SERVER['MAX_REQUESTS'] ?? 0);
for ($nbRequests = 0; !$maxRequests || $nbRequests < $maxRequests; ++$nbRequests) {
    $keepRunning = \frankenphp_handle_request($handler);

    // Faz algo depois de enviar a resposta HTTP
    $myApp->terminate();

    // Chama o coletor de lixo para reduzir as chances de ele ser acionado no
    // meio da geração de uma página
    gc_collect_cycles();

    if (!$keepRunning) break;
}

// Limpeza
$myApp->shutdown();
```

Em seguida, inicie sua aplicação e use a variável de ambiente
`FRANKENPHP_CONFIG` para configurar seu worker:

```console
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

Por padrão, são iniciados 2 workers por CPU.
Você também pode configurar o número de workers a serem iniciados:

```console
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php 42" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### Reiniciar o worker após um certo número de requisições

Como o PHP não foi originalmente projetado para processos de longa duração,
ainda existem muitas bibliotecas e códigos legados que vazam memória.
Uma solução alternativa para usar esse tipo de código no modo worker é reiniciar
o worker script após processar um certo número de requisições:

O trecho de código de worker anterior permite configurar um número máximo de
requisições a serem processadas, definindo uma variável de ambiente chamada
`MAX_REQUESTS`.

### Reiniciar os workers manualmente

Embora seja possível reiniciar os workers
[em alterações de arquivo](config.md#monitorando-alteracoes-em-arquivos), também
é possível reiniciar todos os workers graciosamente por meio da
[API de administração do Caddy](https://caddyserver.com/docs/api).
Se o administrador estiver habilitado no seu
[Caddyfile](config.md#configuracao-do-caddyfile), você pode executar ping no
endpoint de reinicialização com uma simples requisição POST como esta:

```console
curl -X POST http://localhost:2019/frankenphp/workers/restart
```

## Comportamento das superglobais

As
[superglobais do PHP](https://www.php.net/manual/pt_BR/language.variables.superglobals.php)
(`$_SERVER`, `$_ENV`, `$_GET`...) se comportam da seguinte maneira:

- antes da primeira chamada para `frankenphp_handle_request()`, as superglobais
  contêm valores vinculados ao próprio worker script.
- durante e após a chamada para `frankenphp_handle_request()`, as superglobais
  contêm valores gerados a partir da requisição HTTP processada.
  Cada chamada para `frankenphp_handle_request()` altera os valores das
  superglobais.

Para acessar as superglobais do worker script dentro do retorno de chamada, você
deve copiá-las e importar a cópia para o escopo do retorno de chamada:

```php
<?php
// Copia a superglobal $_SERVER do worker antes da primeira chamada para
// frankenphp_handle_request()
$workerServer = $_SERVER;

$handler = static function () use ($workerServer) {
    var_dump($_SERVER); // $_SERVER vinculada à requisição
    var_dump($workerServer); // $_SERVER do worker script
};

// ...
```
