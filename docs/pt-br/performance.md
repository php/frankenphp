# Desempenho

Por padrão, o FrankenPHP tenta oferecer um bom equilíbrio entre desempenho e
facilidade de uso.
No entanto, é possível melhorar substancialmente o desempenho usando uma
configuração apropriada.

## Número de threads e workers

Por padrão, o FrankenPHP inicia 2 vezes mais threads e workers (no modo worker)
do que a quantidade de CPU disponível.

Os valores apropriados dependem muito de como sua aplicação foi escrita, do que
ela faz e do seu hardware.
Recomendamos fortemente alterar esses valores.
Para melhor estabilidade do sistema, recomenda-se ter `num_threads` x
`memory_limit` < `available_memory`.

Para encontrar os valores corretos, é melhor executar testes de carga simulando
tráfego real.
[k6](https://k6.io) e [Gatling](https://gatling.io) são boas ferramentas para
isso.

Para configurar o número de threads, use a opção `num_threads` das diretivas
`php_server` e `php`.
Para alterar o número de workers, use a opção `num` da seção `worker` da
diretiva `frankenphp`.

### `max_threads`

Embora seja sempre melhor saber exatamente como será o seu tráfego, aplicações
reais tendem a ser mais imprevisíveis.
A [configuração](config.md#configuracao-do-caddyfile) `max_threads` permite que
o FrankenPHP crie threads adicionais automaticamente em tempo de execução até o
limite especificado.
`max_threads` pode ajudar você a descobrir quantas threads são necessárias para
lidar com seu tráfego e pode tornar o servidor mais resiliente a picos de
latência.
Se definido como `auto`, o limite será estimado com base no `memory_limit` em
seu `php.ini`.
Caso contrário, `auto` assumirá como padrão o valor 2x `num_threads`.
Lembre-se de que `auto` pode subestimar bastante o número de threads
necessárias.
`max_threads` é semelhante ao
[pm.max_children](https://www.php.net/manual/pt_BR/install.fpm.configuration.php#pm.max-children)
do PHP FPM.
A principal diferença é que o FrankenPHP usa threads em vez de processos e as
delega automaticamente entre diferentes worker scripts e o modo clássico,
conforme necessário.

## Modo worker

Habilitar [o modo worker](worker.md) melhora drasticamente o desempenho, mas sua
aplicação precisa ser adaptada para ser compatível com este modo: você precisa
criar um worker script e garantir que a aplicação não esteja com vazamento de
memória.

## Não use `musl`

A variante Alpine Linux das imagens oficiais do Docker e os binários padrão que
fornecemos usam [a biblioteca C `musl`](https://musl.libc.org).

O PHP é conhecido por ser
[mais lento](https://gitlab.alpinelinux.org/alpine/aports/-/issues/14381)
ao usar esta biblioteca C alternativa em vez da biblioteca GNU tradicional,
especialmente quando compilado no modo ZTS (thread-safe), necessário para o
FrankenPHP.
A diferença pode ser significativa em um ambiente com muitas threads.

Além disso,
[alguns bugs só acontecem ao usar `musl`](https://github.com/php/php-src/issues?q=sort%3Aupdated-desc+is%3Aissue+is%3Aopen+label%3ABug+musl).

Em ambientes de produção, recomendamos o uso do FrankenPHP vinculado à `glibc`.

Isso pode ser feito usando as imagens Docker do Debian (o padrão), baixando o
binário com sufixo -gnu de nossos
[Lançamentos](https://github.com/php/frankenphp/releases) ou
[compilando o FrankenPHP a partir do código-fonte](compile.md).

Como alternativa, fornecemos binários `musl` estáticos compilados com
[o alocador `mimalloc`](https://github.com/microsoft/mimalloc), o que alivia os
problemas em cenários com threads.

## Configuração do runtime do Go

O FrankenPHP é escrito em Go.

Em geral, o runtime do Go não requer nenhuma configuração especial, mas em
certas circunstâncias, configurações específicas melhoram o desempenho.

Você provavelmente deseja definir a variável de ambiente `GODEBUG` como
`cgocheck=0` (o padrão nas imagens Docker do FrankenPHP).

Se você executa o FrankenPHP em contêineres (Docker, Kubernetes, LXC...) e
limita a memória disponível para os contêineres, defina a variável de ambiente
`GOMEMLIMIT` para a quantidade de memória disponível.

Para mais detalhes,
[a página da documentação do Go dedicada a este assunto](https://pkg.go.dev/runtime#hdr-Environment_Variables)
é uma leitura obrigatória para aproveitar ao máximo o runtime.

## `file_server`

Por padrão, a diretiva `php_server` configura automaticamente um servidor de
arquivos para servir arquivos estáticos (assets) armazenados no diretório raiz.

Este recurso é conveniente, mas tem um custo.
Para desativá-lo, use a seguinte configuração:

```caddyfile
php_server {
    file_server off
}
```

## `try_files`

Além de arquivos estáticos e arquivos PHP, `php_server` também tentará servir o
arquivo index da sua aplicação e os arquivos index de diretório (`/path/` ->
`/path/index.php`).
Se você não precisa de arquivos index de diretório, pode desativá-los definindo
explicitamente `try_files` assim:

```caddyfile
php_server {
    try_files {path} index.php
    root /raiz/da/sua/aplicacao # adicionar explicitamente a raiz aqui permite um melhor armazenamento em cache
}
```

Isso pode reduzir significativamente o número de operações desnecessárias com
arquivos.

Uma abordagem alternativa com 0 operações desnecessárias no sistema de arquivos
seria usar a diretiva `php` e separar os arquivos estáticos do PHP usando
caminhos.
Essa abordagem funciona bem se toda a sua aplicação for servida por um arquivo
de entrada.
Um exemplo de [configuração](config.md#configuracao-do-caddyfile) que serve
arquivos estáticos a partir de uma pasta `/assets` poderia ser assim:

```caddyfile
route {
    @assets {
        path /assets/*
    }

    # tudo a partir de /assets é gerenciado pelo servidor de arquivos
    file_server @assets {
        root /raiz/da/sua/aplicacao
    }

    # tudo o que não está em /assets é gerenciado pelo seu arquivo index ou worker PHP
    rewrite index.php
    php {
        root /raiz/da/sua/aplicacao # adicionar explicitamente a raiz aqui permite um melhor armazenamento em cache
    }
}
```

## Placeholders

Você pode usar
[placeholders](https://caddyserver.com/docs/conventions#placeholders) nas
diretivas `root` e `env`.
No entanto, isso impede o armazenamento em cache desses valores e acarreta um
custo significativo de desempenho.

Se possível, evite placeholders nessas diretivas.

## `resolve_root_symlink`

Por padrão, se o diretório raiz for um link simbólico, ele será resolvido
automaticamente pelo FrankenPHP (isso é necessário para o funcionamento correto
do PHP).
Se o diretório raiz não for um link simbólico, você pode desativar esse recurso.

```caddyfile
php_server {
    resolve_root_symlink false
}
```

Isso melhorará o desempenho se a diretiva `root` contiver
[placeholders](https://caddyserver.com/docs/conventions#placeholders).
O ganho será insignificante em outros casos.

## Logs

O logging é obviamente muito útil, mas, por definição, requer operações de E/S e
alocações de memória, o que reduz consideravelmente o desempenho.
Certifique-se de
[definir o nível de logging](https://caddyserver.com/docs/caddyfile/options#log)
corretamente e registrar em log apenas o necessário.

## Desempenho do PHP

O FrankenPHP usa o interpretador PHP oficial.
Todas as otimizações de desempenho usuais relacionadas ao PHP se aplicam ao
FrankenPHP.

Em particular:

- Verifique se o [OPcache](https://www.php.net/manual/pt_BR/book.opcache.php)
  está instalado, habilitado e configurado corretamente;
- Habilite as
  [otimizações do carregador automático do Composer](https://getcomposer.org/doc/articles/autoloader-optimization.md);
- Certifique-se de que o cache do `realpath` seja grande o suficiente para as
  necessidades da sua aplicação;
- Use
  [pré-carregamento](https://www.php.net/manual/pt_BR/opcache.preloading.php).

Para mais detalhes, leia
[a entrada dedicada na documentação do Symfony](https://symfony.com/doc/current/performance.html)
(a maioria das dicas é útil mesmo se você não usa o Symfony).
