# Statik Yapı Oluşturun

PHP kütüphanesinin yerel kurulumunu kullanmak yerine,
harika [static-php-cli projesi](https://github.com/crazywhalecc/static-php-cli) sayesinde FrankenPHP'nin statik veya çoğunlukla statik bir yapısını oluşturmak mümkündür (adına rağmen, bu proje sadece CLI'yı değil, tüm SAPI'leri destekler).

Bu yöntemle, tek, taşınabilir bir ikili PHP yorumlayıcısını, Caddy web sunucusunu ve FrankenPHP'yi içerecektir!

Tamamen statik yerel yürütülebilir dosyalar hiçbir bağımlılık gerektirmez ve hatta [`scratch` Docker imajı](https://docs.docker.com/build/building/base-images/#create-a-minimal-base-image-using-scratch) üzerinde bile çalıştırılabilir. Ancak, dinamik PHP eklentilerini (Xdebug gibi) yükleyemezler ve musl libc kullandıkları için bazı sınırlamalara sahiptirler.

Çoğunlukla statik ikililer yalnızca `glibc` gerektirir ve dinamik eklentileri yükleyebilir.

Mümkün olduğunda, glibc tabanlı, çoğunlukla statik yapıları kullanmanızı öneririz.

FrankenPHP ayrıca [PHP uygulamasının statik ikiliye gömülmesini](embed.md) destekler.

## Linux

Statik Linux ikilileri oluşturmak için Docker imajları sağlıyoruz:

### musl Tabanlı, Tamamen Statik Yapı

Hiçbir bağımlılık olmadan herhangi bir Linux dağıtımında çalışan ancak eklentilerin dinamik yüklenmesini desteklemeyen tamamen statik bir ikili için:

```console
docker buildx bake --load static-builder-musl
docker cp $(docker create --name static-builder-musl dunglas/frankenphp:static-builder-musl):/go/src/app/dist/frankenphp-linux-$(uname -m) frankenphp ; docker rm static-builder-musl
```

Yoğun eşzamanlı senaryolarda daha iyi performans için [mimalloc](https://github.com/microsoft/mimalloc) ayırıcısını kullanmayı düşünebilirsiniz.

```console
docker buildx bake --load --set static-builder-musl.args.MIMALLOC=1 static-builder-musl
```

### glibc Tabanlı, Çoğunlukla Statik Yapı (Dinamik Eklenti Desteği ile)

Seçilen eklentiler statik olarak derlenmiş olsa da PHP eklentilerini dinamik olarak yüklemeyi destekleyen bir ikili için:

```console
docker buildx bake --load static-builder-gnu
docker cp $(docker create --name static-builder-gnu dunglas/frankenphp:static-builder-gnu):/go/src/app/dist/frankenphp-linux-$(uname -m) frankenphp ; docker rm static-builder-gnu
```

Bu ikili tüm glibc 2.17 ve üzeri sürümlerini destekler ancak musl tabanlı sistemlerde (Alpine Linux gibi) çalışmaz.

Elde edilen çoğunlukla statik (glibc hariç) ikili `frankenphp` olarak adlandırılır ve geçerli dizinde mevcuttur.

Statik ikiliyi Docker olmadan oluşturmak istiyorsanız, Linux için de çalışan macOS talimatlarına bir göz atın.

### Özel Eklentiler

Varsayılan olarak, en popüler PHP eklentileri derlenir.

İkilinin boyutunu küçültmek ve saldırı yüzeyini azaltmak için, `PHP_EXTENSIONS` Docker ARG'sini kullanarak derlenecek eklentilerin listesini seçebilirsiniz.

Örneğin, yalnızca `opcache` eklentisini derlemek için aşağıdaki komutu çalıştırın:

```console
docker buildx bake --load --set static-builder-musl.args.PHP_EXTENSIONS=opcache,pdo_sqlite static-builder-musl
# ...
```

Etkinleştirdiğiniz eklentilere ek işlevler sağlayan kütüphaneler eklemek için `PHP_EXTENSION_LIBS` Docker ARG'sini kullanabilirsiniz:

```console
docker buildx bake \
  --load \
  --set static-builder-musl.args.PHP_EXTENSIONS=gd \
  --set static-builder-musl.args.PHP_EXTENSION_LIBS=libjpeg,libwebp \
  static-builder-musl
```

### Ekstra Caddy Modülleri

Ekstra Caddy modülleri eklemek veya [xcaddy](https://github.com/caddyserver/xcaddy)'ye diğer argümanları iletmek için `XCADDY_ARGS` Docker ARG'sini kullanın:

```console
docker buildx bake \
  --load \
  --set static-builder-musl.args.XCADDY_ARGS="--with github.com/darkweak/souin/plugins/caddy --with github.com/dunglas/caddy-cbrotli --with github.com/dunglas/mercure/caddy --with github.com/dunglas/vulcain/caddy" \
  static-builder-musl
```

Bu örnekte, Caddy için [Souin](https://souin.io) HTTP önbellek modülünün yanı sıra [cbrotli](https://github.com/dunglas/caddy-cbrotli), [Mercure](https://mercure.rocks) ve [Vulcain](https://vulcain.rocks) modüllerini ekliyoruz.

> [!TIP]
>
> cbrotli, Mercure ve Vulcain modülleri, `XCADDY_ARGS` boşsa veya ayarlanmamışsa varsayılan olarak dahil edilir.
> Eğer `XCADDY_ARGS` değerini özelleştirirseniz, dahil edilmelerini istiyorsanız bunları açıkça dahil etmelisiniz.

Derlemeyi nasıl [özelleştireceğinize](#customizing-the-build) de bakın.

### GitHub Token

GitHub API kullanım limitine ulaşırsanız, `GITHUB_TOKEN` adlı bir ortam değişkeninde bir GitHub Personal Access Token ayarlayın:

```console
GITHUB_TOKEN="xxx" docker --load buildx bake static-builder-musl
# ...
```

## macOS

macOS için statik bir ikili oluşturmak için aşağıdaki betiği çalıştırın ([Homebrew](https://brew.sh/) yüklü olmalıdır):

```console
git clone https://github.com/php/frankenphp
cd frankenphp
./build-static.sh
```

Not: Bu betik Linux'ta (ve muhtemelen diğer Unix'lerde) da çalışır ve sağladığımız Docker imajları tarafından dahili olarak kullanılır.

## Yapıyı Özelleştirme

Aşağıdaki ortam değişkenleri `docker build` ve `build-static.sh` betiğine aktarılabilir
statik yapıyı özelleştirmek için:

- `FRANKENPHP_VERSION`: kullanılacak FrankenPHP sürümü
- `PHP_VERSION`: kullanılacak PHP sürümü
- `PHP_EXTENSIONS`: oluşturulacak PHP eklentileri ([desteklenen eklentiler listesi](https://static-php.dev/en/guide/extensions.html))
- `PHP_EXTENSION_LIBS`: eklentilere özellikler ekleyen oluşturulacak ekstra kütüphaneler
- `XCADDY_ARGS`: [xcaddy](https://github.com/caddyserver/xcaddy)'ye iletilecek argümanlar, örneğin ekstra Caddy modülleri eklemek için
- `EMBED`: ikili dosyaya gömülecek PHP uygulamasının yolu
- `CLEAN`: ayarlandığında, libphp ve tüm bağımlılıkları sıfırdan oluşturulur (önbellek yok)
- `NO_COMPRESS`: UPX kullanarak ortaya çıkan ikiliyi sıkıştırma
- `DEBUG_SYMBOLS`: ayarlandığında, hata ayıklama sembolleri ayıklanmayacak ve ikili dosyaya eklenecektir
- `MIMALLOC`: (deneysel, yalnızca Linux) musl'un mallocng'sini [mimalloc](https://github.com/microsoft/mimalloc) ile daha iyi performans için değiştirir. Bunu yalnızca musl hedefli derlemeler için kullanmanızı öneririz, glibc için bu seçeneği devre dışı bırakmayı ve ikilinizi çalıştırırken [`LD_PRELOAD`](https://microsoft.github.io/mimalloc/overrides.html) kullanmayı tercih edin.
- `RELEASE`: (yalnızca bakımcılar) ayarlandığında, ortaya çıkan ikili dosya GitHub'a yüklenecektir

## Eklentiler

glibc veya macOS tabanlı ikililerle, PHP eklentilerini dinamik olarak yükleyebilirsiniz. Ancak, bu eklentilerin ZTS desteğiyle derlenmesi gerekecektir. Çoğu paket yöneticisi şu anda eklentilerinin ZTS sürümlerini sunmadığından, bunları kendiniz derlemeniz gerekecektir.

Bunun için `static-builder-gnu` Docker kapsayıcısını derleyebilir ve çalıştırabilir, içine uzaktan bağlanabilir ve eklentileri `./configure --with-php-config=/go/src/app/dist/static-php-cli/buildroot/bin/php-config` ile derleyebilirsiniz.

[Xdebug eklentisi](https://xdebug.org) için örnek adımlar:

```console
docker build -t gnu-ext -f static-builder-gnu.Dockerfile --build-arg FRANKENPHP_VERSION=1.0 .
docker create --name static-builder-gnu -it gnu-ext /bin/sh
docker start static-builder-gnu
docker exec -it static-builder-gnu /bin/sh
cd /go/src/app/dist/static-php-cli/buildroot/bin
git clone https://github.com/xdebug/xdebug.git && cd xdebug
source scl_source enable devtoolset-10
../phpize
./configure --with-php-config=/go/src/app/dist/static-php-cli/buildroot/bin/php-config
make
exit
docker cp static-builder-gnu:/go/src/app/dist/static-php-cli/buildroot/bin/xdebug/modules/xdebug.so xdebug-zts.so
docker cp static-builder-gnu:/go/src/app/dist/frankenphp-linux-$(uname -m) ./frankenphp
docker stop static-builder-gnu
docker rm static-builder-gnu
docker rmi gnu-ext
```

Bu, mevcut dizinde `frankenphp` ve `xdebug-zts.so`'yu oluşturmuş olacaktır. `xdebug-zts.so`'yu uzantı dizininize taşırsanız, php.ini dosyanıza `zend_extension=xdebug-zts.so` ekler ve FrankenPHP'yi çalıştırırsanız, Xdebug'ı yükleyecektir.
