# FrankenPHP Worker'ları Kullanma

Uygulamanızı bir kez önyükleyin ve bellekte tutun.
FrankenPHP gelen istekleri birkaç milisaniye içinde halledecektir.

## Çalışan Komut Dosyalarının Başlatılması

### Docker

`FRANKENPHP_CONFIG` ortam değişkeninin değerini `worker /path/to/your/worker/script.php` olarak ayarlayın:

```console
docker run \
    -e FRANKENPHP_CONFIG="worker /app/path/to/your/worker/script.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### Bağımsız İkili

Geçerli dizinin içeriğini bir worker kullanarak sunmak için `php-server` komutunun `--worker` seçeneğini kullanın:

```console
frankenphp php-server --worker /path/to/your/worker/script.php
```

PHP uygulamanız [ikili dosyaya gömülü](embed.md) ise, uygulamanın kök dizinine özel bir `Caddyfile` ekleyebilirsiniz.
Otomatik olarak kullanılacaktır.

Dosya değişikliklerinde worker'ı yeniden başlatmak ([dosya değişikliklerini izleme](config.md#watching-for-file-changes)) `--watch` seçeneğiyle de mümkündür.
Aşağıdaki komut, `/path/to/your/app/` dizininde veya alt dizinlerde `.php` ile biten herhangi bir dosya değiştirilirse yeniden başlatmayı tetikleyecektir:

```console
frankenphp php-server --worker /path/to/your/worker/script.php --watch="/path/to/your/app/**/*.php"
```

Bu özellik genellikle [hot reloading](hot-reload.md) ile birlikte kullanılır.

## Symfony Çalışma Zamanı

> [!TIP]
> Bu bölüm, FrankenPHP worker moduna yerel desteğin sunulduğu Symfony 7.4 öncesi için gereklidir.

FrankenPHP'nin worker modu [Symfony Runtime Component](https://symfony.com/doc/current/components/runtime.html) tarafından desteklenmektedir.
Herhangi bir Symfony uygulamasını bir worker'da başlatmak için [PHP Runtime](https://github.com/php-runtime/runtime)'ın FrankenPHP paketini yükleyin:

```console
composer require runtime/frankenphp-symfony
```

FrankenPHP Symfony Runtime'ı kullanmak için `APP_RUNTIME` ortam değişkenini tanımlayarak uygulama sunucunuzu başlatın:

```console
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php" \
    -e APP_RUNTIME=Runtime\\FrankenPhpSymfony\\Runtime \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

## Laravel Octane

