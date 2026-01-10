# Bilinen Sorunlar

## Desteklenmeyen PHP Eklentileri

Aşağıdaki eklentilerin FrankenPHP ile uyumlu olmadığı bilinmektedir:

| Adı                                                         | Nedeni                     | Alternatifleri                                                                                                       |
| ----------------------------------------------------------- | -------------------------- | -------------------------------------------------------------------------------------------------------------------- |
| [imap](https://www.php.net/manual/en/imap.installation.php) | İş parçacığı güvenli değil | [javanile/php-imap2](https://github.com/javanile/php-imap2), [webklex/php-imap](https://github.com/Webklex/php-imap) |

## Sorunlu PHP Eklentileri

Aşağıdaki eklentiler FrankenPHP ile kullanıldığında bilinen hatalara ve beklenmeyen davranışlara sahiptir:

| Adı | Problem |
| --- | ------- |

## get_browser

[get_browser()](https://www.php.net/manual/en/function.get-browser.php) fonksiyonu bir süre sonra kötü performans gösteriyor gibi görünüyor. Geçici bir çözüm, statik oldukları için User-Agent başına sonuçları önbelleğe almaktır (örneğin [APCu](https://www.php.net/manual/en/book.apcu.php) ile).

## Binary Çıktısı ve Alpine Tabanlı Docker İmajları

Binary çıktısı ve Alpine tabanlı Docker imajları (dunglas/frankenphp:\*-alpine), daha küçük bir binary boyutu korumak için glibc ve arkadaşları yerine musl libc kullanır. Bu durum bazı uyumluluk sorunlarına yol açabilir. Özellikle, glob seçeneği GLOB_BRACE mevcut değildir.

## Docker ile `https://127.0.0.1` Kullanımı

FrankenPHP varsayılan olarak `localhost` için bir TLS sertifikası oluşturur.
Bu, yerel geliştirme için en kolay ve önerilen seçenektir.

Bunun yerine ana bilgisayar olarak `127.0.0.1` kullanmak istiyorsanız, sunucu adını `127.0.0.1` şeklinde ayarlayarak bunun için bir sertifika oluşturacak yapılandırma yapmak mümkündür.

Ne yazık ki, [ağ sistemi](https://docs.docker.com/network/) nedeniyle Docker kullanırken bu yeterli değildir.
`Curl: (35) LibreSSL/3.3.6: error:1404B438:SSL routines:ST_CONNECT:tlsv1 alert internal error`'a benzer bir TLS hatası alırsınız.

Linux kullanıyorsanız, [ana bilgisayar ağ sürücüsünü](https://docs.docker.com/network/network-tutorial-host/) kullanmak bir çözümdür:

```console
docker run \
    -e SERVER_NAME="127.0.0.1" \
    -v $PWD:/app/public \
    --network host \
    dunglas/frankenphp
```

Ana bilgisayar ağ sürücüsü Mac ve Windows'ta desteklenmez. Bu platformlarda, konteynerin IP adresini tahmin etmeniz ve bunu sunucu adlarına dahil etmeniz gerekecektir.

`docker network inspect bridge`'i çalıştırın ve `IPv4Address` anahtarının altındaki son atanmış IP adresini belirlemek için `Containers` anahtarına bakın ve bir artırın. Eğer hiçbir konteyner çalışmıyorsa, ilk atanan IP adresi genellikle `172.17.0.2`dir.

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
> 172.17.0.3`ü konteynerinize atanacak IP ile değiştirdiğinizden emin olun.

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

## `@php` Referanslı Composer Betikler

[Composer betikleri](https://getcomposer.org/doc/articles/scripts.md) bazı görevler için bir PHP binary çalıştırmak isteyebilir, örneğin [bir Laravel projesinde](laravel.md) `@php artisan package:discover --ansi` çalıştırmak. Bu [şu anda mümkün değil](https://github.com/php/frankenphp/issues/483#issuecomment-1899890915) ve 2 nedeni var:

- Composer FrankenPHP binary dosyasını nasıl çağıracağını bilmiyor;
- Composer, FrankenPHP'nin henüz desteklemediği `-d` bayrağını kullanarak PHP ayarlarını komuta ekleyebilir.

Geçici bir çözüm olarak, `/usr/local/bin/php` içinde desteklenmeyen parametreleri silen ve ardından FrankenPHP'yi çağıran bir kabuk betiği oluşturabiliriz:

```bash
#!/bin/bash
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

Ardından `PHP_BINARY` ortam değişkenini PHP betiğimizin yoluna ayarlayın ve Composer bu yolla çalışacaktır:

```bash
export PHP_BINARY=/usr/local/bin/php
composer install
```
