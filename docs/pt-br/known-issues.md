---
title: Problemas conhecidos, extensões incompatíveis e soluções alternativas do FrankenPHP
description: Referência das limitações do FrankenPHP, incluindo extensões PHP não suportadas, ressalvas da musl libc, configuração TLS do Docker com 127.0.0.1 e scripts @php do Composer.
---

# Problemas conhecidos

## Extensões PHP não suportadas

As seguintes extensões são conhecidas por não serem compatíveis com o FrankenPHP:

| Nome                                                                                                        | Motivo          | Alternativas                                                                                                                                                                    |
| :---------------------------------------------------------------------------------------------------------- | :-------------- | :------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| [imap](https://www.php.net/manual/imap.installation.php)                                                    | Não é thread-safe | [javanile/php-imap2](https://github.com/javanile/php-imap2), [webklex/php-imap](https://github.com/Webklex/php-imap), [ImapEngine](https://github.com/DirectoryTree/ImapEngine) |
| [newrelic](https://docs.newrelic.com/docs/apm/agents/php-agent/getting-started/introduction-new-relic-php/) | Não é thread-safe | -                                                                                                                                                                               |
| [pcov](https://github.com/krakjoe/pcov)                                                                     | Não é thread-safe ([SIGSEGV no modo clássico](https://github.com/krakjoe/pcov/issues/132)) | Coletar cobertura com a SAPI CLI (execute PHPUnit/Pest fora do FrankenPHP) |

## Extensões PHP com falhas

As seguintes extensões apresentam falhas conhecidas e comportamentos inesperados quando usadas com o FrankenPHP:

| Nome                                          | Problema                                                                                                                                         |
| :-------------------------------------------- | :----------------------------------------------------------------------------------------------------------------------------------------------- |
| [datadog](https://github.com/DataDog)         | Apresenta instabilidade ao perfilar o FrankenPHP. Este bug está [sendo rastreado pela DataDog](https://github.com/DataDog/dd-trace-php/issues/3729). |
| [blackfire](https://blackfire.io/)            | [O suporte ao FrankenPHP está em fase beta e ainda não está completo em recursos](https://docs.blackfire.io/php/integrations/frankenphp).                            |
| [imagick](https://github.com/Imagick/imagick) | Os threads OpenMP do ImageMagick entram em conflito com os threads do FrankenPHP, causando instabilidade e travamentos. Mitigue desativando o threading do ImageMagick via `\Imagick::setResourceLimit(\Imagick::RESOURCETYPE_THREAD, 1)` ou compilando o ImageMagick com `--disable-openmp`. O OpenMP já está desativado nos binários estáticos e nos pacotes oficiais `apt`/`apk`/`rpm`, então apenas as imagens Docker e instalações Homebrew são afetadas. |

## Exceder o limite de memória "hard" do PHP paralisa o servidor inteiro

Desde o PHP 8.2.35, 8.3.36, 8.4.27 e 8.5.12, exceder o `memory_limit` não para mais o script imediatamente: o erro fatal "Allowed memory size exhausted" é lançado na próxima chamada de função ou iteração de loop.
Se o script alocar além do limite de memória "hard" antes disso, o PHP encerra o processo.
O limite "hard" padrão é o dobro do `memory_limit` (ou `memory_limit` mais 1 GiB acima de 1 GiB) e pode ser alterado com a configuração INI `memory_hard_limit_percent`.

O código que continua executando além do `memory_limit` pode atingir esse limite, por exemplo, uma única alocação maior que `memory_limit` (como `array_merge()` em grandes arrays) ou concatenações de strings consecutivas.

O PHP-FPM perde apenas um processo filho.
O FrankenPHP atende a todas as requisições a partir de threads de um único processo, então cada requisição em andamento é perdida e o servidor para até ser reiniciado.
Execute o FrankenPHP sob um supervisor que o reinicie, como uma política de reinício do Docker ou o `Restart=` do systemd (já configurado nos pacotes `deb` e `rpm`).

## get_browser

A função [`get_browser()`](https://www.php.net/manual/function.get-browser.php) parece apresentar mau desempenho após algum tempo.
Uma solução alternativa é armazenar em cache (por exemplo, com [APCu](https://www.php.net/manual/book.apcu.php)) os resultados por Agente de Usuário, pois são estáticos.

## Binário standalone e imagens Docker baseadas em Alpine

O binário totalmente estático e as imagens Docker baseadas em Alpine (`dunglas/frankenphp:*-alpine`) usam a [musl libc](https://musl.libc.org/) em vez de [glibc e similares](https://www.etalabs.net/compare_libcs.html), para manter um tamanho de binário menor.
Isso pode levar a alguns problemas de compatibilidade.
Em particular, o sinalizador glob `GLOB_BRACE` [não está disponível](https://www.php.net/manual/function.glob.php).

Prefira usar a variante GNU do binário estático e imagens Docker baseadas em Debian se encontrar problemas.

## Usando `https://127.0.0.1` com Docker

Por padrão, o FrankenPHP gera um certificado TLS para `localhost`.
É a opção mais fácil e recomendada para desenvolvimento local.

Se você realmente deseja usar `127.0.0.1` como host, é possível configurá-lo para gerar um certificado para ele definindo o nome do servidor como `127.0.0.1`.

Infelizmente, isso não é suficiente ao usar o Docker devido ao [seu sistema de rede](https://docs.docker.com/network/).
Você receberá um erro TLS semelhante a `curl: (35) LibreSSL/3.3.6: error:1404B438:SSL routines:ST_CONNECT:tlsv1 alert internal error`.

Se você estiver usando Linux, uma solução é usar [o driver de rede do host](https://docs.docker.com/network/network-tutorial-host/):

```console
docker run \
    -e SERVER_NAME="127.0.0.1" \
    -v $PWD:/app/public \
    --network host \
    dunglas/frankenphp
```

O driver de rede do host não é compatível com Mac e Windows. Nessas plataformas, você terá que adivinhar o endereço IP do contêiner e incluí-lo nos nomes dos servidores.

Execute o comando `docker network inspect bridge` e verifique a chave `Containers` para identificar o último endereço IP atribuído atualmente sob a chave `IPv4Address`, e incremente-o em um. Se nenhum contêiner estiver em execução, o primeiro endereço IP atribuído geralmente é `172.17.0.2`.

Em seguida, inclua isso na variável de ambiente `SERVER_NAME`:

```console
docker run \
    -e SERVER_NAME="127.0.0.1, 172.17.0.3" \
    -v $PWD:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

> [!CAUTION]
>
> Certifique-se de substituir `172.17.0.3` pelo IP que será atribuído ao seu contêiner.

Agora você deve conseguir acessar `https://127.0.0.1` a partir da máquina host.

Se este não for o caso, inicie o FrankenPHP em modo de depuração para tentar descobrir o problema:

```console
docker run \
    -e CADDY_GLOBAL_OPTIONS="debug" \
    -e SERVER_NAME="127.0.0.1" \
    -v $PWD:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

## Scripts do Composer que referenciam `@php`

[Scripts do Composer](https://getcomposer.org/doc/articles/scripts.md) podem querer executar um binário PHP para algumas tarefas, por exemplo, em [um projeto Laravel](laravel.md) para executar `@php artisan package:discover --ansi`. Isso [atualmente falha](https://github.com/php/frankenphp/issues/483#issuecomment-1899890915) por dois motivos:

- O Composer não sabe como chamar o binário do FrankenPHP;
- O Composer pode adicionar configurações do PHP usando a flag `-d` no comando, que o FrankenPHP ainda não suporta.

Como solução alternativa, podemos criar um script de shell em `/usr/local/bin/php` que remove os parâmetros não suportados e, em seguida, chama o FrankenPHP:

```bash
#!/usr/bin/env bash
# /usr/local/bin/php
args=("$@")
index=0
for i in "$@"
do
    if [ "$i" == "-d" ]; then
        unset 'args[$index]'
        unset 'args[$index+1]'
    fi
    index=$((index+1))
done

/usr/local/bin/frankenphp php-cli ${args[@]}
```

Em seguida, defina a variável de ambiente `PHP_BINARY` para o caminho do nosso script `php` e execute o Composer:

```console
export PHP_BINARY=/usr/local/bin/php
composer install
```

## Solução de problemas de TLS/SSL com binários estáticos

Ao usar binários estáticos, você pode encontrar os seguintes erros relacionados a TLS, por exemplo, ao enviar emails usando STARTTLS:

```text
Unable to connect with STARTTLS: stream_socket_enable_crypto(): SSL operation failed with code 5. OpenSSL Error messages:
error:80000002:system library::No such file or directory
error:80000002:system library::No such file or directory
error:80000002:system library::No such file or directory
error:0A000086:SSL routines::certificate verify failed
```

Como o binário estático não empacota certificados TLS, você precisa apontar o OpenSSL para a instalação local de certificados de CA.

Inspecione a saída de [`openssl_get_cert_locations()`](https://www.php.net/manual/function.openssl-get-cert-locations.php), para descobrir onde os certificados de CA devem ser instalados e armazene-os neste local.

> [!WARNING]
>
> Contextos web e CLI podem ter configurações diferentes.
> Certifique-se de executar `openssl_get_cert_locations()` no contexto apropriado.

[Certificados CA extraídos do Mozilla podem ser baixados no site do cURL](https://curl.se/docs/caextract.html).

Como alternativa, muitas distribuições, incluindo Debian, Ubuntu e Alpine, fornecem pacotes chamados `ca-certificates` que contêm esses certificados.

Também é possível usar `SSL_CERT_FILE` e `SSL_CERT_DIR` para indicar à OpenSSL onde procurar certificados CA:

```console
# Define variáveis de ambiente para certificados TLS
export SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
export SSL_CERT_DIR=/etc/ssl/certs
