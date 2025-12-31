# Statik Yapı Oluşturun

PHP kütüphanesinin yerel kurulumunu kullanmak yerine,
harika [static-php-cli projesi](https://github.com/crazywhalecc/static-php-cli) sayesinde FrankenPHP'nin statik bir yapısını oluşturmak mümkündür (adına rağmen, bu proje sadece CLI'yi değil, tüm SAPI'leri destekler).

Bu yöntemle, tek, taşınabilir bir ikili PHP yorumlayıcısını, Caddy web sunucusunu ve FrankenPHP'yi içerecektir!

FrankenPHP ayrıca [PHP uygulamasının statik binary gömülmesini](embed.md) destekler.

## Linux

Linux statik binary dosyası oluşturmak için bir Docker imajı sağlıyoruz:

```console
docker buildx bake --load static-builder
docker cp $(docker create --name static-builder-musl dunglas/frankenphp:static-builder-musl):/go/src/app/dist/frankenphp-linux-$(uname -m) frankenphp ; docker rm static-builder
```

Elde edilen statik binary `frankenphp` olarak adlandırılır ve geçerli dizinde kullanılabilir.

Statik binary dosyasını Docker olmadan oluşturmak istiyorsanız, Linux için de çalışan macOS talimatlarına bir göz atın.

### Özel Eklentiler

Varsayılan olarak, en popüler PHP eklentileri zaten derlenir.

Binary dosyanın boyutunu küçültmek ve saldırı yüzeyini azaltmak için `PHP_EXTENSIONS` Docker ARG'sini kullanarak derlenecek eklentilerin listesini seçebilirsiniz.

Örneğin, yalnızca `opcache` eklentisini derlemek için aşağıdaki komutu çalıştırın:

```console
docker buildx bake --load --set static-builder.args.PHP_EXTENSIONS=opcache,pdo_sqlite static-builder
# ...
```

Etkinleştirdiğiniz eklentilere ek işlevler sağlayan kütüphaneler eklemek için `PHP_EXTENSION_LIBS` Docker ARG'sini kullanabilirsiniz:

```console
docker buildx bake \
  --load \
  --set static-builder.args.PHP_EXTENSIONS=gd \
  --set static-builder.args.PHP_EXTENSION_LIBS=libjpeg,libwebp \
  static-builder
```

### Ekstra Caddy Modülleri

Ekstra Caddy modülleri eklemek veya [xcaddy](https://github.com/caddyserver/xcaddy) adresine başka argümanlar iletmek için `XCADDY_ARGS` Docker ARG'sini kullanın:

```console
docker buildx bake \
  --load \
  --set static-builder.args.XCADDY_ARGS="--with github.com/darkweak/souin/plugins/caddy --with github.com/dunglas/caddy-cbrotli --with github.com/dunglas/mercure/caddy --with github.com/dunglas/vulcain/caddy" \
  static-builder
```

Bu örnekte, Caddy için [Souin](https://souin.io) HTTP önbellek modülünün yanı sıra [cbrotli](https://github.com/dunglas/caddy-cbrotli), [Mercure](https://mercure.rocks) ve [Vulcain](https://vulcain.rocks) modüllerini ekliyoruz.

> [!TIP]
>
> cbrotli, Mercure ve Vulcain modülleri, `XCADDY_ARGS` boşsa veya ayarlanmamışsa varsayılan olarak dahil edilir.
> Eğer `XCADDY_ARGS` değerini özelleştirirseniz, dahil edilmelerini istiyorsanız bunları açıkça dahil etmelisiniz.

Derlemeyi nasıl [özelleştireceğinize](#yapıyı-özelleştirme) de bakın.

### GitHub Token

GitHub API kullanım limitine ulaşırsanız, `GITHUB_TOKEN` adlı bir ortam değişkeninde bir GitHub Personal Access Token ayarlayın:

```console
GITHUB_TOKEN="xxx" docker --load buildx bake static-builder
# ...
```

## macOS

macOS için statik bir binary oluşturmak için aşağıdaki betiği çalıştırın ([Homebrew](https://brew.sh/) yüklü olmalıdır):

```console
git clone https://github.com/php/frankenphp
cd frankenphp
./build-static.sh
```

Not: Bu betik Linux'ta (ve muhtemelen diğer Unix'lerde) da çalışır ve sağladığımız Docker tabanlı statik derleyici tarafından dahili olarak kullanılır.

## Yapıyı Özelleştirme

Aşağıdaki ortam değişkenleri `docker build` ve `build-static.sh` dosyalarına aktarılabilir
statik derlemeyi özelleştirmek için betik:

- `FRANKENPHP_VERSION`: kullanılacak FrankenPHP sürümü
- `PHP_VERSION`: kullanılacak PHP sürümü
- `PHP_EXTENSIONS`: oluşturulacak PHP eklentileri ([desteklenen eklentiler listesi](https://static-php.dev/en/guide/extensions.html))
- `PHP_EXTENSION_LIBS`: eklentilere özellikler ekleyen oluşturulacak ekstra kütüphaneler
- `XCADDY_ARGS`: [xcaddy](https://github.com/caddyserver/xcaddy) adresine iletilecek argümanlar, örneğin ekstra Caddy modülleri eklemek için
- `EMBED`: binary dosyaya gömülecek PHP uygulamasının yolu
- `CLEAN`: ayarlandığında, libphp ve tüm bağımlılıkları sıfırdan oluşturulur (önbellek yok)
- `DEBUG_SYMBOLS`: ayarlandığında, hata ayıklama sembolleri ayıklanmayacak ve binary dosyaya eklenecektir
- `RELEASE`: (yalnızca bakımcılar) ayarlandığında, ortaya çıkan binary dosya GitHub'a yüklenecektir
