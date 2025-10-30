# Binary Dosyası Olarak PHP Uygulamaları

FrankenPHP, PHP uygulamalarının kaynak kodunu ve varlıklarını statik, kendi kendine yeten bir binary dosyaya yerleştirme yeteneğine sahiptir.

Bu özellik sayesinde PHP uygulamaları, uygulamanın kendisini, PHP yorumlayıcısını ve üretim düzeyinde bir web sunucusu olan Caddy'yi içeren bağımsız bir binary dosyalar olarak çıktısı alınabilir ve dağıtılabilir.

Bu özellik hakkında daha fazla bilgi almak için [Kévin tarafından SymfonyCon 2023'te yapılan sunuma](https://dunglas.dev/2023/12/php-and-symfony-apps-as-standalone-binaries/) göz atabilirsiniz.

## Preparing Your App

Bağımsız binary dosyayı oluşturmadan önce uygulamanızın gömülmeye hazır olduğundan emin olun.

Örneğin muhtemelen şunları yapmak istersiniz:

- Uygulamanın üretim bağımlılıklarını yükleyin
- Otomatik yükleyiciyi boşaltın
- Uygulamanızın üretim modunu etkinleştirin (varsa)
- Nihai binary dosyanızın boyutunu küçültmek için `.git` veya testler gibi gerekli olmayan dosyaları çıkarın

Örneğin, bir Symfony uygulaması için aşağıdaki komutları kullanabilirsiniz:

```console
# .git/, vb. dosyalarından kurtulmak için projeyi dışa aktarın
mkdir $TMPDIR/my-prepared-app
git archive HEAD | tar -x -C $TMPDIR/my-prepared-app
cd $TMPDIR/my-prepared-app

# Uygun ortam değişkenlerini ayarlayın
echo APP_ENV=prod > .env.local
echo APP_DEBUG=0 >> .env.local

# Testleri kaldırın
rm -Rf tests/

# Bağımlılıkları yükleyin
composer install --ignore-platform-reqs --no-dev -a

# .env'yi optimize edin
composer dump-env prod
```

## Linux Binary'si Oluşturma

Bir Linux binary çıktısı almanın en kolay yolu, sağladığımız Docker tabanlı derleyiciyi kullanmaktır.

1. Hazırladığınız uygulamanın deposunda `static-build.Dockerfile` adlı bir dosya oluşturun:

   ```dockerfile
   FROM --platform=linux/amd64 dunglas/frankenphp:static-builder-gnu
   # İkili dosyayı musl-libc sistemlerinde çalıştırmayı düşünüyorsanız static-builder-musl kullanın

   # Uygulamanızı kopyalayın
   WORKDIR /go/src/app/dist/app
   COPY . .

   # Statik binary dosyasını oluşturun, yalnızca istediğiniz PHP eklentilerini seçtiğinizden emin olun
   WORKDIR /go/src/app/
   RUN EMBED=dist/app/ \
       ./build-static.sh
   ```

   > [!CAUTION]
   >
   > Bazı `.dockerignore` dosyaları (örneğin varsayılan [Symfony Docker `.dockerignore`](https://github.com/dunglas/symfony-docker/blob/main/.dockerignore))
   > `vendor/` dizinini ve `.env` dosyalarını yok sayacaktır. Derlemeden önce `.dockerignore` dosyasını ayarladığınızdan veya kaldırdığınızdan emin olun.

2. Derleyin:

   ```console
   docker build -t static-app -f static-build.Dockerfile .
   ```

3. Binary dosyasını çıkarın:

   ```console
   docker cp $(docker create --name static-app-tmp static-app):/go/src/app/dist/frankenphp-linux-x86_64 my-app ; docker rm static-app-tmp
   ```

Elde edilen binary dosyası, geçerli dizindeki `my-app` adlı dosyadır.

## Diğer İşletim Sistemleri için Binary Çıktısı Alma

Docker kullanmak istemiyorsanız veya bir macOS binary dosyası oluşturmak istiyorsanız, sağladığımız kabuk betiğini kullanın:

```console
git clone https://github.com/php/frankenphp
cd frankenphp
EMBED=/path/to/your/app \
    ./build-static.sh
```

Elde edilen binary dosyası `dist/` dizinindeki `frankenphp-<os>-<arch>` adlı dosyadır.

## Binary Dosyasını Kullanma

İşte bu kadar! `my-app` dosyası (veya diğer işletim sistemlerinde `dist/frankenphp-<os>-<arch>`) bağımsız uygulamanızı içerir!

Web uygulamasını başlatmak için çalıştırın:

```console
./my-app php-server
```

Uygulamanız bir [worker betiği](worker.md) içeriyorsa, worker'ı aşağıdaki gibi bir şeyle başlatın:

```console
./my-app php-server --worker public/index.php
```

HTTPS (Let's Encrypt sertifikası otomatik olarak oluşturulur), HTTP/2 ve HTTP/3'ü etkinleştirmek için kullanılacak alan adını belirtin:

```console
./my-app php-server --domain localhost
```

Ayrıca binary dosyanıza gömülü PHP CLI betiklerini de çalıştırabilirsiniz:

```console
./my-app php-cli bin/console
```

## Yapıyı Özelleştirme

Binary dosyasının nasıl özelleştirileceğini (uzantılar, PHP sürümü...) görmek için [Statik derleme dokümanını okuyun](static.md).

## Binary Dosyasının Dağıtılması

Linux'ta, oluşturulan ikili dosya [UPX](https://upx.github.io) kullanılarak sıkıştırılır.

Mac'te, göndermeden önce dosyanın boyutunu küçültmek için sıkıştırabilirsiniz.
Biz `xz` öneririz.
