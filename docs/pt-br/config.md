# Configuração

FrankenPHP, Caddy, bem como os módulos [Mercure](mercure.md) e [Vulcain](https://vulcain.rocks), podem ser configurados usando [os formatos suportados pelo Caddy](https://caddyserver.com/docs/getting-started#your-first-config).

O formato mais comum é o `Caddyfile`, que é um formato de texto simples e legível por humanos.
Por padrão, o FrankenPHP procurará por um `Caddyfile` no diretório atual.
Você pode especificar um caminho personalizado com a opção `-c` ou `--config`.

Um `Caddyfile` mínimo para servir uma aplicação PHP é mostrado abaixo:

```caddyfile
# The hostname to respond to
localhost

# Optionally, the directory to serve files from, otherwise defaults to the current directory
#root public/
php_server
```

Um `Caddyfile` mais avançado, que habilita mais recursos e fornece variáveis de ambiente convenientes, é disponibilizado [no repositório FrankenPHP](https://github.com/php/frankenphp/blob/main/caddy/frankenphp/Caddyfile),
e com as imagens Docker.

O próprio PHP pode ser configurado [usando um arquivo `php.ini`](https://www.php.net/manual/en/configuration.file.php).

Dependendo do seu método de instalação, o FrankenPHP e o interpretador PHP procurarão por arquivos de configuração nos locais descritos abaixo.

## Docker

FrankenPHP:

- `/etc/frankenphp/Caddyfile`: o arquivo de configuração principal
- `/etc/frankenphp/Caddyfile.d/*.caddyfile`: arquivos de configuração adicionais que são carregados automaticamente

PHP:

- `php.ini`: `/usr/local/etc/php/php.ini` (nenhum `php.ini` é fornecido por padrão)
- arquivos de configuração adicionais: `/usr/local/etc/php/conf.d/*.ini`
- extensões PHP: `/usr/local/lib/php/extensions/no-debug-zts-<YYYYMMDD>/`
- Você deve copiar um modelo oficial fornecido pelo projeto PHP:

```dockerfile
FROM dunglas/frankenphp

# Produção:
RUN cp $PHP_INI_DIR/php.ini-production $PHP_INI_DIR/php.ini

# Ou desenvolvimento:
RUN cp $PHP_INI_DIR/php.ini-development $PHP_INI_DIR/php.ini
```

## Pacotes RPM e Debian

FrankenPHP:

- `/etc/frankenphp/Caddyfile`: o arquivo de configuração principal
- `/etc/frankenphp/Caddyfile.d/*.caddyfile`: arquivos de configuração adicionais que são carregados automaticamente

PHP:

- `php.ini`: `/etc/php-zts/php.ini` (um arquivo `php.ini` com predefinições de produção é fornecido por padrão)
- arquivos de configuração adicionais: `/etc/php-zts/conf.d/*.ini`

## Binário estático

FrankenPHP:

- No diretório de trabalho atual: `Caddyfile`

PHP:

- `php.ini`: O diretório no qual `frankenphp run` ou `frankenphp php-server` é executado, e então `/etc/frankenphp/php.ini`
- arquivos de configuração adicionais: `/etc/frankenphp/php.d/*.ini`
- extensões PHP: não podem ser carregadas, inclua-as no próprio binário
- copie um dos arquivos `php.ini-production` ou `php.ini-development` fornecidos [nas fontes do PHP](https://github.com/php/php-src/).

## Configuração do Caddyfile

As [diretivas HTTP](https://caddyserver.com/docs/caddyfile/concepts#directives) `php_server` ou `php` podem ser usadas dentro dos blocos de site para servir sua aplicação PHP.

Exemplo mínimo:

```caddyfile
localhost {
	# Habilita compressão (opcional)
	encode zstd br gzip
	# Executa arquivos PHP no diretório atual e serve assets
	php_server
}
```

Você também pode configurar explicitamente o FrankenPHP usando a [opção global](https://caddyserver.com/docs/caddyfile/concepts#global-options) `frankenphp`:

```caddyfile
{
	frankenphp {
		num_threads <num_threads> # Define o número de threads PHP a serem iniciadas. Padrão: 2x o número de CPUs disponíveis.
		max_threads <num_threads> # Limita o número de threads PHP adicionais que podem ser iniciadas em tempo de execução. Padrão: num_threads. Pode ser definido como 'auto'.
		max_wait_time <duration> # Define o tempo máximo que uma requisição pode esperar por uma thread PHP livre antes de atingir o tempo limite. Padrão: desabilitado.
		max_idle_time <duration> # Define o tempo máximo que uma thread autoscaled pode ficar ociosa antes de ser desativada. Padrão: 5s.
		php_ini <key> <value> # Define uma diretiva php.ini. Pode ser usada várias vezes para definir múltiplas diretivas.
		worker {
			file <path> # Define o caminho para o worker script.
			num <num> # Define o número de threads PHP a serem iniciadas, o padrão é 2x o número de CPUs disponíveis.
			env <key> <value> # Define uma variável de ambiente extra para o valor fornecido. Pode ser especificada mais de uma vez para múltiplas variáveis de ambiente.
			watch <path> # Define o caminho para monitorar alterações em arquivos. Pode ser especificada mais de uma vez para múltiplos caminhos.
			name <name> # Define o nome do worker, usado em logs e métricas. Padrão: caminho absoluto do arquivo do worker
			max_consecutive_failures <num> # Define o número máximo de falhas consecutivas antes do worker ser considerado não saudável. -1 significa que o worker sempre reiniciará. Padrão: 6.
		}
	}
}

# ...
```

Alternativamente, você pode usar a forma abreviada de uma linha da opção `worker`:

```caddyfile
{
	frankenphp {
		worker <file> <num>
	}
}

# ...
```

Você também pode definir múltiplos workers se servir múltiplas aplicações no mesmo servidor:

```caddyfile
app.example.com {
    root /path/to/app/public
	php_server {
		root /path/to/app/public # permite melhor cache
		worker index.php <num>
	}
}

other.example.com {
    root /path/to/other/public
	php_server {
		root /path/to/other/public
		worker index.php <num>
	}
}

# ...
```

Usar a diretiva `php_server` é geralmente o que você precisa,
mas se precisar de controle total, você pode usar a diretiva `php` de mais baixo nível.
A diretiva `php` passa toda a entrada para o PHP, em vez de primeiro verificar
se é um arquivo PHP ou não. Leia mais sobre isso na [página de desempenho](performance.md#try_files).

Usar a diretiva `php_server` é equivalente a esta configuração:

```caddyfile
route {
	# Adiciona barra final para requisições de diretório
	@canonicalPath {
		file {path}/index.php
		not path */
	}
	redir @canonicalPath {path}/ 308
	# Se o arquivo requisitado não existir, tenta os arquivos index
	@indexFiles file {
		try_files {path} {path}/index.php index.php
		split_path .php
	}
	rewrite @indexFiles {http.matchers.file.relative}
	# FrankenPHP!
	@phpFiles path *.php
	php @phpFiles
	file_server
}
```

As diretivas `php_server` e `php` têm as seguintes opções:

```caddyfile
php_server [<matcher>] {
	root <directory> # Define a pasta raiz para o site. Padrão: diretiva `root`.
	split_path <delim...> # Define as substrings para dividir o URI em duas partes. A primeira substring correspondente será usada para separar as "informações de caminho" do caminho. A primeira parte é sufixada com a substring correspondente e será assumida como o nome real do recurso (script CGI). A segunda parte será definida como PATH_INFO para o script usar. Padrão: `.php`
	resolve_root_symlink false # Desabilita a resolução do diretório `root` para seu valor real avaliando um link simbólico, se houver (habilitado por padrão).
	env <key> <value> # Define uma variável de ambiente extra para o valor fornecido. Pode ser especificada mais de uma vez para múltiplas variáveis de ambiente.
	file_server off # Desabilita a diretiva interna file_server.
	worker { # Cria um worker específico para este servidor. Pode ser especificada mais de uma vez para múltiplos workers.
		file <path> # Define o caminho para o worker script, pode ser relativo à raiz do php_server
		num <num> # Define o número de threads PHP a serem iniciadas, o padrão é 2x o número de CPUs disponíveis.
		name <name> # Define o nome para o worker, usado em logs e métricas. Padrão: caminho absoluto do arquivo do worker. Sempre começa com m# quando definido em um bloco php_server.
		watch <path> # Define o caminho para monitorar alterações em arquivos. Pode ser especificada mais de uma vez para múltiplos caminhos.
		env <key> <value> # Define uma variável de ambiente extra para o valor fornecido. Pode ser especificada mais de uma vez para múltiplas variáveis de ambiente. As variáveis de ambiente para este worker também são herdadas do pai do php_server, mas podem ser sobrescritas aqui.
		match <path> # Corresponde o worker a um padrão de caminho. Sobrescreve try_files e só pode ser usada na diretiva php_server.
	}
	worker <other_file> <num> # Também pode usar a forma abreviada, como no bloco global frankenphp.
}
```

### Monitorando alterações em arquivos

Como os workers inicializam sua aplicação apenas uma vez e a mantêm na memória,
quaisquer alterações nos seus arquivos PHP não serão refletidas imediatamente.

Os workers podem ser reiniciados em caso de alterações em arquivos por meio da diretiva `watch`.
Isso é útil para ambientes de desenvolvimento.

```caddyfile
{
	frankenphp {
		worker {
			file  /path/to/app/public/worker.php
			watch
		}
	}
}
```

Esta funcionalidade é frequentemente usada em combinação com [recarregamento a quente (hot reload)](hot-reload.md).

Se o diretório `watch` não for especificado, ele usará o valor padrão `./**/*.{env,php,twig,yaml,yml}`,
que monitora todos os arquivos `.env`, `.php`, `.twig`, `.yaml` e `.yml` no diretório e subdiretórios
onde o processo FrankenPHP foi iniciado. Você também pode especificar um ou mais diretórios por meio de um
[padrão de nome de arquivo shell](https://pkg.go.dev/path/filepath#Match):

```caddyfile
{
	frankenphp {
		worker {
			file  /path/to/app/public/worker.php
			watch /path/to/app # monitora todos os arquivos em todos os subdiretórios de /path/to/app
			watch /path/to/app/*.php # monitora arquivos terminados em .php em /path/to/app
			watch /path/to/app/**/*.php # monitora arquivos PHP em /path/to/app e subdiretórios
			watch /path/to/app/**/*.{php,twig} # monitora arquivos PHP e Twig em /path/to/app e subdiretórios
		}
	}
}
```

- O padrão `**` significa monitoramento recursivo
- Diretórios também podem ser relativos (ao local de início do processo FrankenPHP)
- Se você tiver múltiplos workers definidos, todos eles serão reiniciados quando um arquivo for alterado
- Tenha cuidado ao monitorar arquivos que são criados em tempo de execução (como logs), pois eles podem causar reinicializações indesejadas de workers.

O monitor de arquivos é baseado em [e-dant/watcher](https://github.com/e-dant/watcher).

## Correspondendo o worker a um caminho

Em aplicações PHP tradicionais, scripts são sempre colocados no diretório público.
Isso também é verdade para worker scripts, que são tratados como qualquer outro script PHP.
Se você quiser, em vez disso, colocar o worker script fora do diretório público, pode fazê-lo via a diretiva `match`.

A diretiva `match` é uma alternativa otimizada para `try_files`, disponível apenas dentro de `php_server` e `php`.
O exemplo a seguir sempre servirá um arquivo no diretório público, se presente,
e, caso contrário, encaminhará a requisição para o worker que corresponde ao padrão de caminho.

```caddyfile
{
	frankenphp {
		php_server {
			worker {
				file /path/to/worker.php # o arquivo pode estar fora do caminho público
				match /api/* # todas as requisições que começam com /api/ serão tratadas por este worker
			}
		}
	}
}
```

## Variáveis de ambiente

As seguintes variáveis de ambiente podem ser usadas para injetar diretivas Caddy no `Caddyfile` sem modificá-lo:

- `SERVER_NAME`: altera [os endereços nos quais escutar](https://caddyserver.com/docs/caddyfile/concepts#addresses), os nomes de host fornecidos também serão usados para o certificado TLS gerado
- `SERVER_ROOT`: altera o diretório raiz do site, o padrão é `public/`
- `CADDY_GLOBAL_OPTIONS`: injeta [opções globais](https://caddyserver.com/docs/caddyfile/options)
- `FRANKENPHP_CONFIG`: injeta a configuração sob a diretiva `frankenphp`

Assim como para as SAPIs FPM e CLI, as variáveis de ambiente são expostas por padrão na superglobal `$_SERVER`.

O valor `S` da [diretiva `variables_order` do PHP](https://www.php.net/manual/en/ini.core.php#ini.variables-order) é sempre equivalente a `ES` independentemente da colocação de `E` em outra parte desta diretiva.

## Configuração do PHP

Para carregar [arquivos de configuração adicionais do PHP](https://www.php.net/manual/en/configuration.file.php#configuration.file.scan),
a variável de ambiente `PHP_INI_SCAN_DIR` pode ser usada.
Quando definida, o PHP carregará todos os arquivos com a extensão `.ini` presentes nos diretórios fornecidos.

Você também pode alterar a configuração do PHP usando a diretiva `php_ini` no `Caddyfile`:

```caddyfile
{
    frankenphp {
        php_ini memory_limit 256M

        # ou

        php_ini {
            memory_limit 256M
            max_execution_time 15
        }
    }
}
```

### Desabilitando HTTPS

Por padrão, o FrankenPHP habilitará automaticamente o HTTPS para todos os nomes de host, incluindo `localhost`.
Se você quiser desabilitar o HTTPS (por exemplo, em um ambiente de desenvolvimento), você pode definir a variável de ambiente `SERVER_NAME` para `http://` ou `:80`:

Alternativamente, você pode usar todos os outros métodos descritos na [documentação do Caddy](https://caddyserver.com/docs/automatic-https#activation).

Se você quiser usar HTTPS com o endereço IP `127.0.0.1` em vez do nome de host `localhost`, por favor, leia a seção de [problemas conhecidos](known-issues.md#using-https127001-with-docker).

### Full Duplex (HTTP/1)

Ao usar HTTP/1.x, pode ser desejável habilitar o modo full-duplex para permitir a gravação de uma resposta antes que o corpo inteiro
tenha sido lido. (por exemplo: [Mercure](mercure.md), WebSocket, Server-Sent Events, etc.)

Esta é uma configuração de adesão que precisa ser adicionada às opções globais no `Caddyfile`:

```caddyfile
{
  servers {
    enable_full_duplex
  }
}
```

> [!CAUTION]
>
> Habilitar esta opção pode causar deadlock em clientes HTTP/1.x antigos que não suportam full-duplex.
> Isso também pode ser configurado usando a configuração de ambiente `CADDY_GLOBAL_OPTIONS`:

```sh
CADDY_GLOBAL_OPTIONS="servers {
  enable_full_duplex
}"
```

Você pode encontrar mais informações sobre esta configuração na [documentação do Caddy](https://caddyserver.com/docs/caddyfile/options#enable-full-duplex).

## Habilitar o modo de depuração

Ao usar a imagem Docker, defina a variável de ambiente `CADDY_GLOBAL_OPTIONS` como `debug` para habilitar o modo de depuração:

```console
docker run -v $PWD:/app/public \
    -e CADDY_GLOBAL_OPTIONS=debug \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```
