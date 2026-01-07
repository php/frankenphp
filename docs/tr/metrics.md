# Metrikler

[Caddy metrikleri](https://caddyserver.com/docs/metrics) etkinleştirildiğinde, FrankenPHP aşağıdaki metrikleri sunar:

- `frankenphp_total_threads`: Toplam PHP iş parçacığı sayısı.
- `frankenphp_busy_threads`: Şu anda bir isteği işleyen PHP iş parçacığı sayısı (çalışan işçiler her zaman bir iş parçacığı tüketir).
- `frankenphp_queue_depth`: Düzenli olarak kuyruğa alınmış isteklerin sayısı.
- `frankenphp_total_workers{worker="[worker_name]"}`: Toplam işçi sayısı.
- `frankenphp_busy_workers{worker="[worker_name]"}`: Şu anda bir isteği işleyen işçi sayısı.
- `frankenphp_worker_request_time{worker="[worker_name]"}`: Tüm işçiler tarafından isteklerin işlenmesi için harcanan süre.
- `frankenphp_worker_request_count{worker="[worker_name]"}`: Tüm işçiler tarafından işlenen istek sayısı.
- `frankenphp_ready_workers{worker="[worker_name]"}`: `frankenphp_handle_request`'i en az bir kez çağıran işçi sayısı.
- `frankenphp_worker_crashes{worker="[worker_name]"}`: Bir işçinin beklenmedik bir şekilde sonlandığı sayı.
- `frankenphp_worker_restarts{worker="[worker_name]"}`: Bir işçinin bilerek yeniden başlatıldığı sayı.
- `frankenphp_worker_queue_depth{worker="[worker_name]"}`: Kuyruğa alınmış istek sayısı.

İşçi metrikleri için, `[worker_name]` yer tutucusu Caddyfile'daki işçi adıyla değiştirilir, aksi takdirde işçi dosyasının mutlak yolu kullanılacaktır.
