# Contribuindo

## Compilando o PHP

### Com Docker (Linux)

Crie a imagem Docker de desenvolvimento:

```console
docker build -t frankenphp-dev -f dev.Dockerfile .
docker run --cap-add=SYS_PTRACE --security-opt seccomp=unconfined -p 8080:8080 -p 443:443 -p 443:443/udp -v $PWD:/go/src/app -it frankenphp-dev
```

A imagem contém as ferramentas de desenvolvimento usuais (Go, GDB, Valgrind,
Neovim...) e usa os seguintes locais de configuração do PHP:

- php.ini: `/etc/frankenphp/php.ini`.
  Um arquivo `php.ini` com configurações de desenvolvimento é fornecido por
  padrão.
- Arquivos de configuração adicionais: `/etc/frankenphp/php.d/*.ini`.
- Extensões PHP: `/usr/lib/frankenphp/modules/`.

Se a sua versão do Docker for anterior à 23.0, a compilação falhará devido ao
[problema de padrão do `.dockerignore`](https://github.com/moby/moby/pull/42676).
Adicione diretórios ao `.dockerignore`.

```patch
 !testdata/*.php
 !testdata/*.txt
+!caddy
+!internal
```

### Sem Docker (Linux e macOS)

[Siga as instruções para compilar a partir do código-fonte](compile.md) e passe
a flag de configuração `--debug`.

## Executando a suite de testes

```console
go test -tags watcher -race -v ./...
```

## Módulo Caddy

Construa o Caddy com o módulo Caddy FrankenPHP:

```console
cd caddy/frankenphp/
go build -tags watcher,brotli,nobadger,nomysql,nopgx
cd ../../
```

Execute o Caddy com o módulo Caddy FrankenPHP:

```console
cd testdata/
../caddy/frankenphp/frankenphp run
```

O servidor está escutando em `127.0.0.1:80`:

> [!NOTE]
> Se você estiver usando o Docker, terá que vincular a porta 80 do contêiner ou
> executar de dentro do contêiner.

```console
curl -vk http://127.0.0.1/phpinfo.php
```

## Servidor de teste mínimo

Construa o servidor de teste mínimo:

```console
cd internal/testserver/
go build
cd ../../
```

Execute o servidor de teste:

```console
cd testdata/
../internal/testserver/testserver
```

O servidor está escutando em `127.0.0.1:8080`:

```console
curl -v http://127.0.0.1:8080/phpinfo.php
```

## Construindo imagens Docker localmente

Imprima o plano do bake:

```console
docker buildx bake -f docker-bake.hcl --print
```

Construa imagens FrankenPHP para amd64 localmente:

```console
docker buildx bake -f docker-bake.hcl --pull --load --set "*.platform=linux/amd64"
```

Construa imagens FrankenPHP para arm64 localmente:

```console
docker buildx bake -f docker-bake.hcl --pull --load --set "*.platform=linux/arm64"
```

Construa imagens FrankenPHP do zero para arm64 e amd64 e envie para o Docker
Hub:

```console
docker buildx bake -f docker-bake.hcl --pull --no-cache --push
```

## Depurando falhas de segmentação com compilações estáticas

1. Baixe a versão de depuração do binário do FrankenPHP do GitHub ou crie sua
   própria compilação estática personalizada, incluindo símbolos de depuração:

   ```console
   docker buildx bake \
       --load \
       --set static-builder.args.DEBUG_SYMBOLS=1 \
       --set "static-builder.platform=linux/amd64" \
       static-builder
   docker cp $(docker create --name static-builder-musl dunglas/frankenphp:static-builder-musl):/go/src/app/dist/frankenphp-linux-$(uname -m) frankenphp
   ```

2. Substitua sua versão atual do `frankenphp` pelo executável de depuração do
   FrankenPHP.
3. Inicie o FrankenPHP normalmente (alternativamente, você pode iniciar o
   FrankenPHP diretamente com o GDB: `gdb --args frankenphp run`).
4. Anexe ao processo com o GDB:

   ```console
   gdb -p `pidof frankenphp`
   ```

5. Se necessário, digite `continue` no shell do GDB.
6. Faça o FrankenPHP travar.
7. Digite `bt` no shell do GDB.
8. Copie a saída.

## Depurando falhas de segmentação no GitHub Actions

1. Abra o arquivo `.github/workflows/tests.yml`.
2. Habilite os símbolos de depuração do PHP:

   ```patch
       - uses: shivammathur/setup-php@v2
         # ...
         env:
           phpts: ts
   +       debug: true
   ```

3. Habilite o `tmate` para se conectar ao contêiner:

   ```patch
       - name: Set CGO flags
         run: echo "CGO_CFLAGS=$(php-config --includes)" >> "$GITHUB_ENV"
   +   - run: |
   +       sudo apt install gdb
   +       mkdir -p /home/runner/.config/gdb/
   +       printf "set auto-load safe-path /\nhandle SIG34 nostop noprint pass" > /home/runner/.config/gdb/gdbinit
   +   - uses: mxschmitt/action-tmate@v3
   ```

4. Conecte-se ao contêiner.
5. Abra o `frankenphp.go`.
6. Habilite o `cgosymbolizer`:

   ```patch
   -	//_ "github.com/ianlancetaylor/cgosymbolizer"
   +	_ "github.com/ianlancetaylor/cgosymbolizer"
   ```

7. Baixe o módulo: `go get`.
8. No contêiner, você pode usar o GDB e similares:

   ```console
   go test -tags watcher -c -ldflags=-w
   gdb --args frankenphp.test -test.run ^MyTest$
   ```

9. Quando a falha for corrigida, reverta todas essas alterações.

## Recursos diversos de desenvolvimento

- [PHP embedding in uWSGI](https://github.com/unbit/uwsgi/blob/master/plugins/php/php_plugin.c)
- [PHP embedding in NGINX Unit](https://github.com/nginx/unit/blob/master/src/nxt_php_sapi.c)
- [PHP embedding in Go (go-php)](https://github.com/deuill/go-php)
- [PHP embedding in Go (GoEmPHP)](https://github.com/mikespook/goemphp)
- [PHP embedding in C++](https://gist.github.com/paresy/3cbd4c6a469511ac7479aa0e7c42fea7)
- [Extending and Embedding PHP, por Sara Golemon](https://books.google.fr/books?id=zMbGvK17_tYC&pg=PA254&lpg=PA254#v=onepage&q&f=false)
- [What the heck is TSRMLS_CC, anyway?](http://blog.golemon.com/2006/06/what-heck-is-tsrmlscc-anyway.html)
- [SDL bindings](https://pkg.go.dev/github.com/veandco/go-sdl2@v0.4.21/sdl#Main)

## Recursos relacionados ao Docker

- [Definição do arquivo Bake](https://docs.docker.com/build/customize/bake/file-definition/)
- [`docker buildx build`](https://docs.docker.com/engine/reference/commandline/buildx_build/)

## Comando útil

```console
apk add strace util-linux gdb
strace -e 'trace=!futex,epoll_ctl,epoll_pwait,tgkill,rt_sigreturn' -p 1
```

## Traduzindo a documentação

Para traduzir a documentação e o site para um novo idioma, siga estes passos:

1. Crie um novo diretório com o código ISO de 2 caracteres do idioma no
   diretório `docs/` deste repositório.
2. Copie todos os arquivos `.md` da raiz do diretório `docs/` para o novo
   diretório (sempre use a versão em inglês como fonte para tradução, pois está
   sempre atualizada).
3. Copie os arquivos `README.md` e `CONTRIBUTING.md` do diretório raiz para o
   novo diretório.
4. Traduza o conteúdo dos arquivos, mas não altere os nomes dos arquivos, nem
   traduza strings que comecem com `> [!` (é uma marcação especial para o
   GitHub).
5. Crie um pull request com as traduções.
6. No
   [repositório do site](https://github.com/dunglas/frankenphp-website/tree/main),
   copie e traduza os arquivos de tradução nos diretórios `content/`, `data/` e
   `i18n/`.
7. Traduza os valores no arquivo YAML criado.
8. Abra um pull request no repositório do site.
