---
title: Compilação estática do FrankenPHP: servidor de aplicações PHP de binário único
description: Crie binários FrankenPHP estáticos ou principalmente estáticos com PHP e Caddy incorporados no Linux e macOS, personalize extensões e carregue módulos dinâmicos com glibc.
---

# Criar uma compilação estática

Em vez de usar uma instalação local da biblioteca PHP,
é possível criar uma compilação estática ou principalmente estática do FrankenPHP graças ao excelente [projeto static-php-cli](https://github.com/crazywhalecc/static-php-cli) (apesar do nome, este projeto suporta todas as SAPIs, não apenas CLI).

Com este método, um único binário portátil conterá o interpretador PHP, o servidor web Caddy e o FrankenPHP!

Executáveis nativos totalmente estáticos não requerem dependências e podem até ser executados na
[imagem Docker `scratch`](https://docs.docker.com/build/building/base-images/#create-a-minimal-base-image-using-scratch).
No entanto, eles não podem carregar extensões PHP dinâmicas (como o Xdebug) e têm algumas limitações por usarem a musl libc.

Binários principalmente estáticos requerem apenas `glibc` e podem carregar extensões dinâmicas.

Sempre que possível, recomendamos o uso de compilações principalmente estáticas baseadas na `glibc`.

O FrankenPHP também suporta [a incorporação da aplicação PHP no binário estático](embed.md).

## Linux

Fornecemos imagens Docker para compilar binários estáticos para Linux:

### Compilação totalmente estática baseada na musl

Para um binário totalmente estático que roda em qualquer distribuição Linux sem dependências, mas não suporta carregamento dinâmico de extensões:

```console
docker buildx bake --load static-builder-musl
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log dunglas/frankenphp:static-builder-musl
cp "output/frankenphp-linux-$(uname -m)" frankenphp
```

Para melhor desempenho em cenários com alta concorrência, considere usar o alocador [`mimalloc`](https://github.com/microsoft/mimalloc).

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log -e MIMALLOC=1 dunglas/frankenphp:static-builder-musl
```

### Compilação principalmente estática baseada na glibc (com suporte a extensões dinâmicas)

Para um binário que suporta o carregamento dinâmico de extensões PHP, mantendo as extensões selecionadas compiladas estaticamente:

```console
docker buildx bake --load static-builder-gnu
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log dunglas/frankenphp:static-builder-gnu
cp "output/frankenphp-linux-$(uname -m)" frankenphp
```

Este binário suporta todas as versões 2.28 e superiores da `glibc`, mas não roda em sistemas baseados em musl (como o Alpine Linux).

O binário principalmente estático (exceto a `glibc`) resultante é chamado `frankenphp` e está disponível no diretório atual.

Se a compilação falhar, os logs do static-php-cli estarão em `output/log/`.

Se você quiser compilar o binário estático sem o Docker, consulte as instruções para macOS, que também funcionam para Linux.

### Extensões PHP personalizadas na compilação estática

Por padrão, as extensões PHP mais populares são compiladas.
Os padrões estão em [`craft.yml`](https://github.com/php/frankenphp/blob/main/craft.yml).

Para reduzir o tamanho do binário e a superfície de ataque, você pode escolher a lista de extensões a serem compiladas usando a variável `PHP_EXTENSIONS`.

Por exemplo, execute o seguinte comando para compilar apenas a extensão `opcache`:

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log -e PHP_EXTENSIONS=opcache,pdo_sqlite dunglas/frankenphp:static-builder-musl
# ...
```

Para adicionar bibliotecas que habilitem funcionalidades adicionais às extensões habilitadas, você pode passar a variável `PHP_EXTENSION_LIBS`:

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log \
  -e PHP_EXTENSIONS=gd \
  -e PHP_EXTENSION_LIBS=libjpeg,libwebp \
  dunglas/frankenphp:static-builder-musl
```

### Módulos Caddy extras

Para adicionar módulos Caddy extras ou passar outros argumentos para o [`xcaddy`](https://github.com/caddyserver/xcaddy), use a variável `XCADDY_ARGS`:

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log \
  -e XCADDY_ARGS="--with github.com/darkweak/souin/plugins/caddy --with github.com/dunglas/caddy-cbrotli --with github.com/dunglas/mercure/caddy --with github.com/dunglas/vulcain/caddy" \
  dunglas/frankenphp:static-builder-musl
```

Neste exemplo, adicionamos o módulo de cache HTTP [Souin](https://souin.io) para o Caddy, bem como os módulos [cbrotli](https://github.com/dunglas/caddy-cbrotli), [Mercure](https://mercure.rocks) e [Vulcain](https://vulcain.rocks).

> [!TIP]
>
> Os módulos cbrotli, Mercure e Vulcain são incluídos por padrão se `XCADDY_ARGS` estiver vazio ou não definido.
> Se você personalizar o valor de `XCADDY_ARGS`, deverá incluí-los explicitamente se desejar que sejam incluídos.

Veja também como [personalizar a compilação estática do FrankenPHP](#personalizando-a-compilação-estática-do-frankenphp)

### Token do GitHub

Se você atingir o limite de taxa da API do GitHub, defina um Token de Acesso Pessoal do GitHub em uma variável de ambiente chamada `GITHUB_TOKEN`:

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log -e GITHUB_TOKEN="xxx" dunglas/frankenphp:static-builder-musl
# ...
```

## macOS

Execute o seguinte script para criar um binário estático para macOS (você precisa ter o [Homebrew](https://brew.sh/) instalado):

```console
git clone https://github.com/php/frankenphp
cd frankenphp
./build-static.sh
```

Observação: este script também funciona no Linux (e provavelmente em outros sistemas Unix) e é usado internamente pelas imagens Docker que fornecemos.

Publicamos binários para macOS apenas para Apple Silicon. Em Macs Intel, compile a partir do código-fonte com este script.

## Personalizando a compilação estática do FrankenPHP

As seguintes variáveis de ambiente podem ser passadas para `docker run` e para o script `build-static.sh` para personalizar a compilação estática:

- `FRANKENPHP_VERSION`: a versão do FrankenPHP a ser usada;
- `PHP_VERSION`: a versão do PHP a ser usada;
- `PHP_EXTENSIONS`: as extensões PHP a serem compiladas ([lista de extensões suportadas](https://static-php.dev/en/guide/extensions.html));
- `PHP_EXTENSION_LIBS`: bibliotecas extras a serem compiladas que adicionam recursos às extensões;
- `XCADDY_ARGS`: argumentos a passar para o [`xcaddy`](https://github.com/caddyserver/xcaddy), por exemplo, para adicionar módulos Caddy extras;
- `EMBED`: caminho da aplicação PHP a ser incorporada no binário;
- `CLEAN`: quando definida, a libphp e todas as suas dependências são compiladas do zero (sem cache);
- `COMPRESS`: quando definida como `1`, compacta o binário resultante usando UPX (apenas Linux; ignorada quando `DEBUG_SYMBOLS` está definida);
- `DEBUG_SYMBOLS`: quando definida, os símbolos de depuração não serão removidos e serão adicionados ao binário;
- `MIMALLOC`: (experimental, somente Linux) substitui o mallocng do musl por [`mimalloc`](https://github.com/microsoft/mimalloc) para melhor desempenho. Recomendamos usar isso apenas para compilações que visam o musl; para glibc, prefira desabilitar essa opção e usar [`LD_PRELOAD`](https://microsoft.github.io/mimalloc/overrides.html) ao executar seu binário, em vez disso;
- `RELEASE`: (somente pessoas mantenedoras) quando definida, o binário resultante será enviado para o GitHub;
- `LIBC`: (somente Linux) `musl` para um binário totalmente estático (o padrão), ou `gnu` para linkar dinamicamente contra a glibc do sistema. É um atalho para `SPC_TARGET` e `SPC_TOOLCHAIN` do static-php-cli, que ainda podem ser definidos diretamente; eles devem ser variáveis de ambiente reais porque o static-php-cli resolve seu toolchain antes de ler `craft.yml`. O padrão `musl` visa a CPU base da arquitetura (`x86_64-linux-musl` ou `aarch64-linux-musl`) para que o binário funcione em qualquer CPU; defina `SPC_TARGET=native-native-musl` para otimizar para a máquina de compilação, em vez disso.

## Carregando extensões PHP dinamicamente no binário estático

Com os binários baseados na glibc ou no macOS, você pode carregar extensões PHP dinamicamente. No entanto, essas extensões precisarão ser compiladas com suporte a ZTS.
Como a maioria dos gerenciadores de pacotes não oferece atualmente versões ZTS de suas extensões, você terá que compilá-las você mesmo.

Para isso, você pode executar o contêiner Docker `static-builder-gnu`, acessar ele remotamente e compilar as extensões com `./configure --with-php-config=/go/src/app/dist/buildroot/bin/php-config`.

Passos de exemplo para [a extensão Xdebug](https://xdebug.org):

```console
docker buildx bake --load static-builder-gnu
docker run --rm -v "$PWD/dist:/go/src/app/dist" -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log dunglas/frankenphp:static-builder-gnu
docker run --rm -it -v "$PWD/dist:/go/src/app/dist" dunglas/frankenphp:static-builder-gnu /bin/bash
cd /go/src/app/dist/buildroot/bin
git clone https://github.com/xdebug/xdebug.git && cd xdebug
../phpize
./configure --with-php-config=/go/src/app/dist/buildroot/bin/php-config
make
exit
cp dist/buildroot/bin/xdebug/modules/xdebug.so xdebug-zts.so
cp "output/frankenphp-linux-$(uname -m)" frankenphp
```

Isso terá criado `frankenphp` e `xdebug-zts.so` no diretório atual.
Se você mover o `xdebug-zts.so` para seu diretório de extensões, adicione `zend_extension=xdebug-zts.so` ao seu php.ini e execute o FrankenPHP, ele carregará o Xdebug.
