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

### Binary Çıktısı

Geçerli dizinin içeriğini bir worker kullanarak sunmak için `php-server` komutunun `--worker` seçeneğini kullanın:

```console
frankenphp php-server --worker /path/to/your/worker/script.php
```

PHP uygulamanız [binary dosyaya gömülü](embed.md) ise, uygulamanın kök dizinine özel bir `Caddyfile` ekleyebilirsiniz.
Otomatik olarak kullanılacaktır.

## Symfony Çalışma Zamanı

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

Aşağıdaki örnek, üçüncü taraf bir kütüphaneye güvenmeden kendi çalışan kodunuzu nasıl oluşturacağınızı göstermektedir:

```php
<?php
// public/index.php

// Bir istemci bağlantısı kesildiğinde alt komut dosyasının sonlandırılmasını önleyin
ignore_user_abort(true);

// Uygulamanızı önyükleyin
require __DIR__.'/vendor/autoload.php';

$myApp = new \App\Kernel();
$myApp->boot();

// Daha iyi performans için döngü dışında işleyici (daha az iş yapıyor)
$handler = static function () use ($myApp) {
    // Bir istek alındığında çağrılır,
    // superglobals, php://input ve benzerleri sıfırlanır
    echo $myApp->handle($_GET, $_POST, $_COOKIE, $_FILES, $_SERVER);
};

for ($nbRequests = 0, $running = true; isset($_SERVER['MAX_REQUESTS']) && ($nbRequests < ((int)$_SERVER['MAX_REQUESTS'])) && $running; ++$nbRequests) {
    $running = \frankenphp_handle_request($handler);

    // HTTP yanıtını gönderdikten sonra bir şey yapın
    $myApp->terminate();

    // Bir sayfa oluşturmanın ortasında tetiklenme olasılığını azaltmak için çöp toplayıcıyı çağırın
    gc_collect_cycles();
}

// Temizleme
$myApp->shutdown();
```

Ardından, uygulamanızı başlatın ve çalışanınızı yapılandırmak için `FRANKENPHP_CONFIG` ortam değişkenini kullanın:

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

<!-- textlint-disable -->

PHP başlangıçta uzun süreli işlemler için tasarlanmadığından, hala bellek sızdıran birçok kütüphane ve eski kod vardır.

<!-- textlint-enable -->

Bu tür kodları worker modunda kullanmak için geçici bir çözüm, belirli sayıda isteği işledikten sonra worker betiğini yeniden başlatmaktır:

Önceki worker kod parçacığı, `MAX_REQUESTS` adlı bir ortam değişkeni ayarlayarak işlenecek maksimum istek sayısını yapılandırmaya izin verir.
