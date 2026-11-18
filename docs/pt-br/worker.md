---
title: Modo worker do FrankenPHP: mantenha sua aplicação PHP em memória
description: Execute o FrankenPHP no modo worker para manter sua aplicação PHP inicializada entre requisições, reduzir a sobrecarga de inicialização e servir respostas em milissegundos.
---

# Usando workers do FrankenPHP

Inicialize sua aplicação uma vez e mantenha-a na memória.
O FrankenPHP processará as requisições recebidas em poucos milissegundos.

## Iniciando scripts de worker do FrankenPHP

### Executando um worker do FrankenPHP com Docker

Defina o valor da variável de ambiente `FRANKENPHP_CONFIG` como `worker /path/to/your/worker/script.php`:

```bash
docker run \
    -e FRANKENPHP_CONFIG="worker /app/path/to/your/worker/script.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### Executando um worker do FrankenPHP com o binário autônomo

Use a opção `--worker` do comando `php-server` para servir o conteúdo do diretório atual usando um worker:

```bash
frankenphp php-server --worker /path/to/your/worker/script.php
```

Se a sua aplicação PHP estiver [embutida no binário](embed.md), você pode adicionar um `Caddyfile` personalizado no diretório raiz da aplicação.
Ele será usado automaticamente.

Também é possível [reiniciar o worker em alterações de arquivo](config.md#watching-for-file-changes) com a opção `--watch`.
O comando a seguir acionará uma reinicialização se qualquer arquivo terminado em `.php` no diretório `/path/to/your/app/` ou subdiretórios for modificado:

```bash
frankenphp php-server --worker /path/to/your/worker/script.php --watch="/path/to/your/app/**/*.php"
```

Este recurso é frequentemente usado em combinação com [hot reloading](hot-reload.md).

## Modo worker para Symfony

Consulte [a documentação do modo worker do Symfony com FrankenPHP](symfony.md#symfony-worker-mode-com-frankenphp).

## Modo worker para Laravel Octane

Consulte [a documentação do Laravel Octane para FrankenPHP](laravel.md#laravel-octane).

## Escrevendo um script de worker FrankenPHP personalizado

O exemplo a seguir mostra como criar seu próprio script de worker sem depender de uma biblioteca de terceiros:

```php
<?php
// public/index.php

// Inicialize sua aplicação
require __DIR__.'/vendor/autoload.php';

$myApp = new \App\Kernel();
$myApp->boot();

// Manipulador fora do loop para melhor desempenho (fazendo menos trabalho)
$handler = static function () use ($myApp) {
    try {
        // Chamado quando uma requisição é recebida,
        // superglobais, php://input e similares são redefinidos
        echo $myApp->handle($_GET, $_POST, $_COOKIE, $_FILES, $_SERVER);
    } catch (\Throwable $exception) {
        // `set_exception_handler` é chamado apenas quando o script do worker termina,
        // o que pode não ser o que você espera, então capture e trate as exceções aqui
        (new \MyCustomExceptionHandler)->handleException($exception);
    }
};

$maxRequests = (int)($_SERVER['MAX_REQUESTS'] ?? 0);
for ($nbRequests = 0; !$maxRequests || $nbRequests < $maxRequests; ++$nbRequests) {
    $keepRunning = \frankenphp_handle_request($handler);

    // Faça algo depois de enviar a resposta HTTP
    $myApp->terminate();

    // Chame o coletor de lixo para reduzir as chances de ele ser acionado no meio de uma geração de página
    gc_collect_cycles();

    if (!$keepRunning) break;
}

// Limpeza
$myApp->shutdown();
```

Em seguida, inicie sua aplicação e use a variável de ambiente `FRANKENPHP_CONFIG` para configurar seu worker:

```bash
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

Por padrão, são iniciados 2 workers por CPU.
Você também pode configurar o número de workers a serem iniciados:

```bash
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php 42" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### Usando PSR-15

Se sua aplicação usa [PSR-15](https://www.php-fig.org/psr/psr-15/) em vez de superglobais, converta a requisição/resposta nas bordas do manipulador com [`nyholm/psr7`](https://github.com/Nyholm/psr7) e [`nyholm/psr7-server`](https://github.com/Nyholm/psr7-server):

```console
composer require nyholm/psr7 nyholm/psr7-server psr/http-server-handler
```

```php
<?php
// public/index.php

require __DIR__.'/vendor/autoload.php';

use Nyholm\Psr7\Factory\Psr17Factory;
use Nyholm\Psr7Server\ServerRequestCreator;

$myApp = new \App\Kernel(); // implementa Psr\Http\Server\RequestHandlerInterface
$myApp->boot();

