# Implantando em produção

Neste tutorial, aprenderemos como implantar uma aplicação PHP em um único
servidor usando o Docker Compose.

Se você estiver usando o Symfony, prefira ler a entrada de documentação
[Implantar em produção](https://github.com/dunglas/symfony-docker/blob/main/docs/production.md)
do projeto Docker do Symfony (que usa FrankenPHP).

Se você estiver usando a API Platform (que também usa FrankenPHP), consulte
[a documentação de implantação do framework](https://api-platform.com/docs/deployment/).

## Preparando sua aplicação

Primeiro, crie um `Dockerfile` no diretório raiz do seu projeto PHP:

```dockerfile
FROM dunglas/frankenphp

# Certifique-se de substituir "seu-nome-de-dominio.example.com" pelo seu nome de
# domínio
ENV SERVER_NAME=seu-nome-de-dominio.example.com
# Se quiser desabilitar o HTTPS, use este valor:
#ENV SERVER_NAME=:80

# Habilita as configurações de produção do PHP
RUN mv "$PHP_INI_DIR/php.ini-production" "$PHP_INI_DIR/php.ini"

# Copia os arquivos PHP do seu projeto para o diretório public
COPY . /app/public
# Se você usa Symfony ou Laravel, precisa copiar o projeto inteiro:
#COPY . /app
```

Consulte [Criando uma imagem Docker personalizada](docker.md) para mais detalhes
e opções, e para aprender como personalizar a configuração, instalar extensões
PHP e módulos Caddy.

Se o seu projeto usa o Composer, certifique-se de incluí-lo na imagem Docker e
instalar suas dependências.

Em seguida, adicione um arquivo `compose.yaml`:

```yaml
services:
  php:
    image: dunglas/frankenphp
    restart: always
    ports:
      - "80:80" # HTTP
      - "443:443" # HTTPS
      - "443:443/udp" # HTTP/3
    volumes:
      - caddy_data:/data
      - caddy_config:/config

# Volumes necessários para certificados e configuração do Caddy
volumes:
  caddy_data:
  caddy_config:
```

> [!NOTE]
>
> Os exemplos anteriores são destinados ao uso em produção.
> Em desenvolvimento, você pode querer usar um volume, uma configuração PHP
> diferente e um valor diferente para a variável de ambiente `SERVER_NAME`.
>
> Consulte o projeto [Symfony Docker](https://github.com/dunglas/symfony-docker)
> (que usa FrankenPHP) para um exemplo mais avançado usando imagens
> multiestágio, Composer, extensões PHP extras, etc.

Finalmente, se você usa Git, faça o commit e o push desses arquivos.

## Preparando um servidor

Para implantar sua aplicação em produção, você precisa de um servidor.
Neste tutorial, usaremos uma máquina virtual fornecida pela DigitalOcean, mas
qualquer servidor Linux pode funcionar.
Se você já possui um servidor Linux com o Docker instalado, pode pular direto
para [a próxima seção](#configurando-um-nome-de-domínio).

Caso contrário, use [este link de afiliado](https://m.do.co/c/5d8aabe3ab80) para
obter US$ 200 em créditos gratuitos, crie uma conta e clique em "Create a
Droplet".
Em seguida, clique na aba "Marketplace" na seção "Choose an image" e procure a
aplicação "Docker".
Isso provisionará um servidor Ubuntu com as versões mais recentes do Docker e do
Docker Compose já instaladas!

Para fins de teste, os planos mais baratos serão suficientes.
Para uso real em produção, você provavelmente escolherá um plano na seção
"General Purpose" que atenda às suas necessidades.

![Implantando o FrankenPHP na DigitalOcean com Docker](digitalocean-droplet.png)

Você pode manter os padrões para outras configurações ou ajustá-los de acordo
com suas necessidades.
Não se esqueça de adicionar sua chave SSH ou criar uma senha e, em seguida,
clicar no botão "Finalize and create".

Em seguida, aguarde alguns segundos enquanto seu Droplet é provisionado.
Quando seu Droplet estiver pronto, use SSH para se conectar:

```console
ssh root@<droplet-ip>
```

## Configurando um nome de domínio

Na maioria dos casos, você precisará associar um nome de domínio ao seu site.
Se você ainda não possui um nome de domínio, precisará comprar um por meio de um
registrar.

Em seguida, crie um registro DNS do tipo `A` para o seu nome de domínio,
apontando para o endereço IP do seu servidor:

```dns
seu-nome-de-dominio.example.com.  IN  A  <ip-do-seu-servidor>
```

Exemplo com o serviço DigitalOcean Domains ("Networking" > "Domains"):

![Configurando DNS na DigitalOcean](digitalocean-dns.png)

> [!NOTE]
>
> O Let's Encrypt, o serviço usado por padrão pelo FrankenPHP para gerar
> automaticamente um certificado TLS, não suporta o uso de endereços IP.
> O uso de um nome de domínio é obrigatório para usar o Let's Encrypt.

## Implantando

Copie seu projeto para o servidor usando `git clone`, `scp` ou qualquer outra
ferramenta que atenda às suas necessidades.
Se você usa o GitHub, pode ser útil usar
[uma chave de implantação](https://docs.github.com/en/free-pro-team@latest/developers/overview/managing-deploy-keys#deploy-keys).
Chaves de implantação também são [suportadas pelo GitLab](https://docs.gitlab.com/ee/user/project/deploy_keys/).

Exemplo com Git:

```console
git clone git@github.com:<usuario>/<nome-do-projeto>.git
```

Acesse o diretório que contém seu projeto (`<nome-do-projeto>`) e inicie a
aplicação em modo de produção:

```console
docker compose up --wait
```

Seu servidor está funcionando e um certificado HTTPS foi gerado automaticamente
para você.
Acesse `https://seu-nome-de-dominio.example.com` e divirta-se!

> [!CAUTION]
>
> O Docker pode ter uma camada de cache; certifique-se de ter a compilação
> correta para cada implantação ou reconstrua seu projeto com a opção
> `--no-cache` para evitar problemas de cache.

## Implantando em múltiplos nós

Se você deseja implantar sua aplicação em um cluster de máquinas, pode usar o
[Docker Swarm](https://docs.docker.com/engine/swarm/stack-deploy/), que é
compatível com os arquivos Compose fornecidos.
Para implantar no Kubernetes, consulte o
[chart do Helm fornecido com a API Platform](https://api-platform.com/docs/deployment/kubernetes/),
que usa FrankenPHP.
