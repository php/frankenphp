# Bağımsız Çalıştırılabilir PHP Uygulamaları

FrankenPHP, PHP uygulamalarının kaynak kodunu ve varlıklarını statik, bağımsız çalıştırılabilir bir dosyaya yerleştirme yeteneğine sahiptir.

Bu özellik sayesinde PHP uygulamaları, uygulamanın kendisini, PHP yorumlayıcısını ve üretim düzeyinde bir web sunucusu olan Caddy'yi içeren bağımsız çalıştırılabilir dosyalar olarak dağıtılabilir.

Bu özellik hakkında daha fazla bilgi almak için [Kévin tarafından SymfonyCon 2023'te yapılan sunuma](https://dunglas.dev/2023/12/php-and-symfony-apps-as-standalone-binaries/) göz atabilirsiniz.

Laravel uygulamalarını gömmek için [bu özel dokümantasyon girdisini okuyun](laravel.md#laravel-apps-as-standalone-binaries).

## Uygulamanızı Hazırlama

Bağımsız çalıştırılabilir dosyayı oluşturmadan önce uygulamanızın gömülmeye hazır olduğundan emin olun.

Örneğin muhtemelen şunları yapmak istersiniz:

- Uygulamanın üretim bağımlılıklarını yükleyin
- Otomatik yükleyiciyi oluşturun
- Uygulamanızın üretim modunu etkinleştirin (varsa)
- Nihai çalıştırılabilir dosyanızın boyutunu azaltmak için `.git` veya testler gibi gereksiz dosyaları ayıklayın

Örneğin, bir Symfony uygulaması için aşağıdaki komutları kullanabilirsiniz:

```console
# .git/, vb. dosyalarından kurtulmak için projeyi dışa aktarın
mkdir $TMPDIR/my-prepared-app
git archive HEAD | tar -x -C $TMPDIR/my-prepared-app
cd $TMPDIR/my-prepared-app

# Uygun ortam değişkenlerini ayarlayın
echo APP_ENV=prod > .env.local
echo APP_DEBUG=0 >> .env.local

# Testleri ve diğer gereksiz dosyaları yer kazanmak için kaldırın
# Alternatif olarak, bu dosyaları .gitattributes dosyanızdaki export-ignore niteliğiyle ekleyin
rm -Rf tests/

# Bağımlılıkları yükleyin
composer install --ignore-platform-reqs --no-dev -a

# .env'yi optimize edin
composer dump-env prod
```

### Yapılandırmayı Özelleştirme

[Yapılandırmayı](config.md) özelleştirmek için, gömülecek uygulamanın ana dizinine (önceki örnekte `$TMPDIR/my-prepared-app`) bir `Caddyfile` ve bir `php.ini` dosyası yerleştirebilirsiniz.

## Linux Çalıştırılabilir Dosyası Oluşturma

Bir Linux çalıştırılabilir dosyası oluşturmanın en kolay yolu, sağladığımız Docker tabanlı derleyiciyi kullanmaktır.

1. Hazırladığınız uygulamanın deposunda `static-build.Dockerfile` adlı bir dosya oluşturun:

   ```dockerfile
   FROM --platform=linux/amd64 dunglas/frankenphp:static-builder-gnu
   # İkili dosyayı musl-libc sistemlerinde çalıştırmayı düşünüyorsanız static-builder-musl kullanın

   # Uygulamanızı kopyalayın
   WORKDIR /go/src/app/dist/app
   COPY . .

   # Statik çalıştırılabilir dosyayı oluşturun
   WORKDIR /go/src/app/
   RUN EMBED=dist/app/ ./build-static.sh
   ```

   > [!CAUTION]
   >
   > Bazı `.dockerignore` dosyaları (örneğin varsayılan [Symfony Docker `.dockerignore`](https://github.com/dunglas/symfony-docker/blob/main/.dockerignore))
   > `vendor/` dizinini ve `.env` dosyalarını yok sayacaktır. Derlemeden önce `.dockerignore` dosyasını ayarladığınızdan veya kaldırdığınızdan emin olun.

2. Derleyin:

   ```console
   docker build -t static-app -f static-build.Dockerfile .
   ```

3. Çalıştırılabilir dosyayı çıkarın:

   ```console
   docker cp $(docker create --name static-app-tmp static-app):/go/src/app/dist/frankenphp-linux-x86_64 my-app ; docker rm static-app-tmp
   ```

Elde edilen çalıştırılabilir dosya, geçerli dizindeki `my-app` adlı dosyadır.

## Diğer İşletim Sistemleri için Çalıştırılabilir Dosya Oluşturma

Docker kullanmak istemiyorsanız veya bir macOS çalıştırılabilir dosyası oluşturmak istiyorsanız, sağladığımız kabuk betiğini kullanın:

```console
git clone https://github.com/php/frankenphp
cd frankenphp
EMBED=/path/to/your/app ./build-static.sh
```

Elde edilen çalıştırılabilir dosya `dist/` dizinindeki `frankenphp-<os>-<arch>` adlı dosyadır.

## Çalıştırılabilir Dosyayı Kullanma

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

Ayrıca çalıştırılabilir dosyanıza gömülü PHP CLI betiklerini de çalıştırabilirsiniz:

```console
./my-app php-cli bin/console
```

## PHP Uzantıları

Varsayılan olarak, betik projenizin `composer.json` dosyası tarafından (varsa) gerekli olan uzantıları derleyecektir. Eğer `composer.json` dosyası yoksa, [statik derlemeler girdisinde](static.md) belgelendiği gibi varsayılan uzantılar derlenir.

Uzantıları özelleştirmek için `PHP_EXTENSIONS` ortam değişkenini kullanın.

## Yapıyı Özelleştirme

Çalıştırılabilir dosyanın nasıl özelleştirileceğini (uzantılar, PHP sürümü...) görmek için [statik derleme dokümantasyonunu okuyun](static.md).

## Çalıştırılabilir Dosyanın Dağıtılması

Linux'ta, oluşturulan ikili dosya [UPX](https://upx.github.io) kullanılarak sıkıştırılır.

Mac'te, göndermeden önce dosyanın boyutunu küçültmek için sıkıştırabilirsiniz.
Biz `xz` öneririz.
