# Metrikler

> [!TIP]
> Gerçek zamanlı panolar ve üretim izlemesi dahil eksiksiz bir gözlemlenebilirlik kurulumu için [Gözlemlenebilirlik](observability.md) sayfasına bakın.

## Prometheus dışa aktarımı

[Caddy metrikleri](https://caddyserver.com/docs/metrics) etkinleştirildiğinde FrankenPHP şu metrikleri yayınlar:

- `frankenphp_total_threads`: Toplam PHP iş parçacığı sayısı.
- `frankenphp_busy_threads`: Şu anda bir isteği işleyen PHP iş parçacığı sayısı (çalışan worker'lar her zaman bir iş parçacığı tüketir).
- `frankenphp_queue_depth`: Kuyruğa alınmış olağan istek sayısı.
- `frankenphp_total_workers{worker="[worker_name]"}`: Toplam worker sayısı.
- `frankenphp_busy_workers{worker="[worker_name]"}`: Şu anda bir isteği işleyen worker sayısı.
- `frankenphp_worker_request_time{worker="[worker_name]"}`: Tüm worker'ların istekleri işlemek için harcadığı süre.
- `frankenphp_worker_request_count{worker="[worker_name]"}`: Tüm worker'ların işlediği istek sayısı.
- `frankenphp_ready_workers{worker="[worker_name]"}`: En az bir kez `frankenphp_handle_request` çağırmış worker sayısı.
- `frankenphp_worker_crashes{worker="[worker_name]"}`: Bir worker'ın beklenmedik şekilde sonlandığı sayı.
- `frankenphp_worker_restarts{worker="[worker_name]"}`: Bir worker'ın bilinçli olarak yeniden başlatıldığı sayı.
- `frankenphp_worker_queue_depth{worker="[worker_name]"}`: Kuyruğa alınmış istek sayısı.

Worker metriklerinde `[worker_name]` yer tutucusu Caddyfile'daki worker adıyla değiştirilir; aksi halde worker dosyasının mutlak yolu kullanılır.

## İş parçacığı durumu uç noktası

FrankenPHP, [Caddy admin API](https://caddyserver.com/docs/api) üzerinden bir `/frankenphp/threads` uç noktası yayınlar.
Tüm etkin PHP iş parçacıklarının JSON anlık görüntüsünü döndürür; hata ayıklama ve gözlemlenebilirlik araçları oluşturmak için kullanışlıdır.

```console
curl -s http://localhost:2019/frankenphp/threads | jq .
```

### Yanıt biçimi

Uç nokta şu yapıda bir JSON nesnesi döndürür:

```json
{
    "ThreadDebugStates": [
        {
            "Index": 0,
            "Name": "worker-/path/to/worker.php",
            "State": "ready",
            "IsWaiting": true,
            "IsBusy": false,
            "WaitingSinceMilliseconds": 1234,
            "CurrentURI": "",
            "CurrentMethod": "",
            "RequestStartedAt": 0,
            "RequestCount": 42,
            "MemoryUsage": 2097152
        }
    ],
    "ReservedThreadCount": 3
}
```

### Alanlar

| Alan | Tür | Açıklama |
|---|---|---|
| `ReservedThreadCount` | integer | Otomatik ölçekleme için ayrılmış, henüz etkin olmayan iş parçacığı sayısı. |

`ThreadDebugStates` içindeki her girdi şunları içerir:

| Alan | Tür | Açıklama |
|---|---|---|
| `Index` | integer | İş parçacığının indisi. |
| `Name` | string | İş parçacığının adı (ör. worker dosya yolu). |
| `State` | string | İş parçacığının iç durumu (ör. `ready`, `shutting down`). |
| `IsWaiting` | boolean | İş parçacığının bir istek bekleyip beklemediği. |
| `IsBusy` | boolean | İş parçacığının şu anda bir isteği işleyip işlemediği. |
| `WaitingSinceMilliseconds` | integer | İş parçacığının milisaniye cinsinden ne kadar süredir boşta olduğu. İş parçacığı meşgulse `0`. |
| `CurrentURI` | string | Şu anda işlenen URI. İş parçacığı boştaysa boş. |
| `CurrentMethod` | string | Geçerli isteğin HTTP metodu (ör. `GET`, `POST`). İş parçacığı boştaysa boş. |
| `RequestStartedAt` | integer | Geçerli isteğin başladığı Unix zaman damgası (milisaniye). İş parçacığı boştaysa `0`. |
| `RequestCount` | integer | Bu iş parçacığının başladığından beri işlediği toplam istek sayısı. |
| `MemoryUsage` | integer | İş parçacığının geçerli PHP bellek kullanımı, bayt cinsinden. |
