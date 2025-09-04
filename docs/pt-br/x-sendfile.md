# Servindo arquivos estáticos grandes com eficiência (`X-Sendfile`/`X-Accel-Redirect`)

Normalmente, arquivos estáticos podem ser servidos diretamente pelo servidor
web, mas às vezes é necessário executar algum código PHP antes de enviá-los:
controle de acesso, estatísticas, cabeçalhos HTTP personalizados...

Infelizmente, usar PHP para servir arquivos estáticos grandes é ineficiente em
comparação com o uso direto do servidor web (sobrecarga de memória, desempenho
reduzido...).

O FrankenPHP permite delegar o envio de arquivos estáticos ao servidor web
**após** a execução de código PHP personalizado.

Para fazer isso, sua aplicação PHP precisa simplesmente definir um cabeçalho
HTTP personalizado contendo o caminho do arquivo a ser servido.
O FrankenPHP cuida do resto.

Esse recurso é conhecido como **`X-Sendfile`** para Apache e
**`X-Accel-Redirect`** para NGINX.

Nos exemplos a seguir, assumimos que o diretório raiz do projeto é o diretório
`public/` e que queremos usar PHP para servir arquivos armazenados fora do
diretório `public/`, de um diretório chamado `arquivos-privados/`.

## Configuração

Primeiro, adicione a seguinte configuração ao seu `Caddyfile` para habilitar
este recurso:

```patch
	root public/
	# ...

+	# Necessário para Symfony, Laravel e outros projetos que usam o componente
+	# Symfony HttpFoundation
+	request_header X-Sendfile-Type x-accel-redirect
+	request_header X-Accel-Mapping ../arquivos-privados=/arquivos-privados
+
+	intercept {
+		@accel header X-Accel-Redirect *
+		handle_response @accel {
+			root arquivos-privados/
+			rewrite * {resp.header.X-Accel-Redirect}
+			method * GET
+
+			# Remove o cabeçalho X-Accel-Redirect definido pelo PHP para maior
+			# segurança
+			header -X-Accel-Redirect
+
+			file_server
+		}
+	}

	php_server
```

## PHP simples

Defina o caminho relativo do arquivo (de `arquivos-privados/`) como o valor do
cabeçalho `X-Accel-Redirect`:

```php
header('X-Accel-Redirect: arquivo.txt');
```

## Projetos que utilizam o componente Symfony HttpFoundation (Symfony, Laravel, Drupal...)

Symfony HttpFoundation
[suporta nativamente este recurso](https://symfony.com/doc/current/components/http_foundation.html#serving-files).
Ele determinará automaticamente o valor correto para o cabeçalho
`X-Accel-Redirect` e o adicionará à resposta.

```php
use Symfony\Component\HttpFoundation\BinaryFileResponse;

BinaryFileResponse::trustXSendfileTypeHeader();
$response = new BinaryFileResponse(__DIR__.'/../arquivos-privados/arquivo.txt');

// ...
```
