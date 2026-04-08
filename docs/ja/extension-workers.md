# 拡張ワーカー

拡張ワーカーは、[FrankenPHP拡張機能](https://frankenphp.dev/docs/extensions/)がバックグラウンドタスクの実行、非同期イベントの処理、またはカスタムプロトコルの実装のために、PHPスレッドの専用プールを管理できるようにします。キューシステム、イベントリスナー、スケジューラーなどに役立ちます。

## ワーカーの登録

### 静的登録

ワーカーをユーザーが構成可能にする必要がない場合（固定スクリプトパス、固定スレッド数）、`init()` 関数でワーカーを登録するだけです。

```go
package myextension

import (
	"github.com/dunglas/frankenphp"
	"github.com/dunglas/frankenphp/caddy"
)

// ワーカープールと通信するためのグローバルハンドル
var worker frankenphp.Workers

func init() {
	// モジュールがロードされたときにワーカーを登録します。
	worker = caddy.RegisterWorkers(
		"my-internal-worker", // ユニークな名前
		"worker.php",         // スクリプトパス（実行場所からの相対パス、または絶対パス）
		2,                    // 固定スレッド数
		// オプションのライフサイクルフック
		frankenphp.WithWorkerOnServerStartup(func() {
			// グローバルなセットアップロジック...
		}),
	)
}
```

### Caddyモジュール内 (ユーザーが構成可能)

拡張機能を共有する予定がある場合（一般的なキューやイベントリスナーなど）、Caddyモジュールにラップする必要があります。これにより、ユーザーは `Caddyfile` を介してスクリプトパスとスレッド数を構成できます（[例を見る](https://github.com/dunglas/frankenphp-queue/blob/989120d394d66dd6c8e2101cac73dd622fade334/caddy.go)）。

### 純粋なGoアプリケーション内 (組み込み)

[Caddyなしで標準GoアプリケーションにFrankenPHPを組み込む](https://pkg.go.dev/github.com/dunglas/frankenphp#example-ServeHTTP)場合、初期化オプションで `frankenphp.WithExtensionWorkers` を使用して拡張ワーカーを登録できます。

## ワーカーとの対話

ワーカープールがアクティブになったら、タスクをディスパッチできます。これは、[PHPにエクスポートされたネイティブ関数](https://frankenphp.dev/docs/extensions/#writing-the-extension)内、またはGoのロジック（cronスケジューラー、イベントリスナー（MQTT、Kafka）、その他のゴルーチンなど）から実行できます。

### ヘッドレスモード: `SendMessage`

`SendMessage` を使用して、生データをワーカーのスクリプトに直接渡します。これはキューや単純なコマンドに最適です。

#### 例: 非同期キュー拡張機能

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
	// 1. ワーカーが準備できていることを確認する
	if worker == nil {
		return false
	}

	// 2. バックグラウンドワーカーにディスパッチする
	_, err := worker.SendMessage(
		context.Background(), // 標準のGoコンテキスト
		unsafe.Pointer(data), // ワーカーに渡すデータ
		nil, // オプションのhttp.ResponseWriter
	)

	return err == nil
}
```

### HTTPエミュレーション: `SendRequest`

拡張機能が標準のウェブ環境（`$_SERVER`、`$_GET` など）を期待するPHPスクリプトを呼び出す必要がある場合は、`SendRequest` を使用します。

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
	// 1. リクエストとレコーダーを準備する
	url := frankenphp.GoString(unsafe.Pointer(path))
	req, _ := http.NewRequest("GET", url, http.NoBody)
	rr := httptest.NewRecorder()

	// 2. ワーカーにディスパッチする
	if err := worker.SendRequest(rr, req); err != nil {
		return nil
	}

	// 3. キャプチャされたレスポンスを返す
	return frankenphp.PHPString(rr.Body.String(), false)
}
```

## ワーカーのスクリプト

PHPワーカーのスクリプトはループで実行され、生メッセージとHTTPリクエストの両方を処理できます。

```php
<?php
// 同じループで生のメッセージとHTTPリクエストの両方を処理する
$handler = function ($payload = null) {
    // ケース1: メッセージモード
    if ($payload !== null) {
        return "Received payload: " . $payload;
    }

    // ケース2: HTTPモード（標準のPHPスーパーグローバルが設定される）
    echo "Hello from page: " . $_SERVER['REQUEST_URI'];
};

while (frankenphp_handle_request($handler)) {
    gc_collect_cycles();
}
```

## ライフサイクルフック

FrankenPHPは、ライフサイクルの特定の時点でGoコードを実行するためのフックを提供します。

| フックタイプ | オプション名                 | シグネチャ           | コンテキストと使用例                                                                           |
| :----------- | :--------------------------- | :------------------- | :--------------------------------------------------------------------------------------------- |
| **サーバー** | `WithWorkerOnServerStartup`  | `func()`             | グローバルなセットアップ。**一度だけ**実行されます。例: NATS/Redisへの接続。                   |
| **サーバー** | `WithWorkerOnServerShutdown` | `func()`             | グローバルなクリーンアップ。**一度だけ**実行されます。例: 共有接続のクローズ。                 |
| **スレッド** | `WithWorkerOnReady`          | `func(threadID int)` | スレッドごとのセットアップ。スレッドが開始したときに呼び出されます。スレッドIDを受け取ります。 |
| **スレッド** | `WithWorkerOnShutdown`       | `func(threadID int)` | スレッドごとのクリーンアップ。スレッドIDを受け取ります。                                       |

### 例

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

        // サーバー起動時 (グローバル)
        frankenphp.WithWorkerOnServerStartup(func() {
            fmt.Println("Extension: Server starting up...")
        }),

        // スレッド準備完了時 (スレッドごと)
        // 注: この関数はスレッドIDを表す整数を受け入れます
        frankenphp.WithWorkerOnReady(func(id int) {
            fmt.Printf("Extension: Worker thread #%d is ready.\n", id)
        }),
    )
}
```
