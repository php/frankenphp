# Workers de Extensão

Os Workers de Extensão permitem que sua [extensão FrankenPHP](https://frankenphp.dev/docs/extensions/) gerencie um pool dedicado de threads PHP para executar tarefas em segundo plano, lidar com eventos assíncronos ou implementar protocolos personalizados. Útil para sistemas de fila, listeners de eventos, agendadores, etc.

## Registrando o Worker

### Registro Estático

Se você não precisa que o worker seja configurável pelo usuário (caminho de script fixo, número de threads fixo), você pode simplesmente registrar o worker na função `init()`.

```go
package myextension

import (
	"github.com/dunglas/frankenphp"
	"github.com/dunglas/frankenphp/caddy"
)

// Handle global para comunicar com o pool de workers
var worker frankenphp.Workers

func init() {
	// Registra o worker quando o módulo é carregado.
	worker = caddy.RegisterWorkers(
		"my-internal-worker", // Nome único
		"worker.php",         // Caminho do script (relativo à execução ou absoluto)
		2,                    // Contagem fixa de threads
		// Hooks de ciclo de vida opcionais
		frankenphp.WithWorkerOnServerStartup(func() {
			// Lógica de configuração global...
		}),
	)
}
```

### Em um Módulo Caddy (Configurável pelo usuário)

Se você planeja compartilhar sua extensão (como uma fila genérica ou um listener de eventos), você deve encapsulá-la em um módulo Caddy. Isso permite que os usuários configurem o caminho do script e a contagem de threads através do seu `Caddyfile`. Isso exige a implementação da interface `caddy.Provisioner` e a análise do Caddyfile ([veja um exemplo](https://github.com/dunglas/frankenphp-queue/blob/989120d394d66dd6c8e2101cac73dd622fade334/caddy.go)).

### Em uma Aplicação Go Pura (Embedagem)

Se você está [embedando o FrankenPHP em uma aplicação Go padrão sem caddy](https://pkg.go.dev/github.com/dunglas/frankenphp#example-ServeHTTP), você pode registrar workers de extensão usando `frankenphp.WithExtensionWorkers` ao inicializar as opções.

## Interagindo com Workers

Assim que o pool de workers estiver ativo, você pode despachar tarefas para ele. Isso pode ser feito dentro de [funções nativas exportadas para PHP](https://frankenphp.dev/docs/extensions/#writing-the-extension), ou de qualquer lógica Go, como um agendador cron, um listener de eventos (MQTT, Kafka), ou qualquer outra goroutine.

### Modo Headless: `SendMessage`

Use `SendMessage` para passar dados brutos diretamente para o seu script worker. Isso é ideal para filas ou comandos simples.

#### Exemplo: Uma Extensão de Fila Assíncrona

```go
// #include <Zend/zend_types.h>
import "C"
import (
	"context"
	"unsafe"
	"github.com/dunglas/frankenphp"
)

//export_php:function my_queue_push(mixed $data): bool
func my_queue_push(data *C.zval) bool {
	// 1. Garante que o worker esteja pronto
	if worker == nil {
		return false
	}

	// 2. Despacha para o worker em segundo plano
	_, err := worker.SendMessage(
		context.Background(), // Contexto Go padrão
		unsafe.Pointer(data), // Dados a serem passados para o worker
		nil, // http.ResponseWriter opcional
	)

	return err == nil
}
```

### Emulação HTTP: `SendRequest`

Use `SendRequest` se sua extensão precisar invocar um script PHP que espera um ambiente web padrão (populando `$_SERVER`, `$_GET`, etc.).

```go
// #include <Zend/zend_types.h>
import "C"
import (
	"net/http"
	"net/http/httptest"
	"unsafe"
	"github.com/dunglas/frankenphp"
)

//export_php:function my_worker_http_request(string $path): string
func my_worker_http_request(path *C.zend_string) unsafe.Pointer {
	// 1. Prepara a requisição e o gravador
	url := frankenphp.GoString(unsafe.Pointer(path))
	req, _ := http.NewRequest("GET", url, http.NoBody)
	rr := httptest.NewRecorder()

	// 2. Despacha para o worker
	if err := worker.SendRequest(rr, req); err != nil {
		return nil
	}

	// 3. Retorna a resposta capturada
	return frankenphp.PHPString(rr.Body.String(), false)
}
```

## Script do Worker

O script PHP do worker é executado em um loop e pode lidar tanto com mensagens brutas quanto com requisições HTTP.

```php
<?php
// Lida tanto com mensagens brutas quanto com requisições HTTP no mesmo loop
$handler = function ($payload = null) {
    // Caso 1: Modo de Mensagem
    if ($payload !== null) {
        return "Received payload: " . $payload;
    }

    // Caso 2: Modo HTTP (superglobais PHP padrão são populadas)
    echo "Hello from page: " . $_SERVER['REQUEST_URI'];
};

while (frankenphp_handle_request($handler)) {
    gc_collect_cycles();
}
```

## Hooks de Ciclo de Vida

FrankenPHP oferece hooks para executar código Go em pontos específicos do ciclo de vida.

| Tipo de Hook | Nome da Opção                | Assinatura           | Contexto e Caso de Uso                                                           |
| :----------- | :--------------------------- | :------------------- | :------------------------------------------------------------------------------- |
| **Servidor** | `WithWorkerOnServerStartup`  | `func()`             | Configuração global. Executado **Uma Vez**. Exemplo: Conectar ao NATS/Redis.     |
| **Servidor** | `WithWorkerOnServerShutdown` | `func()`             | Limpeza global. Executado **Uma Vez**. Exemplo: Fechar conexões compartilhadas.  |
| **Thread**   | `WithWorkerOnReady`          | `func(threadID int)` | Configuração por thread. Chamado quando um thread inicia. Recebe o ID do Thread. |
| **Thread**   | `WithWorkerOnShutdown`       | `func(threadID int)` | Limpeza por thread. Recebe o ID do Thread.                                       |

### Exemplo

```go
package myextension

import (
    "fmt"
    "github.com/dunglas/frankenphp"
    frankenphpCaddy "github.com/dunglas/frankenphp/caddy"
)

func init() {
    workerHandle = frankenphpCaddy.RegisterWorkers(
        "my-worker", "worker.php", 2,

        // Inicialização do Servidor (Global)
        frankenphp.WithWorkerOnServerStartup(func() {
            fmt.Println("Extensão: Servidor iniciando...")
        }),

        // Thread Pronta (Por Thread)
        // Nota: A função aceita um inteiro representando o ID do Thread
        frankenphp.WithWorkerOnReady(func(id int) {
            fmt.Printf("Extensão: Thread worker #%d está pronta.\n", id)
        }),
    )
}
```
