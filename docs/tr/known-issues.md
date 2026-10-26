---
title: FrankenPHP bilinen sorunlar, uyumsuz eklentiler ve geçici çözümler
description: Desteklenmeyen PHP eklentileri, musl libc uyarıları, 127.0.0.1 ile Docker TLS kurulumu ve Composer @php betikleri dahil olmak üzere FrankenPHP sınırlamalarının referansı.
---

# Bilinen sorunlar

## Desteklenmeyen PHP eklentileri

Aşağıdaki eklentilerin FrankenPHP ile uyumlu olmadığı bilinmektedir:

| Adı | Nedeni | Alternatifler |
|---|---|---|
| [imap](https://www.php.net/manual/imap.installation.php) | İş parçacığı güvenli değil | [javanile/php-imap2](https://github.com/javanile/php-imap2), [webklex/php-imap](https://github.com/Webklex/php-imap), [ImapEngine](https://github.com/DirectoryTree/ImapEngine) |
| [newrelic](https://docs.newrelic.com/docs/apm/agents/php-agent/getting-started/introduction-new-relic-php/) | İş parçacığı güvenli değil | - |
| [pcov](https://github.com/krakjoe/pcov) | İş parçacığı güvenli değil ([klasik modda SIGSEGV](https://github.com/krakjoe/pcov/issues/132)) | Kapsamı CLI SAPI ile toplayın (PHPUnit/Pest'i FrankenPHP dışında çalıştırın) |

## Sorunlu PHP eklentileri

Aşağıdaki eklentiler FrankenPHP ile kullanıldığında bilinen hatalara ve beklenmeyen davranışlara sahiptir:

| Adı | Sorun |
|---|---|
| [datadog](https://github.com/DataDog) | FrankenPHP'yi profilleyerken kararsızlık yaşıyor. Bu hata [DataDog tarafından takip ediliyor](https://github.com/DataDog/dd-trace-php/issues/3729). |
| [blackfire](https://blackfire.io/) | [FrankenPHP desteği beta aşamasındadır ve henüz özellikler tamamlanmamıştır](https://docs.blackfire.io/php/integrations/frankenphp). |
| [imagick](https://github.com/Imagick/imagick) | ImageMagick'in OpenMP iş parçacıkları FrankenPHP'nin iş parçacıklarıyla çakışarak kararsızlığa ve çökmelere neden olur. Bu durum, ImageMagick iş parçacıklarını `\Imagick::setResourceLimit(\Imagick::RESOURCETYPE_THREAD, 1)` aracılığıyla devre dışı bırakarak veya ImageMagick'i `--disable-openmp` ile derleyerek hafifletilebilir. OpenMP, statik ikili dosyalarda ve resmi `apt`/`apk`/`rpm` paketlerinde zaten devre dışıdır, bu nedenle yalnızca Docker imajları ve Homebrew kurulumları etkilenir. |

## PHP'nin sabit bellek sınırını aşmak tüm sunucuyu durdurur

PHP 8.2.35, 8.3.36, 8.4.27 ve 8.5.12'den beri, `memory_limit`'in aşılması betiği hemen durdurmaz: "Allowed memory size exhausted" (İzin verilen bellek boyutu tükendi) ölümcül hatası bir sonraki fonksiyon çağrısında veya döngü iterasyonunda yükseltilir.
Eğer betik, bundan önce sabit bellek sınırını aşacak şekilde bellek ayırırsa, PHP süreci sonlandırır.
Sabit sınır varsayılan olarak `memory_limit`'in iki katıdır (veya 1 GiB'ın üzerinde 1 GiB artı `memory_limit`'tir) ve `memory_hard_limit_percent` INI ayarı ile değiştirilebilir.

`memory_limit`'i aşan kodlar, örneğin `memory_limit`'ten daha büyük tek bir tahsis (büyük dizilerde `array_merge()` gibi) veya ardışık dize birleştirmeleri gibi durumlarda bu sınıra ulaşabilir.

PHP-FPM yalnızca bir çocuk sürecini kaybeder.
FrankenPHP, tüm istekleri tek bir sürecin iş parçacıklarından sunar, bu nedenle devam eden her istek kaybolur ve sunucu yeniden başlatılana kadar durur.
FrankenPHP'yi, Docker yeniden başlatma politikası veya systemd'nin `Restart=` (deb ve rpm paketlerinde zaten ayarlanmış olan) gibi onu yeniden başlatan bir denetleyici altında çalıştırın.

## get_browser

[get_browser()](https://www.php.net/manual/function.get-browser.php) fonksiyonu bir süre sonra kötü performans gösteriyor gibi görünüyor. Geçici bir çözüm, statik oldukları için User-Agent başına sonuçları önbelleğe almaktır (örneğin [APCu](https://www.php.net/manual/book.apcu.php) ile).

## Bağımsız ikili dosya ve Alpine tabanlı Docker imajları

Tamamen statik ikili dosya ve Alpine tabanlı Docker imajları (`dunglas/frankenphp:*-alpine`), daha küçük bir ikili dosya boyutu korumak için [glibc ve benzerleri](https://www.etalabs.net/compare_libcs.html) yerine [musl libc](https://musl.libc.org/) kullanır.
Bu durum bazı uyumluluk sorunlarına yol açabilir.
Özellikle, glob bayrağı `GLOB_BRACE` [mevcut değildir](https://www.php.net/manual/function.glob.php).

Sorun yaşarsanız, statik ikili dosyanın GNU varyantını ve Debian tabanlı Docker imajlarını kullanmayı tercih edin.

## Docker ile `https://127.0.0.1` kullanımı

FrankenPHP varsayılan olarak `localhost` için bir TLS sertifikası oluşturur.
Bu, yerel geliştirme için en kolay ve önerilen seçenektir.

Bunun yerine ana bilgisayar olarak `127.0.0.1` kullanmak istiyorsanız, sunucu adını `127.0.0.1` olarak ayarlayarak bunun için bir sertifika oluşturacak şekilde yapılandırmak mümkündür.

Ne yazık ki, [ağ sistemi](https://docs.docker.com/network/) nedeniyle Docker kullanırken bu yeterli değildir.
`curl: (35) LibreSSL/3.3.6: error:1404B438:SSL routines:ST_CONNECT:tlsv1 alert internal error` benzeri bir TLS hatası alırsınız.

Linux kullanıyorsanız, [ana bilgisayar ağ sürücüsünü](https://docs.docker.com/network/network-tutorial-host/) kullanmak bir çözümdür:

```console
docker run \
    -e SERVER_NAME="127.0.0.1" \
    -v $PWD:/app/public \
    --network host \
    dunglas/frankenphp
```

Ana bilgisayar ağ sürücüsü Mac ve Windows'ta desteklenmez. Bu platformlarda, konteynerin IP adresini tahmin etmeniz ve bunu sunucu adlarına dahil etmeniz gerekecektir.

`docker network inspect bridge` komutunu çalıştırın ve `Containers` anahtarının altındaki `IPv4Address` anahtarındaki son atanmış IP adresini belirlemek için bakın, ardından bu IP'yi bir artırın. Eğer hiçbir konteyner çalışmıyorsa, ilk atanan IP adresi genellikle `172.17.0.2`dir.

Ardından, bunu `SERVER_NAME` ortam değişkenine ekleyin:

```console
docker run \
    -e SERVER_NAME="127.0.0.1, 172.17.0.3" \
    -v $PWD:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

> [!CAUTION]
>
> `172.17.0.3`'ü konteynerinize atanacak IP ile değiştirdiğinizden emin olun.

Artık ana makineden `https://127.0.0.1` adresine erişebilmeniz gerekir.

Eğer durum böyle değilse, sorunu anlamaya çalışmak için FrankenPHP'yi hata ayıklama modunda başlatın:

```console
docker run \
    -e CADDY_GLOBAL_OPTIONS="debug" \
    -e SERVER_NAME="127.0.0.1" \
    -v $PWD:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

## `@php` referanslı Composer betikleri

[Composer betikleri](https://getcomposer.org/doc/articles/scripts.md) bazı görevler için bir PHP ikili dosyası çalıştırmak isteyebilir, örneğin [bir Laravel projesinde](laravel.md) `@php artisan package:discover --ansi` çalıştırmak. Bu durum [şu anda başarısız oluyor](https://github.com/php/frankenphp/issues/483#issuecomment-1899890915) ve iki nedeni vardır:

- Composer, FrankenPHP ikili dosyasını nasıl çağıracağını bilmiyor;
- Composer, FrankenPHP'nin henüz desteklemediği `-d` bayrağını kullanarak PHP ayarlarını komuta ekleyebilir.

Geçici bir çözüm olarak, `/usr/local/bin/php` içinde desteklenmeyen parametreleri silen ve ardından FrankenPHP'yi çağıran bir kabuk betiği oluşturabiliriz:

```bash
#!/usr/bin/env bash
# /usr/local/bin/php
# Parametreleri bir diziye kopyala
args=("$@")
index=0
# Parametreleri döngüye al
for i in "$@"
do
    # "-d" bayrağını ve sonraki parametreyi kaldır
    if [ "$i" == "-d" ]; then
        unset 'args[$index]'
        unset 'args[$index+1]'
    fi
    index=$((index+1))
done

/usr/local/bin/frankenphp php-cli "${args[@]}"
```

Ardından `PHP_BINARY` ortam değişkenini PHP betiğimizin yoluna ayarlayın ve Composer'ı çalıştırın:

```console
export PHP_BINARY=/usr/local/bin/php
composer install
```

## Statik ikili dosyalarla TLS/SSL sorunlarını giderme

Statik ikili dosyaları kullanırken, örneğin STARTTLS kullanarak e-posta gönderirken aşağıdaki TLS ile ilgili hatalarla karşılaşabilirsiniz:

```text
Unable to connect with STARTTLS: stream_socket_enable_crypto(): SSL operation failed with code 5. OpenSSL Error messages:
error:80000002:system library::No such file or directory
error:80000002:system library::No such file or directory
error:80000002:system library::No such file or directory
error:0A000086:SSL routines::certificate verify failed
```

Statik ikili dosya TLS sertifikalarını içermediği için, OpenSSL'i yerel CA sertifikaları kurulumunuza yönlendirmeniz gerekir.

CA sertifikalarının nereye yüklenmesi gerektiğini bulmak ve bunları bu konuma kaydetmek için [`openssl_get_cert_locations()`](https://www.php.net/manual/function.openssl-get-cert-locations.php) çıktısını inceleyin.

> [!WARNING]
>
> Web ve CLI bağlamlarının farklı ayarları olabilir.
> `openssl_get_cert_locations()` fonksiyonunu doğru bağlamda çalıştırdığınızdan emin olun.

Mozilla'dan çıkarılan CA sertifikaları [cURL sitesinden indirilebilir](https://curl.se/docs/caextract.html).

Alternatif olarak, Debian, Ubuntu ve Alpine dahil olmak üzere birçok dağıtım, bu sertifikaları içeren `ca-certificates` adlı paketler sunar.

OpenSSL'e CA sertifikalarını nerede arayacağını belirtmek için `SSL_CERT_FILE` ve `SSL_CERT_DIR` ortam değişkenlerini kullanmak da mümkündür:

```console
# TLS sertifikaları ortam değişkenlerini ayarla
export SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
export SSL_CERT_DIR=/etc/ssl/certs
