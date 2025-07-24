# Criar uma compilação estática

Em vez de usar uma instalação local da biblioteca PHP, é possível criar uma
compilação estática ou principalmente estática do FrankenPHP graças ao excelente
[projeto static-php-cli](https://github.com/crazywhalecc/static-php-cli) (apesar
do nome, este projeto suporta todas as SAPIs, não apenas CLI).

Com este método, um único binário portátil conterá o interpretador PHP, o
servidor web Caddy e o FrankenPHP!

Executáveis nativos totalmente estáticos não requerem dependências e podem até
ser executados na
[imagem Docker `scratch`](https://docs.docker.com/build/building/base-images/#create-a-minimal-base-image-using-scratch).
No entanto, eles não podem carregar extensões PHP dinâmicas (como o Xdebug) e
têm algumas limitações por usarem a `libc` `musl`.

A maioria dos binários estáticos requer apenas `glibc` e pode carregar extensões
dinâmicas.

Sempre que possível, recomendamos o uso de compilações principalmente estáticas
baseadas na `glibc`.

O FrankenPHP também suporta
[a incorporação da aplicação PHP no binário estático](embed.md).

## Linux

Fornecemos imagens Docker para compilar binários estáticos para Linux:

### Compilação totalmente estática baseada na `musl`

Para um binário totalmente estático que roda em qualquer distribuição Linux sem
dependências, mas não suporta carregamento dinâmico de extensões:

```console
docker buildx bake --load static-builder-musl
docker cp $(docker create --name static-builder-musl dunglas/frankenphp:static-builder-musl):/go/src/app/dist/frankenphp-linux-$(uname -m) frankenphp ; docker rm static-builder-musl
```

Para melhor desempenho em cenários com alta concorrência, considere usar o
alocador [`mimalloc`](https://github.com/microsoft/mimalloc).

```console
docker buildx bake --load --set static-builder-musl.args.MIMALLOC=1 static-builder-musl
```

### Compilação principalmente estática baseada na `glibc` (com suporte a extensões dinâmicas)

Para um binário que suporta o carregamento dinâmico de extensões PHP, mantendo
as extensões selecionadas compiladas estaticamente:

```console
docker buildx bake --load static-builder-gnu
docker cp $(docker create --name static-builder-gnu dunglas/frankenphp:static-builder-gnu):/go/src/app/dist/frankenphp-linux-$(uname -m) frankenphp ; docker rm static-builder-gnu
```

Este binário suporta todas as versões 2.17 e superiores da `glibc`, mas não roda
em sistemas baseados em `musl` (como o Alpine Linux).

O binário principalmente estático (exceto a `glibc`) resultante é chamado
`frankenphp` e está disponível no diretório atual.

Se você quiser compilar o binário estático sem o Docker, consulte as instruções
para macOS, que também funcionam para Linux.

### Extensões personalizadas

Por padrão, as extensões PHP mais populares são compiladas.

Para reduzir o tamanho do binário e a superfície de ataque, você pode escolher a
lista de extensões a serem compiladas usando o `ARG` `PHP_EXTENSIONS` do Docker.

Por exemplo, execute o seguinte comando para compilar apenas a extensão
`opcache`:

```console
docker buildx bake --load --set static-builder-musl.args.PHP_EXTENSIONS=opcache,pdo_sqlite static-builder-musl
# ...
```

Para adicionar bibliotecas que habilitem funcionalidades adicionais às extensões
habilitadas, você pode passar o `ARG` `PHP_EXTENSION_LIBS` do Docker:

```console
docker buildx bake \
  --load \
  --set static-builder-musl.args.PHP_EXTENSIONS=gd \
  --set static-builder-musl.args.PHP_EXTENSION_LIBS=libjpeg,libwebp \
  static-builder-musl
```

### Módulos Caddy extras

Para adicionar módulos Caddy extras ou passar outros argumentos para o
[`xcaddy`](https://github.com/caddyserver/xcaddy), use o `ARG` `XCADDY_ARGS` do
Docker:

```console
docker buildx bake \
  --load \
  --set static-builder-musl.args.XCADDY_ARGS="--with github.com/darkweak/souin/plugins/caddy --with github.com/dunglas/caddy-cbrotli --with github.com/dunglas/mercure/caddy --with github.com/dunglas/vulcain/caddy" \
  static-builder-musl
```

Neste exemplo, adicionamos o módulo de cache HTTP [Souin](https://souin.io) para
o Caddy, bem como os módulos
[cbrotli](https://github.com/dunglas/caddy-cbrotli),
[Mercure](https://mercure.rocks) e [Vulcain](https://vulcain.rocks).

> [!TIP]
>
> Os módulos cbrotli, Mercure e Vulcain são incluídos por padrão se
> `XCADDY_ARGS` estiver vazio ou não definido.
> Se você personalizar o valor de `XCADDY_ARGS`, deverá incluí-los
> explicitamente se desejar que sejam incluídos.

Veja também como [personalizar a compilação](#personalizando-a-compilação).

### Token do GitHub

Se você atingir o limite de taxa da API do GitHub, defina um Token de Acesso
Pessoal do GitHub em uma variável de ambiente chamada `GITHUB_TOKEN`:

```console
GITHUB_TOKEN="xxx" docker --load buildx bake static-builder-musl
# ...
```

## macOS

Execute o seguinte script para criar um binário estático para macOS (você
precisa ter o [Homebrew](https://brew.sh/) instalado):

```console
git clone https://github.com/dunglas/frankenphp
cd frankenphp
./build-static.sh
```

Observação: este script também funciona no Linux (e provavelmente em outros
sistemas Unix) e é usado internamente pelas imagens Docker que fornecemos.

## Personalizando a compilação

As seguintes variáveis de ambiente podem ser passadas para `docker build` e para
o script `build-static.sh` para personalizar a compilação estática:

- `FRANKENPHP_VERSION`: a versão do FrankenPHP a ser usada.
- `PHP_VERSION`: a versão do PHP a ser usada.
- `PHP_EXTENSIONS`: as extensões PHP a serem compiladas
  ([lista de extensões suportadas](https://static-php.dev/en/guide/extensions.html)).
- `PHP_EXTENSION_LIBS`: bibliotecas extras a serem compiladas que adicionam
  recursos às extensões.
- `XCADDY_ARGS`: argumentos a passar para o
  [`xcaddy`](https://github.com/caddyserver/xcaddy), por exemplo, para adicionar
  módulos Caddy extras.
- `EMBED`: caminho da aplicação PHP a ser incorporada no binário.
- `CLEAN`: quando definida, a `libphp` e todas as suas dependências são
  compiladas do zero (sem cache).
- `NO_COMPRESS`: não compacta o binário resultante usando UPX.
- `DEBUG_SYMBOLS`: quando definida, os símbolos de depuração não serão removidos
  e serão adicionados ao binário.
- `MIMALLOC`: (experimental, somente Linux) substitui `mallocng` da `musl` por
  [`mimalloc`](https://github.com/microsoft/mimalloc) para melhor desempenho.
  Recomendamos usar isso apenas para compilações direcionadas à `musl`; para
  `glibc`, prefira desabilitar essa opção e usar
  [`LD_PRELOAD`](https://microsoft.github.io/mimalloc/overrides.html) ao
  executar seu binário.
- `RELEASE`: (somente pessoas mantenedoras) quando definida, o binário
  resultante será enviado para o GitHub.

## Extensões

Com os binários `glibc` ou baseados em macOS, você pode carregar extensões PHP
dinamicamente.
No entanto, essas extensões precisarão ser compiladas com suporte a ZTS.
Como a maioria dos gerenciadores de pacotes não oferece atualmente versões ZTS
de suas extensões, você terá que compilá-las você mesmo.

Para isso, você pode compilar e executar o contêiner Docker
`static-builder-gnu`, acessá-lo remotamente e compilar as extensões com
`./configure --with-php-config=/go/src/app/dist/static-php-cli/buildroot/bin/php-config`.

Passos de exemplo para [a extensão Xdebug](https://xdebug.org):

```console
docker build -t gnu-ext -f static-builder-gnu.Dockerfile --build-arg FRANKENPHP_VERSION=1.0 .
docker create --name static-builder-gnu -it gnu-ext /bin/sh
docker start static-builder-gnu
docker exec -it static-builder-gnu /bin/sh
cd /go/src/app/dist/static-php-cli/buildroot/bin
git clone https://github.com/xdebug/xdebug.git && cd xdebug
source scl_source enable devtoolset-10
../phpize
./configure --with-php-config=/go/src/app/dist/static-php-cli/buildroot/bin/php-config
make
exit
docker cp static-builder-gnu:/go/src/app/dist/static-php-cli/buildroot/bin/xdebug/modules/xdebug.so xdebug-zts.so
docker cp static-builder-gnu:/go/src/app/dist/frankenphp-linux-$(uname -m) ./frankenphp
docker stop static-builder-gnu
docker rm static-builder-gnu
docker rmi gnu-ext
```

Isso criará `frankenphp` e `xdebug-zts.so` no diretório atual.
Se você mover `xdebug-zts.so` para o diretório de extensões, adicione
`zend_extension=xdebug-zts.so` ao seu `php.ini` e execute o FrankenPHP, ele
carregará o Xdebug.
