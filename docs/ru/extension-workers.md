# Расширение Workers

Расширение Workers позволяет вашему [расширению FrankenPHP](https://frankenphp.dev/docs/extensions/) управлять выделенным пулом PHP-потоков для выполнения фоновых задач, обработки асинхронных событий или реализации пользовательских протоколов. Полезно для систем очередей, слушателей событий, планировщиков и т. д.

## Регистрация Worker-а

### Статическая регистрация

Если вам не нужно делать Worker-а настраиваемым пользователем (фиксированный путь к скрипту, фиксированное количество потоков), вы можете просто зарегистрировать Worker в функции `init()`.

```go
package myextension

import (
	"github.com/dunglas/frankenphp"
	"github.com/dunglas/frankenphp/caddy"
)

// Глобальный дескриптор для связи с пулом Worker-ов
var worker frankenphp.Workers

func init() {
	// Зарегистрировать Worker при загрузке модуля.
	worker = caddy.RegisterWorkers(
		"my-internal-worker", // Уникальное имя
		"worker.php",         // Путь к скрипту (относительный или абсолютный)
		2,                    // Фиксированное количество потоков
		// Дополнительные хуки жизненного цикла
		frankenphp.WithWorkerOnServerStartup(func() {
			// Глобальная логика настройки...
		}),
	)
}
```

### В модуле Caddy (настраивается пользователем)

Если вы планируете делиться своим расширением (например, универсальной очередью или слушателем событий), вам следует обернуть его в модуль Caddy. Это позволит пользователям настраивать путь к скрипту и количество потоков через свой `Caddyfile`. Это требует реализации интерфейса `caddy.Provisioner` и парсинга Caddyfile ([см. пример](https://github.com/dunglas/frankenphp-queue/blob/989120d394d66dd6c8e2101cac73dd622fade334/caddy.go)).

### В чистом Go-приложении (встраивание)

Если вы [встраиваете FrankenPHP в стандартное Go-приложение без Caddy](https://pkg.go.dev/github.com/dunglas/frankenphp#example-ServeHTTP), вы можете зарегистрировать Worker-ы расширения, используя `frankenphp.WithExtensionWorkers` при инициализации опций.

## Взаимодействие с Worker-ами

Как только пул Worker-ов активен, вы можете отправлять ему задачи. Это можно сделать внутри [нативных функций, экспортированных в PHP](https://frankenphp.dev/docs/extensions/#writing-the-extension), или из любой Go-логики, такой как планировщик cron, слушатель событий (MQTT, Kafka) или любая другая горутина.

### Безголовый режим: `SendMessage`

Используйте `SendMessage` для прямой передачи необработанных данных вашему скрипту Worker-а. Это идеально подходит для очередей или простых команд.

#### Пример: Расширение асинхронной очереди

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
	// 1. Убедитесь, что Worker готов
	if worker == nil {
		return false
	}

	// 2. Отправить фоновому Worker-у
	_, err := worker.SendMessage(
		context.Background(), // Стандартный Go-контекст
		unsafe.Pointer(data), // Данные для передачи Worker-у
		nil, // Опциональный http.ResponseWriter
	)

	return err == nil
}
```

### Эмуляция HTTP: `SendRequest`

Используйте `SendRequest`, если ваше расширение должно вызвать PHP-скрипт, который ожидает стандартную веб-среду (заполнение `$_SERVER`, `$_GET` и т. д.).

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
	// 1. Подготовьте запрос и рекордер
	url := frankenphp.GoString(unsafe.Pointer(path))
	req, _ := http.NewRequest("GET", url, http.NoBody)
	rr := httptest.NewRecorder()

	// 2. Отправить Worker-у
	if err := worker.SendRequest(rr, req); err != nil {
		return nil
	}

	// 3. Вернуть захваченный ответ
	return frankenphp.PHPString(rr.Body.String(), false)
}
```

## Скрипт Worker-а

PHP-скрипт Worker-а выполняется в цикле и может обрабатывать как необработанные сообщения, так и HTTP-запросы.

```php
<?php
// Обрабатывать как необработанные сообщения, так и HTTP-запросы в одном цикле
$handler = function ($payload = null) {
    // Случай 1: Режим сообщений
    if ($payload !== null) {
        return "Received payload: " . $payload;
    }

    // Случай 2: Режим HTTP (заполняются стандартные суперглобальные переменные PHP)
    echo "Hello from page: " . $_SERVER['REQUEST_URI'];
};

while (frankenphp_handle_request($handler)) {
    gc_collect_cycles();
}
```

## Хуки жизненного цикла

FrankenPHP предоставляет хуки для выполнения Go-кода в определенные моменты жизненного цикла.

| Тип хука  | Имя опции                    | Подпись              | Контекст и вариант использования                                      |
| :-------- | :--------------------------- | :------------------- | :-------------------------------------------------------------------- |
| **Сервер** | `WithWorkerOnServerStartup`  | `func()`             | Глобальная настройка. Выполняется **один раз**. Пример: Подключение к NATS/Redis. |
| **Сервер** | `WithWorkerOnServerShutdown` | `func()`             | Глобальная очистка. Выполняется **один раз**. Пример: Закрытие общих соединений. |
| **Поток** | `WithWorkerOnReady`          | `func(threadID int)` | Настройка для каждого потока. Вызывается при запуске потока. Получает ID потока. |
| **Поток** | `WithWorkerOnShutdown`       | `func(threadID int)` | Очистка для каждого потока. Получает ID потока.                     |

### Пример

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

        // Запуск сервера (Глобальный)
        frankenphp.WithWorkerOnServerStartup(func() {
            fmt.Println("Extension: Server starting up...")
        }),

        // Поток готов (Для каждого потока)
        // Примечание: Функция принимает целое число, представляющее ID потока
        frankenphp.WithWorkerOnReady(func(id int) {
            fmt.Printf("Extension: Worker thread #%d is ready.\n", id)
        }),
    )
}
```
