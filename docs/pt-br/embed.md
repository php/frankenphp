# Aplicações PHP como binários independentes

O FrankenPHP tem a capacidade de incorporar o código-fonte e os assets de
aplicações PHP em um binário estático e independente.

Graças a esse recurso, aplicações PHP podem ser distribuídas como binários
independentes que incluem a própria aplicação, o interpretador PHP e o Caddy, um
servidor web de nível de produção.

Saiba mais sobre esse recurso
[na apresentação feita por Kévin na SymfonyCon 2023](https://dunglas.dev/2023/12/php-and-symfony-apps-as-standalone-binaries/).

Para incorporar aplicações Laravel,
[leia esta entrada específica na documentação](laravel.md#aplicacoes-laravel-como-binarios-independentes).

## Preparando sua aplicação

Antes de criar o binário independente, certifique-se de que sua aplicação esteja
pronta para ser incorporada.

Por exemplo, você provavelmente deseja:

- Instalar as dependências de produção da aplicação;
- Fazer o dump do carregador automático;
- Habilitar o modo de produção da sua aplicação (se houver);
- Remover arquivos desnecessários, como `.git` ou testes, para reduzir o tamanho
  do seu binário final.

Por exemplo, para uma aplicação Symfony, você pode usar os seguintes comandos:

```console
# Exporta o projeto para se livrar de .git/, etc.
mkdir $TMPDIR/minha-aplicacao-preparada
git archive HEAD | tar -x -C $TMPDIR/minha-aplicacao-preparada
cd $TMPDIR/minha-aplicacao-preparada

# Define as variáveis de ambiente adequadas
echo APP_ENV=prod > .env.local
echo APP_DEBUG=0 >> .env.local

# Remove os testes e outros arquivos desnecessários para economizar espaço.
# Como alternativa, adicione esses arquivos com o atributo export-ignore no seu
# arquivo .gitattributes.
rm -Rf tests/

# Instala as dependências
composer install --ignore-platform-reqs --no-dev -a

# Otimiza o arquivo .env
composer dump-env prod
```

### Personalizando a configuração

Para personalizar
[a configuração](config.md), você pode colocar um arquivo `Caddyfile` e um
arquivo `php.ini` no diretório principal da aplicação a ser incorporada
(`$TMPDIR/minha-aplicacao-preparada` no exemplo anterior).

## Criando um binário do Linux

A maneira mais fácil de criar um binário do Linux é usar o builder baseado em
Docker que fornecemos.

1. Crie um arquivo chamado `static-build.Dockerfile` no repositório da sua
   aplicação:

   ```dockerfile
   FROM --platform=linux/amd64 dunglas/frankenphp:static-builder-musl
   # Se você pretende executar o binário em sistemas glibc, use o static-builder-gnu

   # Copia sua aplicação
   WORKDIR /go/src/app/dist/app
   COPY . .

   # Compila o binário estático
   WORKDIR /go/src/app/
   RUN EMBED=dist/app/ ./build-static.sh
   ```

   > [!CAUTION]
   >
   > Alguns arquivos `.dockerignore` (por exemplo, o
   > [`.dockerignore` padrão do Docker do Symfony](https://github.com/dunglas/symfony-docker/blob/main/.dockerignore))
   > ignorarão o diretório `vendor/` e os arquivos `.env`.
   > Certifique-se de ajustar ou remover o arquivo `.dockerignore` antes da
   > construção.

2. Construa:

   ```console
   docker build -t aplicacao-estatica -f static-build.Dockerfile .
   ```

3. Extraia o binário:

   ```console
   docker cp $(docker create --name aplicacao-estatica-tmp aplicacao-estatica):/go/src/app/dist/frankenphp-linux-x86_64 minha-aplicacao ; docker rm aplicacao-estatica-tmp
   ```

O binário resultante é o arquivo `minha-aplicacao` no diretório atual.

## Criando um binário para outros sistemas operacionais

Se você não quiser usar o Docker ou quiser compilar um binário para macOS, use o
script de shell que fornecemos:

```console
git clone https://github.com/php/frankenphp
cd frankenphp
EMBED=/caminho/para/sua/aplicacao ./build-static.sh
```

O binário resultante é o arquivo `frankenphp-<os>-<arch>` no diretório `dist/`.

## Usando o binário

É isso! O arquivo `minha-aplicacao` (ou `dist/frankenphp-<os>-<arch>` em outros
sistemas operacionais) contém sua aplicação independente!

Para iniciar a aplicação web, execute:

```console
./minha-aplicacao php-server
```

Se a sua aplicação contiver um [worker script](worker.md), inicie o worker com
algo como:

```console
./minha-aplicacao php-server --worker public/index.php
```

Para habilitar HTTPS (um certificado Let's Encrypt é criado automaticamente),
HTTP/2 e HTTP/3, especifique o nome de domínio a ser usado:

```console
./minha-aplicacao php-server --domain localhost
```

Você também pode executar os scripts PHP CLI incorporados ao seu binário:

```console
./minha-aplicacao php-cli bin/console
```

## Extensões PHP

Por padrão, o script compilará as extensões requeridas pelo arquivo
`composer.json` do seu projeto, se houver.
Se o arquivo `composer.json` não existir, as extensões padrão serão compiladas,
conforme documentado na [entrada de compilações estáticas](static.md).

Para personalizar as extensões, use a variável de ambiente `PHP_EXTENSIONS`.

## Personalizando a compilação

[Leia a documentação da compilação estática](static.md) para ver como
personalizar o binário (extensões, versão do PHP...).

## Distribuindo o binário

No Linux, o binário criado é compactado usando [UPX](https://upx.github.io).

No Mac, para reduzir o tamanho do arquivo antes de enviá-lo, você pode
compactá-lo.
Recomendamos usar `xz`.
