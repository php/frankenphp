---
title: FrankenPHP worker modu: PHP uygulamanızı bellekte tutun
description: FrankenPHP'yi worker modunda çalıştırarak PHP uygulamanızı istekler arasında önyüklü tutun, önyükleme maliyetini azaltın ve yanıtları milisaniyeler içinde sunun.
---

# FrankenPHP worker'ları kullanma

Uygulamanızı bir kez önyükleyin ve bellekte tutun.
FrankenPHP gelen istekleri birkaç milisaniye içinde halledecektir.

## FrankenPHP worker betiklerini başlatma

### Docker ile FrankenPHP worker çalıştırma

`FRANKENPHP_CONFIG` ortam değişkeninin değerini `worker /path/to/your/worker/script.php` olarak ayarlayın:

```bash
docker run \
    -e FRANKENPHP_CONFIG="worker /app/path/to/your/worker/script.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### Bağımsız ikili ile FrankenPHP worker çalıştırma

Geçerli dizinin içeriğini bir worker kullanarak sunmak için `php-server` komutunun `--worker` seçeneğini kullanın:

```bash
frankenphp php-server --worker /path/to/your/worker/script.php
```

PHP uygulamanız [ikili dosyaya gömülü](embed.md) ise, uygulamanın kök dizinine özel bir `Caddyfile` ekleyebilirsiniz.
Otomatik olarak kullanılacaktır.

Dosya değişikliklerinde worker'ı yeniden başlatmak ([dosya değişikliklerini izleme](config.md#dosya-değişikliklerini-izleme)) `--watch` seçeneğiyle de mümkündür.
Aşağıdaki komut, `/path/to/your/app/` dizininde veya alt dizinlerde `.php` ile biten herhangi bir dosya değiştirilirse yeniden başlatmayı tetikleyecektir:

```bash
frankenphp php-server --worker /path/to/your/worker/script.php --watch="/path/to/your/app/**/*.php"
```

Bu özellik genellikle [hot reloading](hot-reload.md) ile birlikte kullanılır.

## Symfony için Worker Modu

Bkz. [FrankenPHP Symfony worker modu dokümantasyonu](symfony.md#symfony-worker-mode-with-frankenphp).

## Laravel Octane için Worker Modu

Bkz. [FrankenPHP Laravel Octane dokümantasyonu](laravel.md#laravel-octane).

## Özel bir FrankenPHP worker betiği yazma

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

```bash
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

Varsayılan olarak, CPU başına 2 worker başlatılır.
Başlatılacak worker sayısını da yapılandırabilirsiniz:

```bash
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php 42" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### PSR-15 Kullanma

Uygulamanız süper küresel değişkenler yerine [PSR-15](https://www.php-fig.org/psr/psr-15/) ile iletişim kuruyorsa, işleyicinin kenarlarındaki istek/yanıtı [nyholm/psr7](https://github.com/Nyholm/psr7) ve [nyholm/psr7-server](https://github.com/Nyholm/psr7-server) ile dönüştürün:

```console
composer require nyholm/psr7 nyholm/psr7-server psr/http-server-handler
```

```php
<?php
// public/index.php

require __DIR__.'/vendor/autoload.php';

use Nyholm\Psr7\Factory\Psr17Factory;
use Nyholm\Psr7Server\ServerRequestCreator;

$myApp = new \App\Kernel(); // implements Psr\Http\Server\RequestHandlerInterface
$myApp->boot();

$psr17Factory = new Psr17Factory();
$creator = new ServerRequestCreator(
    $psr17Factory, // ServerRequestFactory
    $psr17Factory, // UriFactory
    $psr17Factory, // UploadedFileFactory
    $psr17Factory, // StreamFactory
);

$handler = static function () use ($myApp, $creator) {
    $response = $myApp->handle($creator->fromGlobals());

    http_response_code($response->getStatusCode());
    foreach ($response->getHeaders() as $name => $values) {
        foreach ($values as $value) {
            header("$name: $value", false);
        }
    }
    echo $response->getBody();
};

