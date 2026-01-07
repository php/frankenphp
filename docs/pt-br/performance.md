# Desempenho

Por padrão, o FrankenPHP tenta oferecer um bom equilíbrio entre desempenho e
facilidade de uso.
No entanto, é possível melhorar substancialmente o desempenho usando uma
configuração apropriada.

## Número de Threads e Workers

Por padrão, o FrankenPHP inicia 2 vezes mais threads e workers (no modo worker)
do que o número de CPUs disponíveis.

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

Embora seja sempre melhor saber exatamente como será o seu tráfego, aplicativos
reais tendem a ser mais imprevisíveis.
A [configuração](config.md#caddyfile-config) `max_threads` permite que
o FrankenPHP crie threads adicionais automaticamente em tempo de execução até o
limite especificado.
`max_threads` pode ajudar você a descobrir quantas threads são necessárias para
lidar com seu tráfego e pode tornar o servidor mais resiliente a picos de
latência.
Se definido como `auto`, o limite será estimado com base no `memory_limit` em
seu `php.ini`.
Se não for possível fazer isso, `auto` assumirá como padrão o valor 2x
`num_threads`.
Lembre-se de que `auto` pode subestimar bastante o número de threads
necessárias.
`max_threads` é semelhante ao
[pm.max_children](https://www.php.net/manual/en/install.fpm.configuration.php#pm.max-children)
do PHP FPM.
A principal diferença é que o FrankenPHP usa threads em vez de processos e as
delega automaticamente entre diferentes scripts worker e o 'modo clássico',
conforme necessário.

## Modo Worker

Habilitar [o modo worker](worker.md) melhora drasticamente o desempenho, mas sua
aplicação precisa ser adaptada para ser compatível com este modo: você precisa
criar um script worker e garantir que a aplicação não esteja com vazamento de
memória.

## Não use musl

A variante Alpine Linux das imagens oficiais do Docker e os binários padrão que
fornecemos usam [a biblioteca C `musl`](https://musl.libc.org).

O PHP é conhecido por ser
[mais lento](https://gitlab.alpinelinux.org/alpine/aports/-/issues/14381)
ao usar esta biblioteca C alternativa em vez da biblioteca GNU tradicional,
especialmente quando compilado no modo ZTS (thread-safe), que é necessário para
o FrankenPHP.
A diferença pode ser significativa em um ambiente com muitas threads.

Além disso,
[alguns bugs só acontecem ao usar musl](https://github.com/php/php-src/issues?q=sort%3Aupdated-desc+is%3Aissue+is%3Aopen+label%3ABug+musl).

Em ambientes de produção, recomendamos o uso do FrankenPHP vinculado à glibc,
compilado com um nível de otimização apropriado.

Isso pode ser alcançado usando as imagens Docker do Debian, usando nossos
pacotes [.deb](https://debs.henderkes.com) ou [.rpm](https://rpms.henderkes.com)
dos mantenedores, ou [compilando o FrankenPHP a partir do código-fonte](compile.md).

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
    root /root/to/your/app # adicionar explicitamente o root aqui permite um melhor cache
}
```

Isso pode reduzir significativamente o número de operações desnecessárias com
arquivos.

Uma abordagem alternativa com 0 operações desnecessárias no sistema de arquivos
seria usar a diretiva `php` e separar os arquivos estáticos do PHP por caminho.
Essa abordagem funciona bem se toda a sua aplicação for servida por um arquivo
de entrada.
Um exemplo de [configuração](config.md#caddyfile-config) que serve
arquivos estáticos a partir de uma pasta `/assets` poderia ser assim:

```caddyfile
route {
    @assets {
        path /assets/*
    }

    # tudo em /assets é gerenciado pelo servidor de arquivos
    file_server @assets {
        root /root/to/your/app
    }

    # tudo o que não está em /assets é gerenciado pelo seu arquivo PHP `index` ou `worker`
    rewrite index.php
    php {
        root /root/to/your/app # adicionar explicitamente o root aqui permite um melhor cache
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

- Verificar se o [OPcache](https://www.php.net/manual/en/book.opcache.php)
  está instalado, habilitado e configurado corretamente;
- Habilitar as
  [otimizações do carregador automático do Composer](https://getcomposer.org/doc/articles/autoloader-optimization.md);
- Garantir que o cache do `realpath` seja grande o suficiente para as
  necessidades da sua aplicação;
- Usar
  [pré-carregamento](https://www.php.net/manual/en/opcache.preloading.php).

Para mais detalhes, leia
[a entrada dedicada na documentação do Symfony](https://symfony.com/doc/current/performance.html)
(a maioria das dicas é útil mesmo que você não utilize o Symfony).

## Dividindo o Pool de Threads

É comum que aplicativos interajam com serviços externos lentos, como uma
API que tende a ser não confiável sob alta carga ou consistentemente leva mais
de 10 segundos para responder.
Nesses casos, pode ser benéfico dividir o pool de threads para ter pools
"lentos" dedicados.
Isso evita que os endpoints lentos consumam todos os recursos/threads do servidor
e limita a concorrência de requisições indo em direção ao endpoint lento,
semelhante a um pool de conexões.

```caddyfile
{
    frankenphp {
        max_threads 100 # máximo de 100 threads compartilhadas por todos os workers
    }
}

example.com {
    php_server {
        root /app/public # o diretório raiz da sua aplicação
        worker index.php {
            match /slow-endpoint/* # todas as requisições com o caminho /slow-endpoint/* são tratadas por este pool de threads
            num 10 # mínimo de 10 threads para requisições que correspondem a /slow-endpoint/*
        }
        worker index.php {
            match * # todas as outras requisições são tratadas separadamente
            num 20 # mínimo de 20 threads para outras requisições, mesmo que os endpoints lentos comecem a travar
        }
    }
}
```

Geralmente, também é aconselhável lidar com endpoints muito lentos de forma
assíncrona, usando mecanismos relevantes, como filas de mensagens.
