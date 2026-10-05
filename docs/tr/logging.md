# Günlük kaydı

> [!TIP]
> Günlük kaydı, FrankenPHP'nin gözlemlenebilirlik öyküsünün bir parçasıdır. Gerçek zamanlı izleme ve metrikler dahil tam tablo için [Gözlemlenebilirlik](observability.md) sayfasına bakın.

FrankenPHP, [Caddy'nin günlük sistemiyle](https://caddyserver.com/docs/logging) sorunsuz entegre olur.
Standart PHP işlevleriyle mesaj kaydedebilir veya gelişmiş yapılandırılmış günlük yetenekleri için özel `frankenphp_log()` işlevini kullanabilirsiniz.

## `frankenphp_log()`

`frankenphp_log()` işlevi, yapılandırılmış günlükleri doğrudan PHP uygulamanızdan üretmenizi sağlar;
bu da Datadog, Grafana Loki veya Elastic gibi platformlara aktarımı ve OpenTelemetry desteğini kolaylaştırır.

Perde arkasında `frankenphp_log()`, zengin günlük özellikleri sunmak için [Go'nun `log/slog` paketini](https://pkg.go.dev/log/slog) sarmalar.

Bu günlükler şiddet düzeyini ve isteğe bağlı bağlam verisini içerir.

```php
function frankenphp_log(string $message, int $level = FRANKENPHP_LOG_LEVEL_INFO, array $context = []): void
```

### Parametreler

- **`message`**: Günlük mesajı dizesi.
- **`level`**: Günlüğün şiddet düzeyi. Herhangi bir tamsayı olabilir. Yaygın düzeyler için kolaylık sabitleri sağlanır: `FRANKENPHP_LOG_LEVEL_DEBUG` (`-4`), `FRANKENPHP_LOG_LEVEL_INFO` (`0`), `FRANKENPHP_LOG_LEVEL_WARN` (`4`) ve `FRANKENPHP_LOG_LEVEL_ERROR` (`8`). Varsayılan `FRANKENPHP_LOG_LEVEL_INFO`'dur.
- **`context`**: Günlük kaydına eklenecek ek verilerin ilişkisel dizisi.

### Örnek

```php
<?php

// Basit bir bilgilendirme mesajı kaydedin
frankenphp_log("Hello from FrankenPHP!");

// Bağlam verisiyle bir uyarı kaydedin
frankenphp_log(
    "Memory usage high",
    FRANKENPHP_LOG_LEVEL_WARN,
    [
        'current_usage' => memory_get_usage(),
        'peak_usage' => memory_get_peak_usage(),
    ],
);

```

Günlükler görüntülendiğinde (ör. `docker compose logs` ile) çıktı yapılandırılmış JSON olarak görünür:

```json
{"level":"info","ts":1704067200,"logger":"frankenphp","msg":"Hello from FrankenPHP!"}
{"level":"warn","ts":1704067200,"logger":"frankenphp","msg":"Memory usage high","current_usage":10485760,"peak_usage":12582912}
```

## `error_log()`

FrankenPHP, standart `error_log()` işleviyle günlük kaydına da izin verir. `$message_type` parametresi `4` (SAPI) ise
bu mesajlar Caddy logger'ına yönlendirilir.

Varsayılan olarak `error_log()` ile gönderilen mesajlar yapılandırılmamış metin olarak işlenir.
Standart PHP kütüphanesine dayanan mevcut uygulamalar veya kütüphanelerle uyumluluk için kullanışlıdır.

### error_log() örneği

```php
error_log("Database connection failed", 4);
```

Bu, Caddy günlüklerinde görünür; çoğu zaman PHP'den geldiğini belirten bir önekle.

> [!TIP]
> Üretim ortamlarında daha iyi gözlemlenebilirlik için `frankenphp_log()` tercih edin;
> çünkü günlükleri düzeye göre (Debug, Error vb.) süzmenize
> ve günlük altyapınızda belirli alanları sorgulamanıza olanak tanır.