$psr17Factory = new Psr17Factory();
$creator = new ServerRequestCreator(
    $psr17Factory, // ServerRequestFactory
    $psr17Factory, // UriFactory
    $psr17Factory, // UploadedFileFactory
    $psr17Factory, // StreamFactory
);

$handler = static function () use ($myApp, $creator) {
    $response = $myApp->handle($creator->fromGlobals());

    http_response_code($response->getStatusCode());
    foreach ($response->getHeaders() as $name => $values) {
        foreach ($values as $value) {
            header("$name: $value", false);
        }
    }
    echo $response->getBody();
};

$maxRequests = (int)($_SERVER['MAX_REQUESTS'] ?? 0);
for ($nbRequests = 0; !$maxRequests || $nbRequests < $maxRequests; ++$nbRequests) {
    $keepRunning = \frankenphp_handle_request($handler);
    gc_collect_cycles();
    if (!$keepRunning) break;
}
```

### Reiniciar o worker após um certo número de requisições

Como o PHP não foi originalmente projetado para processos de longa duração, muitas bibliotecas e códigos legados ainda vazam memória.
Uma solução alternativa para usar esse tipo de código no modo worker é reiniciar o script de worker após processar um certo número de requisições:

O trecho de código de worker anterior permite configurar um número máximo de requisições a serem processadas, definindo uma variável de ambiente chamada `MAX_REQUESTS`.

### Reiniciar os workers manualmente

Embora seja possível reiniciar os workers [em alterações de arquivo](config.md#watching-for-file-changes), também é possível reiniciar todos os workers
graciosamente por meio da [API de administração do Caddy](https://caddyserver.com/docs/api). Se o administrador estiver habilitado no seu
[Caddyfile](config.md#caddyfile-config), você pode acionar o endpoint de reinicialização com uma simples requisição POST como esta:

```bash
curl -X POST http://localhost:2019/frankenphp/workers/restart
```

### Falhas de worker

Se um script de worker travar com um código de saída diferente de zero, o FrankenPHP o reiniciará com uma estratégia de backoff exponencial.
Se o script de worker permanecer ativo por mais tempo do que o último backoff × 2,
ele não irá penalizar o script de worker e o reiniciará novamente.
No entanto, se o script de worker continuar a falhar com um código de saída diferente de zero em um curto período de tempo
(por exemplo, com um erro de digitação em um script), o FrankenPHP travará com o erro: `too many consecutive failures`.

O número de falhas consecutivas pode ser configurado no seu [Caddyfile](config.md#caddyfile-config) com a opção `max_consecutive_failures`:

```caddyfile
frankenphp {
    worker {
        # ...
        max_consecutive_failures 10
    }
}
```

### Número de threads

Quando o modo worker está habilitado, um thread adicional é necessário para lidar com as requisições recebidas,
portanto, o valor padrão de `num_threads` é `1 + 2x(número de CPUs disponíveis)`.

Assim, nesse modo, se `num_threads` e `num` forem definidos explicitamente, `num_threads` deve ser estritamente maior que `num`.

## Comportamento das superglobais

As [superglobais do PHP](https://www.php.net/manual/language.variables.superglobals.php) (`$_SERVER`, `$_ENV`, `$_GET`...)
se comportam da seguinte maneira:

- antes da primeira chamada para `frankenphp_handle_request()`, as superglobais contêm valores vinculados ao próprio script de worker
- durante e após a chamada para `frankenphp_handle_request()`, as superglobais contêm valores gerados a partir da requisição HTTP processada, cada chamada para `frankenphp_handle_request()` altera os valores das superglobais

Para acessar as superglobais do script de worker dentro do retorno de chamada, você deve copiá-las e importar a cópia para o escopo do retorno de chamada:

```php
<?php
// Copie a superglobal $_SERVER do worker antes da primeira chamada para frankenphp_handle_request()
$workerServer = $_SERVER;

$handler = static function () use ($workerServer) {
    var_dump($_SERVER); // $_SERVER vinculado à requisição
    var_dump($workerServer); // $_SERVER do script do worker
};

