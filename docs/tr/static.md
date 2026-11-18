---
title: FrankenPHP statik yapısı: tek-binary PHP uygulama sunucusu
description: Linux ve macOS'ta gömülü PHP ve Caddy ile statik veya çoğunlukla statik FrankenPHP binary'leri oluşturun, eklentileri özelleştirin ve glibc ile dinamik modülleri yükleyin.
---

# Statik bir yapı oluşturun

PHP kütüphanesinin yerel kurulumunu kullanmak yerine, harika [static-php-cli projesi](https://github.com/crazywhalecc/static-php-cli) sayesinde FrankenPHP'nin statik veya çoğunlukla statik bir yapısını oluşturmak mümkündür (adına rağmen, bu proje yalnızca CLI'yi değil, tüm SAPI'leri destekler).

Bu yöntemle, tek, taşınabilir bir ikili PHP yorumlayıcısını, Caddy web sunucusunu ve FrankenPHP'yi içerecektir!

Tamamen statik yerel yürütülebilir dosyalar hiçbir bağımlılık gerektirmez ve hatta bir [\`scratch\` Docker imajında](https://docs.docker.com/build/building/base-images/#create-a-minimal-base-image-using-scratch) bile çalıştırılabilir. Ancak, dinamik PHP eklentilerini (Xdebug gibi) yükleyemezler ve musl libc kullandıkları için bazı sınırlamalara sahiptirler.

Çoğunlukla statik binary'ler yalnızca \`glibc\` gerektirir ve dinamik eklentileri yükleyebilir.

Mümkün olduğunda, glibc tabanlı, çoğunlukla statik yapıları kullanmanızı öneririz.

FrankenPHP ayrıca [PHP uygulamasının statik binary'ye gömülmesini](embed.md) destekler.

## Linux

Statik Linux binary'leri oluşturmak için Docker imajları sağlıyoruz:

### musl tabanlı, tamamen statik yapı

Bağımlılık olmaksızın herhangi bir Linux dağıtımında çalışan ancak eklentilerin dinamik olarak yüklenmesini desteklemeyen tamamen statik bir binary için:

```console
docker buildx bake --load static-builder-musl
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log dunglas/frankenphp:static-builder-musl
cp "output/frankenphp-linux-$(uname -m)" frankenphp
```

Yoğun eşzamanlı senaryolarda daha iyi performans için, [mimalloc](https://github.com/microsoft/mimalloc) ayırıcısını kullanmayı düşünebilirsiniz.

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log -e MIMALLOC=1 dunglas/frankenphp:static-builder-musl
```

### glibc tabanlı, çoğunlukla statik yapı (dinamik eklenti desteği ile)

Seçilen eklentiler statik olarak derlenmiş olsa da, PHP eklentilerini dinamik olarak yüklemeyi destekleyen bir binary için:

```console
docker buildx bake --load static-builder-gnu
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log dunglas/frankenphp:static-builder-gnu
cp "output/frankenphp-linux-$(uname -m)" frankenphp
```

Bu binary, glibc'nin 2.28 ve üzeri tüm sürümlerini destekler ancak musl tabanlı sistemlerde (Alpine Linux gibi) çalışmaz.

Ortaya çıkan çoğunlukla statik (glibc hariç) binary `frankenphp` olarak adlandırılır ve mevcut dizinde bulunur.

Eğer yapılandırma başarısız olursa, static-php-cli günlükleri `output/log/` dizinindedir.

Eğer statik binary'yi Docker olmadan oluşturmak isterseniz, Linux için de çalışan macOS talimatlarına göz atın.

### Statik yapıda özel PHP eklentileri

Varsayılan olarak, en popüler PHP eklentileri derlenir. Varsayılanlar [`craft.yml`](https://github.com/php/frankenphp/blob/main/craft.yml) dosyasında bulunur.

Binary dosyanın boyutunu küçültmek ve saldırı yüzeyini azaltmak için `PHP_EXTENSIONS` değişkenini kullanarak derlenecek eklentilerin listesini seçebilirsiniz.

Örneğin, yalnızca `opcache` eklentisini derlemek için aşağıdaki komutu çalıştırın:

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log -e PHP_EXTENSIONS=opcache,pdo_sqlite dunglas/frankenphp:static-builder-musl
# ...
```

Etkinleştirdiğiniz eklentilere ek işlevler sağlayan kütüphaneler eklemek için `PHP_EXTENSION_LIBS` değişkenini geçirebilirsiniz:

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log \
  -e PHP_EXTENSIONS=gd \
  -e PHP_EXTENSION_LIBS=libjpeg,libwebp \
  dunglas/frankenphp:static-builder-musl
```

### Ekstra Caddy modülleri

Ekstra Caddy modülleri eklemek veya [xcaddy](https://github.com/caddyserver/xcaddy) adresine başka argümanlar iletmek için `XCADDY_ARGS` değişkenini kullanın:

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log \
  -e XCADDY_ARGS="--with github.com/darkweak/souin/plugins/caddy --with github.com/dunglas/caddy-cbrotli --with github.com/dunglas/mercure/caddy --with github.com/dunglas/vulcain/caddy" \
  dunglas/frankenphp:static-builder-musl
```

Bu örnekte, Caddy için [Souin](https://souin.io) HTTP önbellek modülünün yanı sıra [cbrotli](https://github.com/dunglas/caddy-cbrotli), [Mercure](https://mercure.rocks) ve [Vulcain](https://vulcain.rocks) modüllerini ekliyoruz.

> [!TIP]
>
> cbrotli, Mercure ve Vulcain modülleri, `XCADDY_ARGS` boşsa veya ayarlanmamışsa varsayılan olarak dahil edilir.
> Eğer `XCADDY_ARGS` değerini özelleştirirseniz, dahil edilmelerini istiyorsanız bunları açıkça dahil etmelisiniz.

Ayrıca [FrankenPHP statik yapısını nasıl özelleştireceğinize](#frankenphp-statik-yapısını-özelleştirme) de bakın.

### GitHub token

GitHub API kullanım limitine ulaşırsanız, `GITHUB_TOKEN` adlı bir ortam değişkeninde bir GitHub Personal Access Token ayarlayın:

```console
docker run --rm -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log -e GITHUB_TOKEN="xxx" dunglas/frankenphp:static-builder-musl
# ...
```

## macOS

macOS için statik bir binary oluşturmak için aşağıdaki betiği çalıştırın ([Homebrew](https://brew.sh/) yüklü olmalıdır):

```console
git clone https://github.com/php/frankenphp
cd frankenphp
./build-static.sh
```

Not: Bu betik Linux'ta (ve muhtemelen diğer Unix'lerde) da çalışır ve sağladığımız Docker imajları tarafından dahili olarak kullanılır.

Yalnızca Apple Silicon için macOS binary'leri yayınlıyoruz. Intel Mac'lerde, bu betik ile kaynaktan derleyin.

## FrankenPHP statik yapısını özelleştirme

Aşağıdaki ortam değişkenleri `docker run` komutuna ve `build-static.sh` betiğine, statik yapıyı özelleştirmek için geçirilebilir:

- `FRANKENPHP_VERSION`: kullanılacak FrankenPHP sürümü
- `PHP_VERSION`: kullanılacak PHP sürümü
- `PHP_EXTENSIONS`: oluşturulacak PHP eklentileri ([desteklenen eklentiler listesi](https://static-php.dev/en/guide/extensions.html))
- `PHP_EXTENSION_LIBS`: eklentilere özellikler ekleyen oluşturulacak ekstra kütüphaneler
- `XCADDY_ARGS`: [xcaddy](https://github.com/caddyserver/xcaddy) adresine iletilecek argümanlar, örneğin ekstra Caddy modülleri eklemek için
- `EMBED`: binary dosyaya gömülecek PHP uygulamasının yolu
- `CLEAN`: ayarlandığında, libphp ve tüm bağımlılıkları sıfırdan oluşturulur (önbellek yok)
- `COMPRESS`: `1` olarak ayarlandığında, ortaya çıkan binary'yi UPX ile paketler (yalnızca Linux; `DEBUG_SYMBOLS` ayarlanmışsa yok sayılır)
- `DEBUG_SYMBOLS`: ayarlandığında, hata ayıklama sembolleri ayıklanmayacak ve binary dosyaya eklenecektir
- `MIMALLOC`: (deneysel, yalnızca Linux) geliştirilmiş performans için musl'un mallocng'sini [mimalloc](https://github.com/microsoft/mimalloc) ile değiştirir. Bunu yalnızca musl hedefleyen yapılar için kullanmanızı öneririz, glibc için bu seçeneği devre dışı bırakmayı ve bunun yerine binary'nizi çalıştırırken [`LD_PRELOAD`](https://microsoft.github.io/mimalloc/overrides.html) kullanmayı tercih edin.
- `RELEASE`: (yalnızca bakımcılar) ayarlandığında, ortaya çıkan binary dosya GitHub'a yüklenecektir
- `LIBC`: (yalnızca Linux) tamamen statik bir binary için `musl` (varsayılan) veya sistem glibc'sine dinamik olarak bağlanmak için `gnu`. static-php-cli'nin doğrudan ayarlanabilen `SPC_TARGET` ve `SPC_TOOLCHAIN` için bir kısaltmadır; statik-php-cli toolchain'ini `craft.yml` dosyasını okumadan önce çözdüğü için bunların gerçek ortam değişkenleri olması gerekir. `musl` varsayılanı, mimarinin temel CPU'sunu hedefler (`x86_64-linux-musl` veya `aarch64-linux-musl`) böylece binary herhangi bir CPU'da çalışır; bunun yerine derleme makinesi için optimize etmek üzere `SPC_TARGET=native-native-musl` olarak ayarlayın.

## Statik binary'de PHP eklentilerini dinamik olarak yükleme

glibc veya macOS tabanlı binary'ler ile PHP eklentilerini dinamik olarak yükleyebilirsiniz. Ancak, bu eklentilerin ZTS desteğiyle derlenmesi gerekecektir.
Çoğu paket yöneticisi şu anda eklentilerinin ZTS sürümlerini sunmadığından, bunları kendiniz derlemeniz gerekecektir.

Bunun için, `static-builder-gnu` Docker container'ını çalıştırabilir, içine uzaktan bağlanabilir ve eklentileri `./configure --with-php-config=/go/src/app/dist/buildroot/bin/php-config` ile derleyebilirsiniz.

[Xdebug eklentisi](https://xdebug.org) için örnek adımlar:

```console
docker buildx bake --load static-builder-gnu
docker run --rm -v "$PWD/dist:/go/src/app/dist" -v "$PWD/output:/output" -e OUTPUT_DIR=/output -e SPC_LOGS_DIR=/output/log dunglas/frankenphp:static-builder-gnu
docker run --rm -it -v "$PWD/dist:/go/src/app/dist" dunglas/frankenphp:static-builder-gnu /bin/bash
cd /go/src/app/dist/buildroot/bin
git clone https://github.com/xdebug/xdebug.git && cd xdebug
../phpize
./configure --with-php-config=/go/src/app/dist/buildroot/bin/php-config
make
exit
cp dist/buildroot/bin/xdebug/modules/xdebug.so xdebug-zts.so
cp "output/frankenphp-linux-$(uname -m)" frankenphp
```

Bu, mevcut dizinde `frankenphp` ve `xdebug-zts.so` dosyalarını oluşturacaktır. Eğer `xdebug-zts.so` dosyasını eklenti dizininize taşır, php.ini dosyanıza `zend_extension=xdebug-zts.so` eklersiniz ve FrankenPHP'yi çalıştırırsanız, Xdebug yüklenecektir.
