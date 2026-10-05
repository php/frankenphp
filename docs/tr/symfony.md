# Symfony

## Symfony'yi Symfony Docker ile çalıştırma

[Symfony](https://symfony.com) projeleri için FrankenPHP'nin yazarı tarafından bakımı yapılan resmi Symfony Docker kurulumu olan [Symfony Docker](https://github.com/dunglas/symfony-docker)'ı öneririz. FrankenPHP, otomatik HTTPS, HTTP/2, HTTP/3 ve worker modu desteğiyle kutudan çıkar çıkmaz eksiksiz bir Docker tabanlı ortam sağlar.

## Symfony'yi FrankenPHP ile yerel kurma

Alternatif olarak Symfony projelerinizi FrankenPHP ile yerel makinenizden çalıştırabilirsiniz:

1. [FrankenPHP'yi kurun](../#getting-started)
2. Aşağıdaki yapılandırmayı Symfony projenizin kök dizinindeki `Caddyfile` adlı bir dosyaya ekleyin:

   ```caddyfile
   # Caddyfile
   # Sunucunuzun alan adı
   localhost

   root public/
   php_server {
   	# İsteğe bağlı: daha iyi performans için worker modunu etkinleştirin
   	worker ./public/index.php
   }
   ```

   Daha fazla iyileştirme için [performans belgelerine](performance.md) bakın.

3. FrankenPHP'yi Symfony projenizin kök dizininden başlatın: `frankenphp run`

## FrankenPHP ile Symfony worker modu

Symfony 7.4'ten itibaren FrankenPHP worker modu yerel olarak desteklenir.

Daha eski sürümler için [PHP Runtime](https://github.com/php-runtime/runtime) FrankenPHP paketini kurun:

```console
composer require runtime/frankenphp-symfony
```

FrankenPHP Symfony Runtime'ını kullanmak için `APP_RUNTIME` ortam değişkenini tanımlayarak uygulama sunucunuzu başlatın:

```console
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php" \
    -e APP_RUNTIME=Runtime\\FrankenPhpSymfony\\Runtime \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

[Worker modu](worker.md) hakkında daha fazla bilgi edinin.

### Worker uyumluluğunu denetleme

[Igor PHP](https://github.com/igor-php/igor-php), Symfony projelerini üretimde sorun çıkmadan önce durum sızıntılarına karşı tarayan statik bir linter'dır: `ResetInterface` eksik servisler, sıfırlanmayan durumlu özellikler, değiştirilebilir yerel static'ler, `exit()`/`die()` çağrıları ve süper küresel yazmaları. Hem uygulama kodunuzu hem `vendor/` içinde bildirilen servisleri denetler.

```console
composer require --dev igor-php/igor-php
vendor/bin/igor-php .
```

## Symfony için sıcak yeniden yükleme

Sıcak yeniden yükleme [Symfony Docker](https://github.com/dunglas/symfony-docker) içinde varsayılan olarak etkindir.

[Sıcak yeniden yükleme](hot-reload.md) özelliğini Symfony Docker olmadan kullanmak için [Mercure](mercure.md)'ü etkinleştirin ve `Caddyfile`'ınızdaki `php_server` yönergesine `hot_reload` alt yönergesini ekleyin:

```caddyfile
localhost

mercure {
	anonymous
}

root public/
php_server {
	hot_reload
	worker ./public/index.php
}
```

Ardından aşağıdaki kodu `templates/base.html.twig` dosyanıza ekleyin:

```twig
{# templates/base.html.twig #}
{% if app.request.server.has('FRANKENPHP_HOT_RELOAD') %}
    <meta name="frankenphp-hot-reload:url" content="{{ app.request.server.get('FRANKENPHP_HOT_RELOAD') }}">
    <script src="https://cdn.jsdelivr.net/npm/idiomorph/dist/idiomorph.min.js"></script>
    <script src="https://cdn.jsdelivr.net/npm/frankenphp-hot-reload/+esm" type="module"></script>
{% endif %}
```

Son olarak Symfony projenizin kök dizininden `frankenphp run` çalıştırın.

## Varlıkları önceden sıkıştırma

Symfony'nin [AssetMapper bileşeni](https://symfony.com/doc/current/frontend/asset_mapper.html) dağıtım sırasında varlıkları Brotli ve Zstandard ile önceden sıkıştırabilir. FrankenPHP (Caddy'nin `file_server`'ı aracılığıyla) bu önceden sıkıştırılmış dosyaları doğrudan sunabilir ve anlık sıkıştırma yükünü ortadan kaldırır.

1. Varlıklarınızı derleyin ve sıkıştırın:

   ```console
   php bin/console asset-map:compile
   ```

2. Önceden sıkıştırılmış varlıkları sunmak için `Caddyfile`'ınızı güncelleyin:

   ```caddyfile
   # Caddyfile
   localhost

   @assets path /assets/*
   file_server @assets {
   	precompressed zstd br gzip
   }

   root public/
   php_server {
   	worker ./public/index.php
   }
   ```

`precompressed` yönergesi Caddy'ye istenen dosyanın önceden sıkıştırılmış sürümlerini (ör. `app.css.zst`, `app.css.br`) aramasını ve istemci destekliyorsa bunları doğrudan sunmasını söyler.

## Büyük statik dosyaları sunma (`X-Sendfile`)

FrankenPHP, PHP kodunu çalıştırdıktan sonra [büyük statik dosyaları verimli sunmayı](x-sendfile.md) destekler (erişim denetimi, istatistikler vb. için).

Symfony HttpFoundation [bu özelliği yerel olarak destekler](https://symfony.com/doc/current/components/http_foundation.html#serving-files).
[`Caddyfile`'ınızı yapılandırdıktan](x-sendfile.md#configuring-x-accel-redirect-in-the-frankenphp-caddyfile) sonra `X-Accel-Redirect` başlığı için doğru değeri otomatik belirler ve yanıta ekler:

```php
use Symfony\Component\HttpFoundation\BinaryFileResponse;

BinaryFileResponse::trustXSendfileTypeHeader();
$response = new BinaryFileResponse(__DIR__.'/../private-files/file.txt');

// ...
```

## Symfony uygulamalarını bağımsız ikili dosyalar olarak dağıtma

[FrankenPHP'nin uygulama gömme özelliğini](embed.md) kullanarak Symfony uygulamalarını
bağımsız ikili dosyalar olarak dağıtmak mümkündür.

Symfony uygulamanızı hazırlamak ve paketlemek için şu adımları izleyin:

1. Uygulamanızı hazırlayın:

   ```console
   # .git/ vb. dosyalarından kurtulmak için projeyi dışa aktarın
   mkdir $TMPDIR/my-prepared-app
   git archive HEAD | tar -x -C $TMPDIR/my-prepared-app
   cd $TMPDIR/my-prepared-app

   # Uygun ortam değişkenlerini ayarlayın
   echo APP_ENV=prod > .env.local
   echo APP_DEBUG=0 >> .env.local

   # Yer kazanmak için testleri ve diğer gereksiz dosyaları kaldırın
   # Alternatif olarak bu dosyaları .gitattributes dosyanızda export-ignore özniteliğiyle ekleyin
   rm -Rf tests/

   # Bağımlılıkları yükleyin
   composer install --ignore-platform-reqs --no-dev -a

   # .env'yi optimize edin
   composer dump-env prod
   ```

2. Uygulamanızın deposunda `static-build.Dockerfile` adlı bir dosya oluşturun:

   ```dockerfile
   # static-build.Dockerfile
   FROM --platform=linux/amd64 dunglas/frankenphp:static-builder-gnu
   # İkili dosyayı musl-libc sistemlerinde çalıştırmayı düşünüyorsanız bunun yerine static-builder-musl kullanın

   # Uygulamanızı kopyalayın
   WORKDIR /go/src/app/dist/app
   COPY . .

   # Statik ikili dosyayı derleyin
   WORKDIR /go/src/app/
   RUN EMBED=dist/app/ ./build-static.sh
   ```

   > [!CAUTION]
   >
   > Bazı `.dockerignore` dosyaları (ör. varsayılan [Symfony Docker `.dockerignore`](https://github.com/dunglas/symfony-docker/blob/main/.dockerignore))
   > `vendor/` dizinini ve `.env` dosyalarını yok sayar. Derlemeden önce `.dockerignore` dosyasını ayarladığınızdan veya kaldırdığınızdan emin olun.

3. Derleyin:

   ```console
   docker build -t static-symfony-app -f static-build.Dockerfile .
   ```

4. İkili dosyayı çıkarın:

   ```console
   docker cp $(docker create --name static-symfony-app-tmp static-symfony-app):/go/src/app/dist/frankenphp-linux-x86_64 my-app ; docker rm static-symfony-app-tmp
   ```

5. Sunucuyu başlatın:

   ```console
   ./my-app php-server
   ```

Kullanılabilir seçenekler ve diğer işletim sistemleri için ikili dosya derleme hakkında daha fazla bilgiyi [uygulama gömme](embed.md)
belgelerinde bulabilirsiniz.