// ...
```

A maioria das superglobais (`$_GET`, `$_POST`, `$_COOKIE`, `$_FILES`, `$_SERVER`, `$_REQUEST`) são automaticamente redefinidas entre as requisições.
No entanto, **`$_ENV` atualmente não é redefinida entre as requisições**.
Isso significa que quaisquer modificações feitas em `$_ENV` durante uma requisição persistirão e serão visíveis para requisições subsequentes manipuladas pelo mesmo thread de worker.
Evite armazenar dados específicos da requisição ou sensíveis em `$_ENV`.

## Persistência de estado

Como o modo worker mantém o processo PHP ativo entre as requisições, o FrankenPHP apenas redefine o estado vinculado à requisição HTTP sendo processada: superglobais (exceto `$_ENV`), o corpo da requisição, cabeçalhos e saída da resposta, e a sessão.
Todo o resto é compartilhado entre as requisições manipuladas pelo mesmo thread, como em um script CLI de longa duração, incluindo:

- **Variáveis estáticas**: Variáveis declaradas com `static` dentro de funções ou métodos retêm seus valores entre as requisições.
- **Propriedades estáticas de classe**: Propriedades estáticas em classes persistem entre as requisições.
- **Variáveis globais**: Variáveis no escopo global do script de worker persistem entre as requisições.
- **Caches em memória**: Quaisquer dados armazenados em memória (arrays, objetos) fora do manipulador de requisições persistem.
- **Configurações do motor de tempo de execução**: Alterações feitas com `ini_set()`, `stream_context_set_default()`, `date_default_timezone_set()`, `chdir()`, `stream_wrapper_register()`, `set_error_handler()` ou `set_exception_handler()` permanecem em vigor para requisições posteriores. Nunca armazene valores específicos da requisição ou do usuário ali, como credenciais no contexto de fluxo padrão: passe um contexto explícito para cada chamada.
- **Estado do motor e das extensões**: `error_get_last()`, `libxml_get_errors()` e `libxml_get_last_error()` retornam os erros deixados por uma requisição anterior. `http_get_last_response_headers()` retorna os cabeçalhos da última chamada HTTP feita no thread, mesmo por uma requisição anterior. `strtok()` continua a string que uma requisição anterior forneceu. Após `mt_srand()` ou `srand()` com uma semente, `mt_rand()`, `rand()`, `shuffle()`, `str_shuffle()` e `array_rand()` continuam essa sequência semeada. Os erros, cabeçalhos e strings podem conter dados de outro cliente, e uma sequência semeada torna os valores aleatórios previsíveis: redefina-os conforme mostrado abaixo.
- **Objetos em ciclos de referência**: O coletor de lixo os destrói quando o PHP o aciona, possivelmente enquanto uma requisição posterior é manipulada, então seus destrutores são executados nessa requisição e sua saída vai para o cliente dela. Chame `gc_collect_cycles()` após `frankenphp_handle_request()` retornar, como no exemplo acima.
- **Funções de desligamento**: Callbacks registrados com `register_shutdown_function()` são executados quando o script do worker para, não no final da requisição que os registrou, e permanecem na memória até então. Se uma requisição parar o script (`exit()`, um erro fatal, um timeout), eles são executados dentro dessa requisição e sua saída e cabeçalhos vão para o cliente dela. Registre-os uma vez, fora do manipulador, e execute o trabalho por requisição no final do manipulador ou após `frankenphp_handle_request()` retornar.
- **Configurações de todo o processo**: `umask()` e `setlocale()` alteram todo o processo FrankenPHP, não apenas o thread atual. O modo worker não os restaura entre as requisições, e em qualquer modo eles também afetam as requisições em execução ao mesmo tempo. Defina-os uma vez na inicialização, ou chame `chmod()` em arquivos criados em vez de alterar o umask.

Isso é por design e é o que torna o modo worker rápido. No entanto, exige atenção para evitar efeitos colaterais indesejados:

```php
<?php
function getCounter(): int {
    static $count = 0;
    return ++$count; // Incrementa entre requisições!
}

$handler = static function () {
    echo getCounter(); // 1, 2, 3, ... para cada requisição neste thread
};

while (\frankenphp_handle_request($handler)) {
    // ...
}
```

Ao escrever scripts de worker, certifique-se de redefinir qualquer estado específico da requisição entre as requisições.
O PHP fornece funções para redefinir o estado do motor e das extensões listado acima:

```php
<?php
$handler = static function () {
    // ...
};

while (\frankenphp_handle_request($handler)) {
    error_clear_last();
    libxml_clear_errors(); // requer a extensão libxml
    http_clear_last_response_headers(); // PHP 8.4+
    mt_srand(); // resemeia aleatoriamente, necessário apenas se a aplicação o semear
}
```

Passe a string para tokenizar na primeira chamada a `strtok()` de cada requisição, em vez de depender de uma chamada anterior.
Prefira uma instância de `Random\Randomizer` com seu próprio motor em vez de semear o gerador global.

Frameworks como [Symfony](symfony.md) e [Laravel Octane](laravel.md) cuidam de redefinir a maior parte do estado para você, mas você ainda pode precisar redefinir seus próprios serviços. Com o Symfony, os serviços que mantêm o estado específico da requisição devem implementar [`Symfony\Contracts\Service\ResetInterface`](https://github.com/symfony/contracts/blob/main/Service/ResetInterface.php) para que sejam redefinidos pelo kernel entre as requisições.
