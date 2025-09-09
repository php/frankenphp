# Construindo uma imagem Docker personalizada

[As imagens Docker do FrankenPHP](https://hub.docker.com/r/dunglas/frankenphp)
são baseadas em [imagens oficiais do PHP](https://hub.docker.com/_/php/).
Variantes do Debian e do Alpine Linux são fornecidas para arquiteturas
populares.
Variantes do Debian são recomendadas.

Variantes para PHP 8.2, 8.3 e 8.4 são fornecidas.

As tags seguem este padrão:
`dunglas/frankenphp:<versao-do-frankenphp>-php<versao-do-php>-<so>`.

- `<versao-do-frankenphp>` e `<versao-do-php>` são números de versão do
  FrankenPHP e do PHP, respectivamente, variando de maior (ex.: `1`), menor
  (ex.: `1.2`) a versões de patch (ex.: `1.2.3`).
- `<so>` é `bookworm` (para Debian Bookworm) ou `alpine` (para a versão estável
  mais recente do Alpine).

[Navegue pelas tags](https://hub.docker.com/r/dunglas/frankenphp/tags).

## Como usar as imagens

Crie um `Dockerfile` no seu projeto:

```dockerfile
FROM dunglas/frankenphp

COPY . /app/public
```

Em seguida, execute estes comandos para construir e executar a imagem Docker:

```console
docker build -t minha-app-php .
docker run -it --rm --name minha-app-rodando minha-app-php
```

## Como instalar mais extensões PHP

O script
[`docker-php-extension-installer`](https://github.com/mlocati/docker-php-extension-installer)
é fornecido na imagem base.
Adicionar extensões PHP adicionais é simples:

```dockerfile
FROM dunglas/frankenphp

# adicione extensões adicionais aqui:
RUN install-php-extensions \
	pdo_mysql \
	gd \
	intl \
	zip \
	opcache
```

## Como instalar mais módulos Caddy

O FrankenPHP é construído sobre o Caddy, e todos os
[módulos Caddy](https://caddyserver.com/docs/modules/) podem ser usados com o
FrankenPHP.

A maneira mais fácil de instalar módulos Caddy personalizados é usar o
[xcaddy](https://github.com/caddyserver/xcaddy):

```dockerfile
FROM dunglas/frankenphp:builder AS builder

# Copia o xcaddy para a imagem do builder
COPY --from=caddy:builder /usr/bin/xcaddy /usr/bin/xcaddy

# O CGO precisa estar habilitado para compilar o FrankenPHP
RUN CGO_ENABLED=1 \
    XCADDY_SETCAP=1 \
    XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
    CGO_CFLAGS=$(php-config --includes) \
    CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
    xcaddy build \
        --output /usr/local/bin/frankenphp \
        --with github.com/php/frankenphp=./ \
        --with github.com/php/frankenphp/caddy=./caddy/ \
        --with github.com/dunglas/caddy-cbrotli \
        # Mercure e Vulcain estão incluídos na compilação oficial, mas sinta-se
        # à vontade para removê-los
        --with github.com/dunglas/mercure/caddy \
        --with github.com/dunglas/vulcain/caddy
        # Adicione módulos Caddy extras aqui

FROM dunglas/frankenphp AS runner

# Substitui o binário oficial pelo que contém seus módulos personalizados
COPY --from=builder /usr/local/bin/frankenphp /usr/local/bin/frankenphp
```

A imagem `builder` fornecida pelo FrankenPHP contém uma versão compilada da
`libphp`.
[Imagens de builder](https://hub.docker.com/r/dunglas/frankenphp/tags?name=builder)
são fornecidas para todas as versões do FrankenPHP e do PHP, tanto para Debian
quanto para Alpine.

> [!TIP]
>
> Se você estiver usando Alpine Linux e Symfony, pode ser necessário
> [aumentar o tamanho padrão da pilha](compile.md#using-xcaddy).

## Habilitando o modo worker por padrão

Defina a variável de ambiente `FRANKENPHP_CONFIG` para iniciar o FrankenPHP com
um worker script:

```dockerfile
FROM dunglas/frankenphp

# ...

ENV FRANKENPHP_CONFIG="worker ./public/index.php"
```

## Usando um volume em desenvolvimento

Para desenvolver facilmente com o FrankenPHP, monte o diretório do seu host que
contém o código-fonte da aplicação como um volume no contêiner Docker:

```console
docker run -v $PWD:/app/public -p 80:80 -p 443:443 -p 443:443/udp --tty minha-app-php
```

> [!TIP]
>
> A opção `--tty` permite ter logs legíveis por humanos em vez de logs JSON.

Com o Docker Compose:

```yaml
# compose.yaml

services:
  php:
    image: dunglas/frankenphp
    # Descomente a linha a seguir se quiser usar um Dockerfile personalizado
    #build: .
    # Descomente a linha a seguir se quiser executar isso em um ambiente de
    # produção
    # restart: always
    ports:
      - "80:80" # HTTP
      - "443:443" # HTTPS
      - "443:443/udp" # HTTP/3
    volumes:
      - ./:/app/public
      - caddy_data:/data
      - caddy_config:/config
    # Comente a linha a seguir em produção, isso permite ter logs legíveis em
    # desenvolvimento
    tty: true

# Volumes necessários para certificados e configuração do Caddy
volumes:
  caddy_data:
  caddy_config:
```

## Executando como um usuário não root

O FrankenPHP pode ser executado como um usuário não root no Docker.

Aqui está um exemplo de `Dockerfile` fazendo isso:

```dockerfile
FROM dunglas/frankenphp

ARG USER=appuser

RUN \
	# Use "adduser -D ${USER}" para distribuições baseadas em Alpine
	useradd ${USER}; \
	# Adiciona capacidade adicional para vincular às portas 80 e 443
	setcap CAP_NET_BIND_SERVICE=+eip /usr/local/bin/frankenphp; \
	# Concede acesso de escrita a /config/caddy e /data/caddy
	chown -R ${USER}:${USER} /config/caddy /data/caddy

USER ${USER}
```

### Executando sem capacidades adicionais

Mesmo executando sem root, o FrankenPHP precisa do recurso
`CAP_NET_BIND_SERVICE` para vincular o servidor web em portas privilegiadas (80
e 443).

Se você expor o FrankenPHP em uma porta não privilegiada (1024 e acima), é
possível executar o servidor web como um usuário não root e sem a necessidade de
nenhuma capacidade adicional:

```dockerfile
FROM dunglas/frankenphp

ARG USER=appuser

RUN \
	# Use "adduser -D ${USER}" para distribuições baseadas em Alpine
	useradd ${USER}; \
	# Remove a capacidade padrão
	setcap -r /usr/local/bin/frankenphp; \
	# Concede acesso de escrita a /config/caddy e /data/caddy
	chown -R ${USER}:${USER} /config/caddy /data/caddy

USER ${USER}
```

Em seguida, defina a variável de ambiente `SERVER_NAME` para usar uma porta sem
privilégios.
Exemplo: `:8000`

## Atualizações

As imagens Docker são construídas:

- Quando uma tag de uma nova versão é criada;
- Diariamente às 4h UTC, se novas versões das imagens oficiais do PHP estiverem
  disponíveis.

## Versões de desenvolvimento

As versões de desenvolvimento estão disponíveis no repositório Docker
[`dunglas/frankenphp-dev`](https://hub.docker.com/repository/docker/dunglas/frankenphp-dev).
Uma nova construção é acionada sempre que um commit é enviado para o branch
principal do repositório do GitHub.

As tags `latest*` apontam para o HEAD do branch `main`.
Tags no formato `sha-<git-commit-hash>` também estão disponíveis.
