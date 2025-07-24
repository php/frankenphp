# Compilar a partir dos fontes

Este documento explica como criar um binário FrankenPHP que carregará o PHP como
uma biblioteca dinâmica.
Este é o método recomendado.

Como alternativa, [compilações totalmente e principalmente estáticas](static.md)
também podem ser criadas.

## Instalar o PHP

O FrankenPHP é compatível com PHP 8.2 e versões superiores.

### Com o Homebrew (Linux e Mac)

A maneira mais fácil de instalar uma versão da `libphp` compatível com o
FrankenPHP é usar os pacotes ZTS fornecidos pelo
[Homebrew PHP](https://github.com/shivammathur/homebrew-php).

Primeiro, se ainda não o fez, instale o [Homebrew](https://brew.sh).

Em seguida, instale a variante ZTS do PHP, o Brotli (opcional, para suporte à
compressão) e o watcher (opcional, para detecção de alterações em arquivos):

```console
brew install shivammathur/php/php-zts brotli watcher
brew link --overwrite --force shivammathur/php/php-zts
```

### Compilando o PHP

Alternativamente, você pode compilar o PHP a partir dos códigos-fonte com as
opções necessárias para o FrankenPHP seguindo estes passos.

Primeiro, [obtenha os códigos-fonte do PHP](https://www.php.net/downloads.php) e
extraia-os:

```console
tar xf php-*
cd php-*/
```

Em seguida, execute o script `configure` com as opções necessárias para sua
plataforma.
As seguintes flags `./configure` são obrigatórias, mas você pode adicionar
outras, por exemplo, para compilar extensões ou recursos adicionais.

#### Linux

```console
./configure \
    --enable-embed \
    --enable-zts \
    --disable-zend-signals \
    --enable-zend-max-execution-timers
```

#### Mac

Use o gerenciador de pacotes [Homebrew](https://brew.sh/) para instalar as
dependências necessárias e opcionais:

```console
brew install libiconv bison brotli re2c pkg-config watcher
echo 'export PATH="/opt/homebrew/opt/bison/bin:$PATH"' >> ~/.zshrc
```

Em seguida, execute o script `configure`:

```console
./configure \
    --enable-embed \
    --enable-zts \
    --disable-zend-signals \
    --with-iconv=/opt/homebrew/opt/libiconv/
```

#### Compilar o PHP

Finalmente, compile e instale o PHP:

```console
make -j"$(getconf _NPROCESSORS_ONLN)"
sudo make install
```

## Instalar dependências opcionais

Alguns recursos do FrankenPHP dependem de dependências opcionais do sistema que
devem ser instaladas.
Alternativamente, esses recursos podem ser desabilitados passando as tags de
compilação para o compilador Go.

| Recurso                                | Dependência                                                           | Tag de compilação para desabilitá-lo |
|----------------------------------------|-----------------------------------------------------------------------|--------------------------------------|
| Compressão Brotli                      | [Brotli](https://github.com/google/brotli)                            | `nobrotli`                           |
| Reiniciar workers ao alterar o arquivo | [Watcher C](https://github.com/e-dant/watcher/tree/release/watcher-c) | `nowatcher`                          |

## Compilando a aplicação Go

Agora você pode compilar o binário final.

### Usando o `xcaddy`

A maneira recomendada é usar o [`xcaddy`](https://github.com/caddyserver/xcaddy)
para compilar o FrankenPHP.
O `xcaddy` também permite adicionar facilmente
[módulos Caddy personalizados](https://caddyserver.com/docs/modules/) e
extensões FrankenPHP:

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/dunglas/frankenphp/caddy \
    --with github.com/dunglas/mercure/caddy \
    --with github.com/dunglas/vulcain/caddy
    # Adicione módulos Caddy e extensões FrankenPHP extras aqui
```

> [!TIP]
>
> Se você estiver usando a `libc` `musl` (o padrão no Alpine Linux) e Symfony,
> pode ser necessário aumentar o tamanho da pilha padrão.
> Caso contrário, você poderá receber erros como `PHP Fatal error: Maximum call
> stack size of 83360 bytes reached during compilation.
> Try splitting expression`.
>
> Para fazer isso, altere a variável de ambiente `XCADDY_GO_BUILD_FLAGS` para
> algo como
> `XCADDY_GO_BUILD_FLAGS=$'-ldflags "-w -s -extldflags \'-Wl,-z,stack-size=0x80000\'"'`
> (altere o valor do tamanho da pilha de acordo com as necessidades da sua
> aplicação).

### Sem o `xcaddy`

Alternativamente, é possível compilar o FrankenPHP sem o `xcaddy` usando o
comando `go` diretamente:

```console
curl -L https://github.com/dunglas/frankenphp/archive/refs/heads/main.tar.gz | tar xz
cd frankenphp-main/caddy/frankenphp
CGO_CFLAGS=$(php-config --includes) CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" go build -tags=nobadger,nomysql,nopgx
```
