# FrankenPHP Worker'ları Kullanma

Uygulamanızı bir kez önyükleyin ve bellekte tutun.
FrankenPHP gelen istekleri birkaç milisaniye içinde halledecektir.

## Worker Betiklerinin Başlatılması

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

Ayrıca `--watch` seçeneğiyle [dosya değişikliklerinde worker'ı yeniden başlatmak](config.md#watching-for-file-changes) da mümkündür.
Aşağıdaki komut, `/path/to/your/app/` dizininde veya alt dizinlerinde `.php` ile biten herhangi bir dosya değiştirilirse yeniden başlatmayı tetikleyecektir:

```console
frankenphp php-server --worker /path/to/your/worker/script.php --watch="/path/to/your/app/**/*.php"
```

Bu özellik genellikle [hot reloading](hot-reload.md) ile birlikte kullanılır.

## Symfony Runtime

> [!TIP]
> Aşağıdaki bölüm yalnızca Symfony 7.4 öncesi için gereklidir; Symfony 7.4 ile FrankenPHP worker modu için yerel destek sunulmuştur.

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

Bkz. [ilgili doküman](laravel.md#laravel-octane).

## Özel Uygulamalar

Aşağıdaki örnek, üçüncü taraf bir kütüphaneye güvenmeden kendi worker betiğinizi nasıl oluşturacağınızı göstermektedir:

```php
<?php
// public/index.php

// Bir istemci bağlantısı kesildiğinde worker betiğinin sonlandırılmasını önleyin
ignore_user_abort(true);

// Uygulamanızı önyükleyin
require __DIR__.'/vendor/autoload.php';

$myApp = new \App\Kernel();
$myApp->boot();

// Daha iyi performans için döngü dışında işleyici (daha az iş yapıyor)
$handler = static function () use ($myApp) {
    try {
        // Bir istek alındığında çağrılır,
        // superglobals, php://input ve benzerleri sıfırlanır
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

### Worker'ları Manuel Olarak Yeniden Başlatın

Worker'ları [dosya değişikliklerinde](config.md#watching-for-file-changes) yeniden başlatmak mümkünken, tüm worker'ları
[Caddy yönetici API'si](https://caddyserver.com/docs/api) aracılığıyla sorunsuz bir şekilde yeniden başlatmak da mümkündür. Yönetici API'si
[Caddyfile](config.md#caddyfile-config) dosyanızda etkinleştirilmişse, yeniden başlatma uç noktasına aşağıdaki gibi basit bir POST isteği gönderebilirsiniz:

```console
curl -X POST http://localhost:2019/frankenphp/workers/restart
```

### Worker Hataları

Bir worker betiği sıfır olmayan bir çıkış koduyla çökerse, FrankenPHP onu üstel geri çekilme (exponential backoff) stratejisiyle yeniden başlatacaktır.
Eğer worker betiği son geri çekilmenin * 2 katından daha uzun süre çalışır durumda kalırsa,
worker betiğini cezalandırmayacak ve tekrar yeniden başlatacaktır.
Ancak, worker betiği kısa bir süre içinde sıfır olmayan bir çıkış koduyla başarısız olmaya devam ederse
(örneğin, bir betikte yazım hatası olması), FrankenPHP `too many consecutive failures` hatasıyla çökecektir.

Ardışık hata sayısı, [Caddyfile](config.md#caddyfile-config) dosyanızda `max_consecutive_failures` seçeneğiyle yapılandırılabilir:

```caddyfile
frankenphp {
    worker {
        # ...
        max_consecutive_failures 10
    }
}
```

## Süperküresel Değişkenlerin Davranışı

[PHP süperküresel değişkenleri](https://www.php.net/manual/en/language.variables.superglobals.php) (`$_SERVER`, `$_ENV`, `$_GET`...)
şu şekilde davranır:

- `frankenphp_handle_request()` fonksiyonunun ilk çağrılmasından önce, süperküresel değişkenler worker betiğinin kendisine bağlı değerleri içerir
- `frankenphp_handle_request()` çağrısı sırasında ve sonrasında, süperküresel değişkenler işlenmiş HTTP isteğinden üretilen değerleri içerir; `frankenphp_handle_request()` fonksiyonunun her çağrısı süperküresel değişkenlerin değerlerini değiştirir

Geri arama (callback) içinde worker betiğinin süperküresel değişkenlerine erişmek için, bunları kopyalamanız ve kopyayı geri aramanın kapsamına (scope) aktarmanız gerekir:

```php
<?php
// worker'ın $_SERVER süperküresel değişkenini frankenphp_handle_request() fonksiyonunun ilk çağrılmasından önce kopyalayın
$workerServer = $_SERVER;

$handler = static function () use ($workerServer) {
    var_dump($_SERVER); // İsteğe bağlı $_SERVER
    var_dump($workerServer); // worker betiğinin $_SERVER'ı
};

// ...
