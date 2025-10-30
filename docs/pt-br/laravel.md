# Laravel

## Docker

Servir uma aplicação web [Laravel](https://laravel.com) com FrankenPHP é tão
fácil quanto montar o projeto no diretório `/app` da imagem Docker oficial.

Execute este comando a partir do diretório principal da sua aplicação Laravel:

```console
docker run -p 80:80 -p 443:443 -p 443:443/udp -v $PWD:/app dunglas/frankenphp
```

E divirta-se!

## Instalação local

Alternativamente, você pode executar seus projetos Laravel com FrankenPHP a
partir da sua máquina local:

1. [Baixe o binário correspondente ao seu sistema](../#standalone-binary).
2. Adicione a seguinte configuração a um arquivo chamado `Caddyfile` no
   diretório raiz do seu projeto Laravel:

   ```caddyfile
   {
       frankenphp
   }

   # O nome de domínio do seu servidor
   localhost {
       # Define o diretório raiz como public/
       root public/
       # Habilita a compressão (opcional)
       encode zstd br gzip
       # Executa os arquivos PHP a partir do diretório public/ e serve os assets
       php_server {
           try_files {path} index.php
       }
   }
   ```

3. Inicie o FrankenPHP a partir do diretório raiz do seu projeto Laravel:
   `frankenphp run`.

## Laravel Octane

O Octane pode ser instalado através do gerenciador de pacotes Composer:

```console
composer require laravel/octane
```

Após instalar o Octane, você pode executar o comando `octane:install` do
Artisan, que instalará o arquivo de configuração do Octane em sua aplicação:

```console
php artisan octane:install --server=frankenphp
```

O servidor Octane pode ser iniciado por meio do comando `octane:frankenphp` do
Artisan.

```console
php artisan octane:frankenphp
```

O comando `octane:frankenphp` pode receber as seguintes opções:

- `--host`: O endereço IP ao qual o servidor deve se vincular (padrão:
  `127.0.0.1`);
- `--port`: A porta na qual o servidor deve estar disponível (padrão: `8000`);
- `--admin-port`: A porta na qual o servidor de administração deve estar
  disponível (padrão: `2019`);
- `--workers`: O número de workers que devem estar disponíveis para processar
  requisições (padrão: `auto`);
- `--max-requests`: O número de requisições a serem processadas antes de
  recarregar o servidor (padrão: `500`);
- `--caddyfile`: O caminho para o arquivo `Caddyfile` do FrankenPHP (padrão:
  [stub do `Caddyfile` no Laravel Octane](https://github.com/laravel/octane/blob/2.x/src/Commands/stubs/Caddyfile));
- `--https`: Habilita HTTPS, HTTP/2 e HTTP/3 e gera e renova certificados
  automaticamente;
- `--http-redirect`: Habilita o redirecionamento de HTTP para HTTPS (somente
- habilitado se `--https` for passada);
- `--watch`: Recarrega o servidor automaticamente quando a aplicação é
  modificada;
- `--poll`: Usa o polling do sistema de arquivos durante a verificação para
  monitorar arquivos em uma rede;
- `--log-level`: Registra mensagens de log no nível de log especificado ou acima
  dele, usando o logger nativo do Caddy.

> [!TIP]
> Para obter logs JSON estruturados (útil ao usar soluções de análise de logs),
> passe explicitamente a opção `--log-level`.

Saiba mais sobre o
[Laravel Octane em sua documentação oficial](https://laravel.com/docs/octane).

## Aplicações Laravel como binários independentes

Usando o [recurso de incorporação de aplicações do FrankenPHP](embed.md), é
possível distribuir aplicações Laravel como binários independentes.

Siga estes passos para empacotar sua aplicação Laravel como um binário
independente para Linux:

1. Crie um arquivo chamado `static-build.Dockerfile` no repositório da sua
   aplicação:

   ```dockerfile
   FROM dunglas/frankenphp:static-builder-gnu
   # Se você pretende executar o binário em sistemas musl-libc, use o static-builder-musl

   # Copia sua aplicação
   WORKDIR /go/src/app/dist/app
   COPY . .

   # Remove os testes e outros arquivos desnecessários para economizar espaço
   # Como alternativa, adicione esses arquivos a um arquivo .dockerignore
   RUN rm -Rf tests/

   # Copia o arquivo .env
   RUN cp .env.example .env
   # Altera APP_ENV e APP_DEBUG para que estejam prontas para produção
   RUN sed -i'' -e 's/^APP_ENV=.*/APP_ENV=production/' -e 's/^APP_DEBUG=.*/APP_DEBUG=false/' .env

   # Faça outras alterações no seu arquivo .env, se necessário

   # Instala as dependências
   RUN composer install --ignore-platform-reqs --no-dev -a

   # Compila o binário estático
   WORKDIR /go/src/app/
   RUN EMBED=dist/app/ ./build-static.sh
   ```

   > [!CAUTION]
   >
   > Alguns arquivos `.dockerignore` ignorarão o diretório `vendor/` e os
   > arquivos `.env`.
   > Certifique-se de ajustar ou remover o arquivo `.dockerignore` antes da
   > compilação.

2. Construa:

   ```console
   docker build -t static-laravel-app -f static-build.Dockerfile .
   ```

3. Extraia o binário:

   ```console
   docker cp $(docker create --name static-laravel-app-tmp static-laravel-app):/go/src/app/dist/frankenphp-linux-x86_64 frankenphp ; docker rm static-laravel-app-tmp
   ```

4. Popule os caches:

   ```console
   frankenphp php-cli artisan optimize
   ```

5. Execute as migrações de banco de dados (se houver):

   ```console
   frankenphp php-cli artisan migrate
   ```

6. Gere a chave secreta da aplicação:

   ```console
   frankenphp php-cli artisan key:generate
   ```

7. Inicie o servidor:

   ```console
   frankenphp php-server
   ```

Agora sua aplicação está pronta!

Saiba mais sobre as opções disponíveis e como compilar binários para outros
sistemas operacionais na documentação de
[incorporação de aplicações](embed.md).

### Alterando o caminho do armazenamento

Por padrão, o Laravel armazena arquivos enviados, caches, logs, etc., no
diretório `storage/` da aplicação.
Isso não é adequado para aplicações embarcadas, pois cada nova versão será
extraída para um diretório temporário diferente.

Defina a variável de ambiente `LARAVEL_STORAGE_PATH` (por exemplo, no seu
arquivo `.env`) ou chame o método
`Illuminate\Foundation\Application::useStoragePath()` para usar um diretório
fora do diretório temporário.

### Executando o Octane com binários independentes

É possível até empacotar aplicações Octane do Laravel como binários
independentes!

Para fazer isso, [instale o Octane corretamente](#laravel-octane) e siga os
passos descritos na
[seção anterior](#aplicações-laravel-como-binários-independentes).

Em seguida, para iniciar o FrankenPHP no modo worker através do Octane, execute:

```console
PATH="$PWD:$PATH" frankenphp php-cli artisan octane:frankenphp
```

> [!CAUTION]
>
> Para que o comando funcione, o binário independente **deve** ser nomeado
> `frankenphp` porque o Octane precisa de um programa chamado `frankenphp`
> disponível no caminho.
