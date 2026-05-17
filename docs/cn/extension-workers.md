# 扩展 Worker

扩展 Worker 使您的 [FrankenPHP 扩展](https://frankenphp.dev/docs/extensions/) 能够管理专用的 PHP 线程池，用于执行后台任务、处理异步事件或实现自定义协议。适用于队列系统、事件监听器、调度器等。

## 注册 Worker

### 静态注册

如果您的 worker 不需要用户配置（固定的脚本路径、固定的线程数），您可以直接在 `init()` 函数中注册 worker。

```go
package myextension

import (
	"github.com/dunglas/frankenphp"
	"github.com/dunglas/frankenphp/caddy"
)

// 与 worker 池通信的全局句柄
var worker frankenphp.Workers

func init() {
	// 模块加载时注册 worker。
	worker = caddy.RegisterWorkers(
		"my-internal-worker", // 唯一名称
		"worker.php",         // 脚本路径（相对于执行目录或绝对路径）
		2,                    // 固定线程数
		// 可选的生命周期钩子
		frankenphp.WithWorkerOnServerStartup(func() {
			// 全局设置逻辑...
		}),
	)
}
```

### 在 Caddy 模块中（用户可配置）

如果您计划共享您的扩展（例如通用的队列或事件监听器），您应该将其封装在一个 Caddy 模块中。这允许用户通过 `Caddyfile` 配置脚本路径和线程数。这需要实现 `caddy.Provisioner` 接口并解析 Caddyfile ([查看示例](https://github.com/dunglas/frankenphp-queue/blob/989120d394d66dd6c8e2101cac73dd622fade334/caddy.go))。

### 在纯 Go 应用程序中（嵌入式）

如果您 [在没有 Caddy 的标准 Go 应用程序中嵌入 FrankenPHP](https://pkg.go.dev/github.com/dunglas/frankenphp#example-ServeHTTP)，您可以在初始化选项时使用 `frankenphp.WithExtensionWorkers` 注册扩展 worker。

## 与 Worker 交互

一旦 worker 池激活，您就可以向其分派任务。这可以在 [导出到 PHP 的原生函数](https://frankenphp.dev/docs/extensions/#writing-the-extension) 中完成，也可以从任何 Go 逻辑中完成，例如 cron 调度器、事件监听器 (MQTT、Kafka) 或任何其他 goroutine。

### 无头模式：`SendMessage`

使用 `SendMessage` 将原始数据直接传递给您的 worker 脚本。这非常适合队列或简单命令。

#### 示例：一个异步队列扩展

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
	// 1. 确保 worker 已准备就绪
	if worker == nil {
		return false
	}

	// 2. 分派给后台 worker
	_, err := worker.SendMessage(
		context.Background(), // 标准 Go 上下文
		unsafe.Pointer(data), // 要传递给 worker 的数据
		nil, // 可选的 http.ResponseWriter
	)

	return err == nil
}
```

### HTTP 模拟：`SendRequest`

如果您的扩展需要调用一个期望标准 Web 环境（填充 `$_SERVER`、`$_GET` 等）的 PHP 脚本，请使用 `SendRequest`。

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
	// 1. 准备请求和记录器
	url := frankenphp.GoString(unsafe.Pointer(path))
	req, _ := http.NewRequest("GET", url, http.NoBody)
	rr := httptest.NewRecorder()

	// 2. 分派给 worker
	if err := worker.SendRequest(rr, req); err != nil {
		return nil
	}

	// 3. 返回捕获的响应
	return frankenphp.PHPString(rr.Body.String(), false)
}
```

## Worker 脚本

PHP worker 脚本在一个循环中运行，可以处理原始消息和 HTTP 请求。

```php
<?php
// 在同一个循环中处理原始消息和 HTTP 请求
$handler = function ($payload = null) {
    // 情况 1：消息模式
    if ($payload !== null) {
        return "Received payload: " . $payload;
    }

    // 情况 2：HTTP 模式（标准 PHP 超全局变量会被填充）
    echo "Hello from page: " . $_SERVER['REQUEST_URI'];
};

while (frankenphp_handle_request($handler)) {
    gc_collect_cycles();
}
```

## 生命周期钩子

FrankenPHP 提供了钩子，用于在生命周期的特定点执行 Go 代码。

| 钩子类型   | 选项名称                     | 签名                 | 上下文与用例                                        |
| :--------- | :--------------------------- | :------------------- | :-------------------------------------------------- |
| **服务器** | `WithWorkerOnServerStartup`  | `func()`             | 全局设置。**只运行一次**。示例：连接到 NATS/Redis。 |
| **服务器** | `WithWorkerOnServerShutdown` | `func()`             | 全局清理。**只运行一次**。示例：关闭共享连接。      |
| **线程**   | `WithWorkerOnReady`          | `func(threadID int)` | 每线程设置。在线程启动时调用。接收线程 ID。         |
| **线程**   | `WithWorkerOnShutdown`       | `func(threadID int)` | 每线程清理。接收线程 ID。                           |

### 示例

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

        // 服务器启动（全局）
        frankenphp.WithWorkerOnServerStartup(func() {
            fmt.Println("扩展：服务器正在启动...")
        }),

        // 线程就绪（每线程）
        // 注意：此函数接受一个表示线程 ID 的整数
        frankenphp.WithWorkerOnReady(func(id int) {
            fmt.Printf("扩展：Worker 线程 #%d 已就绪。\n", id)
        }),
    )
}
```