$maxRequests = (int)($_SERVER['MAX_REQUESTS'] ?? 0);
for ($nbRequests = 0; !$maxRequests || $nbRequests < $maxRequests; ++$nbRequests) {
    $keepRunning = \frankenphp_handle_request($handler);
    gc_collect_cycles();
    if (!$keepRunning) break;
}
```

### Belirli sayıda istekten sonra worker'ı yeniden başlatın

PHP başlangıçta uzun süreli işlemler için tasarlanmadığından, hala bellek sızdıran birçok kütüphane ve eski kod vardır.
Bu tür kodları worker modunda kullanmak için geçici bir çözüm, belirli sayıda isteği işledikten sonra worker betiğini yeniden başlatmaktır:

Önceki worker kod parçacığı, `MAX_REQUESTS` adlı bir ortam değişkeni ayarlayarak işlenecek maksimum istek sayısını yapılandırmaya izin verir.

### Worker'ları manuel olarak yeniden başlatma

Worker'ları [dosya değişikliklerinde](config.md#dosya-değişikliklerini-izleme) yeniden başlatmak mümkünken, tüm worker'ları
[Caddy yönetici API'si](https://caddyserver.com/docs/api) aracılığıyla sorunsuz bir şekilde yeniden başlatmak da mümkündür. Yönetici
[Caddyfile](config.md#caddyfile-yapılandırması)'ınızda etkinleştirilmişse, yeniden başlatma uç noktasına aşağıdaki gibi basit bir POST isteği gönderebilirsiniz:

```bash
curl -X POST http://localhost:2019/frankenphp/workers/restart
```

### Worker hataları

Bir worker betiği sıfır olmayan bir çıkış koduyla çökerse, FrankenPHP onu üstel bir geri çekilme (exponential backoff) stratejisiyle yeniden başlatacaktır.
Worker betiği, son geri çekilme süresinin 2 katından daha uzun süre çalışır durumda kalırsa,
worker betiğini cezalandırmayacak ve tekrar yeniden başlatacaktır.
Ancak, worker betiği kısa bir süre içinde sıfır olmayan bir çıkış koduyla başarısız olmaya devam ederse
(örneğin, bir betikte yazım hatası olması durumunda), FrankenPHP `too many consecutive failures` hatasıyla çökecektir.

Ardışık hata sayısı, [Caddyfile](config.md#caddyfile-yapılandırması)'ınızda `max_consecutive_failures` seçeneği ile yapılandırılabilir:

```caddyfile
frankenphp {
    worker {
        # ...
        max_consecutive_failures 10
    }
}
```

### İş parçacığı sayısı

Worker modu etkinleştirildiğinde, gelen istekleri işlemek için ek bir iş parçacığı gerektiğinden, varsayılan `num_threads` değeri `1 + 2x(kullanılabilir CPU sayısı)` şeklindedir.

Bu nedenle, bu modda `num_threads` ve `num` açıkça ayarlanmışsa, `num_threads`, `num`'dan kesinlikle büyük olmalıdır.

## Süper küresel değişkenlerin davranışı

[PHP süper küresel değişkenleri](https://www.php.net/manual/language.variables.superglobals.php) (`$_SERVER`, `$_ENV`, `$_GET`...)
aşağıdaki gibi davranır:

- `frankenphp_handle_request()`'e ilk çağrıdan önce, süper küresel değişkenler worker betiğinin kendisine bağlı değerleri içerir
- `frankenphp_handle_request()` çağrısı sırasında ve sonrasında, süper küresel değişkenler işlenen HTTP isteğinden üretilen değerleri içerir, `frankenphp_handle_request()`'e yapılan her çağrı süper küresel değişken değerlerini değiştirir

Geri çağırım içinde worker betiğinin süper küresel değişkenlerine erişmek için, bunları kopyalamalı ve kopyayı geri çağırımın kapsamına aktarmalısınız:

```php
<?php
// frankenphp_handle_request()'e ilk çağrıdan önce worker'ın $_SERVER süper küresel değişkenini kopyalayın
$workerServer = $_SERVER;

