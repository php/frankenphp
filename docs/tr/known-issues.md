# Bilinen Sorunlar

## Desteklenmeyen PHP Eklentileri

Aşağıdaki eklentilerin FrankenPHP ile uyumlu olmadığı bilinmektedir:

| Adı                                                                                                         | Nedeni                     | Alternatifleri                                                                                                       |
| :---------------------------------------------------------------------------------------------------------- | :------------------------- | :------------------------------------------------------------------------------------------------------------------- |
| [imap](https://www.php.net/manual/en/imap.installation.php)                                                 | İş parçacığı güvenli değil | [javanile/php-imap2](https://github.com/javanile/php-imap2), [webklex/php-imap](https://github.com/Webklex/php-imap) |
| [newrelic](https://docs.newrelic.com/docs/apm/agents/php-agent/getting-started/introduction-new-relic-php/) | İş parçacığı güvenli değil | -                                                                                                                    |

## Sorunlu PHP Eklentileri

Aşağıdaki eklentiler FrankenPHP ile kullanıldığında bilinen hatalara ve beklenmeyen davranışlara sahiptir:

| Adı                                                           | Problem                                                                                                                                                                                                                   |
| :------------------------------------------------------------ | :------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| [ext-openssl](https://www.php.net/manual/en/book.openssl.php) | musl libc kullanıldığında, OpenSSL eklentisi yoğun yük altında çökebilir. Bu sorun, daha popüler olan GNU libc kullanıldığında ortaya çıkmaz. Bu hata [PHP tarafından takip edilmektedir](https://github.com/php/php-src/issues/13648). |

## get_browser

[get_browser()](https://www.php.net/manual/en/function.get-browser.php) fonksiyonu bir süre sonra kötü performans gösteriyor gibi görünüyor. Geçici bir çözüm, statik oldukları için User Agent başına sonuçları önbelleğe almaktır (örneğin [APCu](https://www.php.net/manual/en/book.apcu.php) ile).

## Tek Başına İkili ve Alpine Tabanlı Docker İmajları

Tek başına ikili ve Alpine tabanlı Docker imajları (`dunglas/frankenphp:*-alpine`), daha küçük bir ikili boyutu korumak için [glibc ve arkadaşları](https://www.etalabs.net/compare_libcs.html) yerine [musl libc](https://musl.libc.org/) kullanır.
Bu durum bazı uyumluluk sorunlarına yol açabilir.
Özellikle, glob bayrağı `GLOB_BRACE` [mevcut değildir](https://www.php.net/manual/en/function.glob.php).

Sorunlarla karşılaşmanız durumunda, statik ikilinin GNU varyantını ve Debian tabanlı Docker imajlarını kullanmayı tercih edin.

## Docker ile `https://127.0.0.1` Kullanımı

FrankenPHP varsayılan olarak `localhost` için bir TLS sertifikası oluşturur.
Bu, yerel geliştirme için en kolay ve önerilen seçenektir.

Bunun yerine ana bilgisayar olarak `127.0.0.1` kullanmak istiyorsanız, sunucu adını `127.0.0.1` şeklinde ayarlayarak bunun için bir sertifika oluşturacak şekilde yapılandırmak mümkündür.

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

`docker network inspect bridge` komutunu çalıştırın ve `Containers` anahtarının altındaki `IPv4Address` anahtarındaki son atanmış IP adresini belirlemek için bakın ve bir artırın. Eğer hiçbir konteyner çalışmıyorsa, ilk atanan IP adresi genellikle `172.17.0.2`dir.

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
> `172.17.0.3` değerini konteynerinize atanacak IP ile değiştirdiğinizden emin olun.

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

## `@php` Referanslı Composer Betikleri

[Composer betikleri](https://getcomposer.org/doc/articles/scripts.md) bazı görevler için bir PHP ikilisi çalıştırmak isteyebilir, örneğin [bir Laravel projesinde](laravel.md) `@php artisan package:discover --ansi` çalıştırmak. Bu [şu anda başarısız oluyor](https://github.com/php/frankenphp/issues/483#issuecomment-1899890915) ve bunun iki nedeni var:

- Composer, FrankenPHP ikilisini nasıl çağıracağını bilmiyor;
- Composer, FrankenPHP'nin henüz desteklemediği `-d` bayrağını kullanarak komuta PHP ayarları ekleyebilir.

Geçici bir çözüm olarak, `/usr/local/bin/php` içinde desteklenmeyen parametreleri temizleyen ve ardından FrankenPHP'yi çağıran bir kabuk betiği oluşturabiliriz:

```bash
#!/usr/bin/env bash
args=("$@")
index=0
for i in "$@"
do
    if [ "$i" == "-d" ]; then
        unset 'args[$index]'
        unset 'args[$index+1]'
    fi
    index=$((index+1))
done

/usr/local/bin/frankenphp php-cli ${args[@]}
```

Ardından `PHP_BINARY` ortam değişkenini PHP betiğimizin yoluna ayarlayın ve Composer'ı çalıştırın:

```console
export PHP_BINARY=/usr/local/bin/php
composer install
```

## Statik İkililerle TLS/SSL Sorunlarını Giderme

Statik ikilileri kullanırken, örneğin STARTTLS kullanarak e-posta gönderirken aşağıdaki TLS ile ilgili hatalarla karşılaşabilirsiniz:

```text
Unable to connect with STARTTLS: stream_socket_enable_crypto(): SSL operation failed with code 5. OpenSSL Error messages:
error:80000002:system library::No such file or directory
error:80000002:system library::No such file or directory
error:80000002:system library::No such file or directory
error:0A000086:SSL routines::certificate verify failed
```

Statik ikili TLS sertifikalarını içermediğinden, OpenSSL'i yerel CA sertifikaları kurulumunuza yönlendirmeniz gerekir.

CA sertifikalarının nereye yüklenmesi gerektiğini bulmak ve bu konuma kaydetmek için [`openssl_get_cert_locations()`](https://www.php.net/manual/en/function.openssl-get-cert-locations.php) çıktısını inceleyin.

> [!WARNING]
>
> Web ve CLI bağlamları farklı ayarlara sahip olabilir.
> `openssl_get_cert_locations()` fonksiyonunu doğru bağlamda çalıştırdığınızdan emin olun.

[Mozilla'dan çıkarılan CA sertifikaları cURL sitesinden indirilebilir](https://curl.se/docs/caextract.html).

Alternatif olarak, Debian, Ubuntu ve Alpine dahil olmak üzere birçok dağıtım, bu sertifikaları içeren `ca-certificates` adlı paketler sağlar.

OpenSSL'e CA sertifikalarını nerede arayacağını belirtmek için `SSL_CERT_FILE` ve `SSL_CERT_DIR` değişkenlerini kullanmak da mümkündür:

```console
# TLS sertifikası ortam değişkenlerini ayarla
export SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
export SSL_CERT_DIR=/etc/ssl/certs
```
