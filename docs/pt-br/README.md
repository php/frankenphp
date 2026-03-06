# FrankenPHP: um moderno servidor de aplicações para PHP

<h1 align="center"><a href="https://frankenphp.dev/pt-br"><img src="frankenphp.png" alt="FrankenPHP" width="600"></a></h1>

O FrankenPHP é um moderno servidor de aplicações para PHP, construído sobre o
servidor web [Caddy](https://caddyserver.com/).

O FrankenPHP dá superpoderes às suas aplicações PHP graças aos seus recursos
impressionantes: [_Early Hints_](early-hints.md), [modo worker](worker.md),
[recursos em tempo real](mercure.md), suporte automático a HTTPS, HTTP/2 e
HTTP/3...

O FrankenPHP funciona com qualquer aplicação PHP e torna seus projetos Laravel e
Symfony mais rápidos do que nunca, graças às suas integrações oficiais com o
modo worker.

O FrankenPHP também pode ser usado como uma biblioteca Go independente para
incorporar PHP em qualquer aplicação usando `net/http`.

[**Saiba mais** em _frankenphp.dev_](https://frankenphp.dev/pt-br) e neste
conjunto de slides:

<a href="https://dunglas.dev/2022/10/frankenphp-the-modern-php-app-server-written-in-go/"><img src="https://dunglas.dev/wp-content/uploads/2022/10/frankenphp.png" alt="Slides" width="600"></a>

## Começando

No Windows, use [WSL](https://learn.microsoft.com/pt-br/windows/wsl/) para
executar o FrankenPHP.

### Script de instalação

Você pode copiar esta linha no seu terminal para instalar automaticamente a
versão apropriada para sua plataforma:

```console
curl https://frankenphp.dev/install.sh | sh
```

### Binário independente

Fornecemos binários estáticos do FrankenPHP para desenvolvimento em Linux e macOS contendo o
[PHP 8.4](https://www.php.net/releases/8.4/pt_BR.php) e as extensões PHP mais populares.

[Baixe o FrankenPHP](https://github.com/php/frankenphp/releases)

**Instalação de extensões:** As extensões mais comuns já estão incluídas. Não é possível instalar mais extensões.

### Pacotes rpm

Nossos mantenedores oferecem pacotes rpm para todos os sistemas que usam `dnf`. Para instalar, execute:

```console
sudo dnf install https://rpm.henderkes.com/static-php-1-0.noarch.rpm
sudo dnf module enable php-zts:static-8.4 # 8.2-8.5 disponíveis
sudo dnf install frankenphp
```

**Instalação de extensões:** `sudo dnf install php-zts-<extension>`

Para extensões não disponíveis por padrão, use o [PIE](https://github.com/php/pie):

```console
sudo dnf install pie-zts
sudo pie-zts install asgrim/example-pie-extension
```

### Pacotes deb

Nossos mantenedores oferecem pacotes deb para todos os sistemas que usam `apt`. Para instalar, execute:

```console
sudo curl -fsSL https://key.henderkes.com/static-php.gpg -o /usr/share/keyrings/static-php.gpg && \
echo "deb [signed-by=/usr/share/keyrings/static-php.gpg] https://deb.henderkes.com/ stable main" | sudo tee /etc/apt/sources.list.d/static-php.list && \
sudo apt update
sudo apt install frankenphp
```

**Instalação de extensões:** `sudo apt install php-zts-<extension>`

Para extensões não disponíveis por padrão, use o [PIE](https://github.com/php/pie):

```console
sudo apt install pie-zts
sudo pie-zts install asgrim/example-pie-extension
```

Para servir o conteúdo do diretório atual, execute:

```console
frankenphp php-server
```

Você também pode executar scripts de linha de comando com:

```console
frankenphp php-cli /caminho/para/seu/script.php
```

### Docker

Alternativamente, [imagens Docker](docker.md) estão disponíveis:

```console
docker run -v .:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

Acesse `https://localhost` e divirta-se!

> [!TIP]
>
> Não tente usar `https://127.0.0.1`.
> Use `https://localhost` e aceite o certificado autoassinado.
> Use a
> [variável de ambiente `SERVER_NAME`](config.md#variaveis-de-ambiente)
> para alterar o domínio a ser usado.

### Homebrew

O FrankenPHP também está disponível como um pacote [Homebrew](https://brew.sh)
para macOS e Linux.

Para instalá-lo:

```console
brew install dunglas/frankenphp/frankenphp
```

**Instalação de extensões:** Use o [PIE](https://github.com/php/pie).

### Uso

Para servir o conteúdo do diretório atual, execute:

```console
frankenphp php-server
```

Você também pode executar scripts de linha de comando com:

```console
frankenphp php-cli /caminho/para/seu/script.php
```

Para os pacotes deb e rpm, você também pode iniciar o serviço systemd:

```console
sudo systemctl start frankenphp
```

## Documentação

- [Modo clássico](classic.md)
- [Modo worker](worker.md)
- [Suporte a Early Hints (código de status HTTP 103)](early-hints.md)
- [Tempo real](mercure.md)
- [Servindo grandes arquivos estáticos com eficiência](x-sendfile.md)
- [Configuração](config.md)
- [Escrevendo extensões PHP em Go](extensions.md)
- [Imagens Docker](docker.md)
- [Implantação em produção](production.md)
- [Otimização de desempenho](performance.md)
- [Crie aplicações PHP **independentes** e autoexecutáveis](embed.md)
- [Crie binários estáticos](static.md)
- [Compile a partir do código-fonte](compile.md)
- [Monitorando o FrankenPHP](metrics.md)
- [Integração com Laravel](laravel.md)
- [Problemas conhecidos](known-issues.md)
- [Aplicação de demonstração (Symfony) e benchmarks](https://github.com/dunglas/frankenphp-demo)
- [Documentação da biblioteca Go](https://pkg.go.dev/github.com/php/frankenphp)
- [Contribuindo e depurando](CONTRIBUTING.md)

## Exemplos e esqueletos

- [Symfony](https://github.com/dunglas/symfony-docker)
- [API Platform](https://api-platform.com/docs/symfony)
- [Laravel](laravel.md)
- [Sulu](https://sulu.io/blog/running-sulu-with-frankenphp)
- [WordPress](https://github.com/StephenMiracle/frankenwp)
- [Drupal](https://github.com/dunglas/frankenphp-drupal)
- [Joomla](https://github.com/alexandreelise/frankenphp-joomla)
- [TYPO3](https://github.com/ochorocho/franken-typo3)
- [Magento2](https://github.com/ekino/frankenphp-magento2)