$handler = static function () use ($workerServer) {
    var_dump($_SERVER); // İsteğe bağlı $_SERVER
    var_dump($workerServer); // Worker betiğinin $_SERVER'ı
};

// ...
```

Çoğu süper küresel değişken (`$_GET`, `$_POST`, `$_COOKIE`, `$_FILES`, `$_SERVER`, `$_REQUEST`) istekler arasında otomatik olarak sıfırlanır.
Ancak, **`$_ENV` şu anda istekler arasında sıfırlanmamaktadır**.
Bu, bir istek sırasında `$_ENV` üzerinde yapılan herhangi bir değişikliğin kalıcı olacağı ve aynı worker iş parçacığı tarafından işlenen sonraki istekler tarafından görüleceği anlamına gelir.
İsteğe özgü veya hassas verileri `$_ENV` içinde saklamaktan kaçının.

## Durum Kalıcılığı

Worker modu, PHP sürecini istekler arasında canlı tuttuğu için, aşağıdaki durum istekler arasında kalıcı olur:

- **Statik değişkenler**: Fonksiyonlar veya metotlar içinde `static` ile bildirilen değişkenler, istekler arasında değerlerini korur.
- **Sınıf statik özellikleri**: Sınıflardaki statik özellikler, istekler arasında kalıcı olur.
- **Küresel değişkenler**: Worker betiğinin küresel kapsamındaki değişkenler, istekler arasında kalıcı olur.
- **Bellek içi önbellekler**: İstek işleyicisi dışında bellekte depolanan herhangi bir veri (diziler, nesneler) kalıcı olur.
- **Çalışma zamanı motoru ayarları**: `ini_set()`, `stream_context_set_default()`, `date_default_timezone_set()`, `chdir()`, `stream_wrapper_register()`, `set_error_handler()` veya `set_exception_handler()` ile yapılan değişiklikler sonraki istekler için geçerliliğini korur. Varsayılan akış bağlamında kimlik bilgileri gibi isteğe veya kullanıcıya özgü değerleri asla orada saklamayın: bunun yerine her çağrıya açık bir bağlam geçirin.
- **Süreç genelindeki ayarlar**: `umask()` ve `setlocale()` tüm FrankenPHP sürecini değiştirir, yalnızca mevcut iş parçacığını değil. Worker modu bunları istekler arasında geri yüklemez ve her modda aynı anda çalışan istekleri de etkiler. Bunları başlangıçta bir kez ayarlayın veya umask'ı değiştirmek yerine oluşturulan dosyalarda `chmod()` çağırın.

Bu, tasarım gereğidir ve worker modunu hızlı yapan şey budur. Ancak, istenmeyen yan etkilerden kaçınmak için dikkat gerektirir:

```php
<?php
function getCounter(): int {
    static $count = 0;
    return ++$count; // İstekler arasında artar!
}

$handler = static function () {
    echo getCounter(); // Bu iş parçacığındaki her istek için 1, 2, 3, ...
};

while (\frankenphp_handle_request($handler)) {
    // ...
}
```

Worker betikleri yazarken, istekler arasında isteğe özgü herhangi bir durumu sıfırladığınızdan emin olun.
[Symfony](symfony.md) ve [Laravel Octane](laravel.md) gibi framework'ler çoğu durumu sizin için sıfırlarken, kendi servislerinizi yine de sıfırlamanız gerekebilir. Symfony ile, isteğe özgü durum tutan servisler, kernel tarafından istekler arasında sıfırlanmaları için [`Symfony\Contracts\Service\ResetInterface`](https://github.com/symfony/contracts/blob/main/Service/ResetInterface.php) uygulamasını implemente etmelidir.
