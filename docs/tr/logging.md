# Günlüğe Kayıt

FrankenPHP, [Caddy'nin günlüğe kayıt sistemi](https://caddyserver.com/docs/logging) ile sorunsuz bir şekilde entegre olur.
Standart PHP fonksiyonlarını kullanarak mesajları günlüğe kaydedebilir veya gelişmiş
yapılandırılmış günlüğe kayıt yetenekleri için özel `frankenphp_log()` fonksiyonunu kullanabilirsiniz.

## `frankenphp_log()`

`frankenphp_log()` fonksiyonu, yapılandırılmış logları doğrudan PHP uygulamanızdan yaymanıza olanak tanır,
böylece Datadog, Grafana Loki veya Elastic gibi platformlara alımı ve OpenTelemetry desteği çok daha kolay hale gelir.

Arka planda, `frankenphp_log()`, zengin günlük kaydı özellikleri sunmak için [Go'nun `log/slog` paketi](https://pkg.go.dev/log/slog)'nı sarmalar.

Bu loglar, önem derecesi seviyesini ve isteğe bağlı bağlam verilerini içerir.

```php
function frankenphp_log(string $message, int $level = FRANKENPHP_LOG_LEVEL_INFO, array $context = []): void
```

### Parametreler

- **`message`**: Günlük mesaj dizisi.
- **`level`**: Günlüğün önem derecesi seviyesi. Herhangi bir keyfi tam sayı olabilir. Yaygın seviyeler için kolaylık sabitleri sağlanmıştır: `FRANKENPHP_LOG_LEVEL_DEBUG` (`-4`), `FRANKENPHP_LOG_LEVEL_INFO` (`0`), `FRANKENPHP_LOG_LEVEL_WARN` (`4`) ve `FRANKENPHP_LOG_LEVEL_ERROR` (`8`)). Varsayılan değer `FRANKENPHP_LOG_LEVEL_INFO`'dur.
- **`context`**: Günlük girdisine eklenecek ek verilerden oluşan ilişkisel bir dizi.

### Örnek

```php
<?php

// Basit bir bilgilendirme mesajı günlüğe kaydet
frankenphp_log("Hello from FrankenPHP!");

// Bağlam verileriyle bir uyarı günlüğe kaydet
frankenphp_log(
    "Memory usage high",
    FRANKENPHP_LOG_LEVEL_WARN,
    [
        'current_usage' => memory_get_usage(),
        'peak_usage' => memory_get_peak_usage(),
    ],
);

```

Logları görüntülerken (örn. `docker compose logs` aracılığıyla), çıktı yapılandırılmış JSON olarak görünecektir:

```json
{"level":"info","ts":1704067200,"logger":"frankenphp","msg":"Hello from FrankenPHP!"}
{"level":"warn","ts":1704067200,"logger":"frankenphp","msg":"Memory usage high","current_usage":10485760,"peak_usage":12582912}
```

## `error_log()`

FrankenPHP, standart `error_log()` fonksiyonunu kullanarak da günlüğe kayıt yapılmasına olanak tanır. Eğer `$message_type` parametresi `4` (SAPI) ise, bu mesajlar Caddy günlükçüsüne yönlendirilir.

Varsayılan olarak, `error_log()` aracılığıyla gönderilen mesajlar yapılandırılmamış metin olarak ele alınır.
Mevcut uygulamalar veya standart PHP kütüphanesine dayanan kütüphanelerle uyumluluk için kullanışlıdırlar.

### `error_log()` ile Örnek

```php
error_log("Database connection failed", 4);
```

Bu, Caddy loglarında görünecektir, genellikle PHP'den kaynaklandığını belirtmek için ön eklenmiş olarak.

> [!TIP]
> Üretim ortamlarında daha iyi gözlemlenebilirlik için, `frankenphp_log()` tercih edin
> çünkü bu, logları seviyeye göre (Hata Ayıklama, Hata vb.) filtrelemenize
> ve günlük kaydı altyapınızdaki belirli alanları sorgulamanıza olanak tanır.