Bkz. [özel dokümantasyon](laravel.md#laravel-octane).

## Özel Uygulamalar

Aşağıdaki örnek, üçüncü taraf bir kütüphaneye güvenmeden kendi worker betiğinizi nasıl oluşturacağınızı göstermektedir:

```php
<?php
// public/index.php

// Uygulamanızı önyükleyin
require __DIR__.'/vendor/autoload.php';

$myApp = new \App\Kernel();
$myApp->boot();

// Daha iyi performans için döngü dışında işleyici (daha az iş yapıyor)
$handler = static function () use ($myApp) {
    try {
        // Bir istek alındığında çağrılır,
        // süper küresel değişkenler, php://input ve benzerleri sıfırlanır
        echo $myApp->handle($_GET, $_POST, $_COOKIE, $_FILES, $_SERVER);
    } catch (\Throwable $exception) {
        // `set_exception_handler` yalnızca worker betiği sona erdiğinde çağrılır,
        // bu beklediğiniz gibi olmayabilir, bu yüzden istisnaları burada yakalayın ve ele alın
        (new \MyCustomExceptionHandler)->handleException($exception);
    }
};

$maxRequests = (int)($_SERVER['MAX_REQUESTS'] ?? 0);
for ($nbRequests = 0; !$maxRequests || $nbRequests < $maxRequests; ++$nbRequests) {
    $keepRunning = \frankenphp_handle_request($handler);

    // HTTP yanıtını gönderdikten sonra bir şey yapın
    $myApp->terminate();

    // Bir sayfa oluşturmanın ortasında tetiklenme olasılığını azaltmak için çöp toplayıcıyı çağırın
    gc_collect_cycles();

    if (!$keepRunning) break;
}

// Temizleme
$myApp->shutdown();
```

Ardından, uygulamanızı başlatın ve worker'ınızı yapılandırmak için `FRANKENPHP_CONFIG` ortam değişkenini kullanın:

```console
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

Varsayılan olarak, CPU başına 2 worker başlatılır.
Başlatılacak worker sayısını da yapılandırabilirsiniz:

```console
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php 42" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### Belirli Sayıda İstekten Sonra Worker'ı Yeniden Başlatın

PHP başlangıçta uzun süreli işlemler için tasarlanmadığından, hala bellek sızdıran birçok kütüphane ve eski kod vardır.
Bu tür kodları worker modunda kullanmak için geçici bir çözüm, belirli sayıda isteği işledikten sonra worker betiğini yeniden başlatmaktır:

Önceki worker kod parçacığı, `MAX_REQUESTS` adlı bir ortam değişkeni ayarlayarak işlenecek maksimum istek sayısını yapılandırmaya izin verir.

### Worker'ları Manuel Olarak Yeniden Başlatma

Worker'ları [dosya değişikliklerinde](config.md#watching-for-file-changes) yeniden başlatmak mümkünken, tüm worker'ları [Caddy admin API](https://caddyserver.com/docs/api) aracılığıyla sorunsuz bir şekilde yeniden başlatmak da mümkündür. Yönetici [Caddyfile](config.md#caddyfile-config)'ınızda etkinleştirilmişse, yeniden başlatma uç noktasına aşağıdaki gibi basit bir POST isteği gönderebilirsiniz:

```console
curl -X POST http://localhost:2019/frankenphp/workers/restart
```

### Worker Hataları

Bir worker betiği sıfır olmayan bir çıkış koduyla çökerse, FrankenPHP onu üstel bir geri çekilme (exponential backoff) stratejisiyle yeniden başlatacaktır. Worker betiği, son geri çekilme süresinin 2 katından daha uzun süre çalışır durumda kalırsa, worker betiğini cezalandırmayacak ve tekrar yeniden başlatacaktır. Ancak, worker betiği kısa bir süre içinde sıfır olmayan bir çıkış koduyla başarısız olmaya devam ederse (örneğin, bir betikte yazım hatası olması durumunda), FrankenPHP `too many consecutive failures` hatasıyla çökecektir.

Ardışık hata sayısı, [Caddyfile](config.md#caddyfile-config)'ınızda `max_consecutive_failures` seçeneği ile yapılandırılabilir:

```caddyfile
frankenphp {
    worker {
        # ...
        max_consecutive_failures 10
    }
}
```

## Süper Küresel Değişkenlerin Davranışı

[PHP süper küresel değişkenleri](https://www.php.net/manual/en/language.variables.superglobals.php) (`$_SERVER`, `$_ENV`, `$_GET`...) aşağıdaki gibi davranır:

- `frankenphp_handle_request()`'e ilk çağrıdan önce, süper küresel değişkenler worker betiğinin kendisine bağlı değerleri içerir
- `frankenphp_handle_request()` çağrısı sırasında ve sonrasında, süper küresel değişkenler işlenen HTTP isteğinden üretilen değerleri içerir, `frankenphp_handle_request()`'e yapılan her çağrı süper küresel değişken değerlerini değiştirir

Geri çağırım içinde worker betiğinin süper küresel değişkenlerine erişmek için, bunları kopyalamalı ve kopyayı geri çağırımın kapsamına aktarmalısınız:

```php
<?php
// frankenphp_handle_request()'e ilk çağrıdan önce worker'ın $_SERVER süper küresel değişkenini kopyalayın
$workerServer = $_SERVER;

$handler = static function () use ($workerServer) {
    var_dump($_SERVER); // HTTP isteğine bağlı $_SERVER
    var_dump($workerServer); // Worker betiğinin $_SERVER'ı
};

// ...
