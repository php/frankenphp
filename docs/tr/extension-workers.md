# Uzantı İşçileri

Uzantı İşçileri, [FrankenPHP uzantınızın](https://frankenphp.dev/docs/extensions/) arka plan görevlerini yürütmek, eşzamansız olayları işlemek veya özel protokolleri uygulamak için özel bir PHP iş parçacığı havuzunu yönetmesini sağlar. Kuyruk sistemleri, olay dinleyicileri, zamanlayıcılar vb. için kullanışlıdır.

## İşçiyi Kaydetme

### Statik Kayıt

İşçiyi kullanıcı tarafından yapılandırılabilir hale getirmeniz gerekmiyorsa (sabit komut dosyası yolu, sabit iş parçacığı sayısı), işçiyi `init()` fonksiyonunda basitçe kaydedebilirsiniz.

```go
package myextension

import (
	"github.com/dunglas/frankenphp"
	"github.com/dunglas/frankenphp/caddy"
)

// İşçi havuzuyla iletişim kurmak için genel tanıtıcı
var worker frankenphp.Workers

func init() {
	// Modül yüklendiğinde işçiyi kaydet.
	worker = caddy.RegisterWorkers(
		"my-internal-worker", // Benzersiz isim
		"worker.php",         // Komut dosyası yolu (çalışmaya göre veya mutlak)
		2,                    // Sabit iş parçacığı sayısı
		// İsteğe bağlı Yaşam Döngüsü Kancaları
		frankenphp.WithWorkerOnServerStartup(func() {
			// Genel kurulum mantığı...
		}),
	)
}
```

### Bir Caddy Modülünde (Kullanıcı tarafından yapılandırılabilir)

Uzantınızı paylaşmayı planlıyorsanız (genel bir kuyruk veya olay dinleyici gibi), onu bir Caddy modülüne sarmalısınız. Bu, kullanıcıların `Caddyfile` aracılığıyla komut dosyası yolunu ve iş parçacığı sayısını yapılandırmasına olanak tanır. Bu, `caddy.Provisioner` arayüzünü uygulamayı ve Caddyfile'ı ayrıştırmayı gerektirir ([bir örnek görmek için](https://github.com/dunglas/frankenphp-queue/blob/989120d394d66dd6c8e2101cac73dd622fade334/caddy.go)).

### Saf Bir Go Uygulamasında (Gömme)

FrankenPHP'yi [Caddy olmadan standart bir Go uygulamasına gömüyorsanız](https://pkg.go.dev/github.com/dunglas/frankenphp#example-ServeHTTP), seçenekleri başlatırken `frankenphp.WithExtensionWorkers` kullanarak uzantı işçilerini kaydedebilirsiniz.

## İşçilerle Etkileşim Kurma

İşçi havuzu aktif hale geldiğinde, ona görevler gönderebilirsiniz. Bu, [PHP'ye dışa aktarılan yerel fonksiyonlar](https://frankenphp.dev/docs/extensions/#writing-the-extension) içinde veya bir cron zamanlayıcı, bir olay dinleyicisi (MQTT, Kafka) veya herhangi başka bir goroutine gibi herhangi bir Go mantığından yapılabilir.

### Başsız Mod : `SendMessage`

Doğrudan işçi komut dosyanıza ham veri geçirmek için `SendMessage` kullanın. Bu, kuyruklar veya basit komutlar için idealdir.

#### Örnek: Asenkron Bir Kuyruk Uzantısı

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
	// 1. İşçinin hazır olduğundan emin olun
	if worker == nil {
		return false
	}

	// 2. Arka plan işçisine gönder
	_, err := worker.SendMessage(
		context.Background(), // Standart Go bağlamı
		unsafe.Pointer(data), // İşçiye iletilecek veri
		nil, // İsteğe bağlı http.ResponseWriter
	)

	return err == nil
}
```

### HTTP Emülasyonu :`SendRequest`

Uzantınızın standart bir web ortamı bekleyen ( `$_SERVER`, `$_GET` vb. dolduran) bir PHP komut dosyasını çağırması gerekiyorsa `SendRequest` kullanın.

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
	// 1. İsteği ve kaydediciyi hazırla
	url := frankenphp.GoString(unsafe.Pointer(path))
	req, _ := http.NewRequest("GET", url, http.NoBody)
	rr := httptest.NewRecorder()

	// 2. İşçiye gönder
	if err := worker.SendRequest(rr, req); err != nil {
		return nil
	}

	// 3. Yakalanan yanıtı döndür
	return frankenphp.PHPString(rr.Body.String(), false)
}
```

## İşçi Komut Dosyası

PHP işçi komut dosyası bir döngüde çalışır ve hem ham mesajları hem de HTTP isteklerini işleyebilir.

```php
<?php
// Hem ham mesajları hem de HTTP isteklerini aynı döngüde işle
$handler = function ($payload = null) {
    // Durum 1: Mesaj Modu
    if ($payload !== null) {
        return "Received payload: " . $payload;
    }

    // Durum 2: HTTP Modu (standart PHP süper küreselleri doldurulur)
    echo "Hello from page: " . $_SERVER['REQUEST_URI'];
};

while (frankenphp_handle_request($handler)) {
    gc_collect_cycles();
}
```

## Yaşam Döngüsü Kancaları

FrankenPHP, yaşam döngüsünün belirli noktalarında Go kodunu yürütmek için kancalar sağlar.

| Kanca Türü | Seçenek Adı | İmza | Bağlam ve Kullanım Durumu |
| :--------- | :--------------------------- | :------------------- | :--------------------------------------------------------------------- |
| **Sunucu** | `WithWorkerOnServerStartup`  | `func()`             | Genel kurulum. **Bir Kez** çalışır. Örnek: NATS/Redis'e bağlanma. |
| **Sunucu** | `WithWorkerOnServerShutdown` | `func()`             | Genel temizleme. **Bir Kez** çalışır. Örnek: Paylaşılan bağlantıları kapatma. |
| **İş Parçacığı** | `WithWorkerOnReady`          | `func(threadID int)` | İş parçacığı başına kurulum. Bir iş parçacığı başladığında çağrılır. İş Parçacığı Kimliğini alır. |
| **İş Parçacığı** | `WithWorkerOnShutdown`       | `func(threadID int)` | İş parçacığı başına temizleme. İş Parçacığı Kimliğini alır. |

### Örnek

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

        // Sunucu Başlatma (Genel)
        frankenphp.WithWorkerOnServerStartup(func() {
            fmt.Println("Uzantı: Sunucu başlıyor...")
        }),

        // İş Parçacığı Hazır (İş Parçacığı Başına)
        // Not: Fonksiyon, İş Parçacığı Kimliğini temsil eden bir tamsayı kabul eder
        frankenphp.WithWorkerOnReady(func(id int) {
            fmt.Printf("Uzantı: İşçi iş parçacığı #%d hazır.\n", id)
        }),
    )
}
