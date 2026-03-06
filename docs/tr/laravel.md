# Laravel

## Docker

Bir [Laravel](https://laravel.com) web uygulamasını FrankenPHP ile çalıştırmak, projeyi resmi Docker imajının `/app` dizinine monte etmek kadar kolaydır.

Bu komutu Laravel uygulamanızın ana dizininden çalıştırın:

```console
docker run -p 80:80 -p 443:443 -p 443:443/udp -v $PWD:/app dunglas/frankenphp
```

Ve tadını çıkarın!

## Yerel Kurulum

Alternatif olarak, Laravel projelerinizi FrankenPHP ile yerel makinenizden çalıştırabilirsiniz:

1. [Sisteminize karşılık gelen ikili dosyayı indirin](../#standalone-binary)
2. Aşağıdaki yapılandırmayı Laravel projenizin kök dizinindeki `Caddyfile` adlı bir dosyaya ekleyin:

   ```caddyfile
   {
   	frankenphp
   }

   # Sunucunuzun alan adı
   localhost {
   	# Webroot'u public/ dizinine ayarlayın
   	root public/
   	# Sıkıştırmayı etkinleştir (isteğe bağlı)
   	encode zstd br gzip
   	# public/ dizininden PHP dosyalarını çalıştırın ve statik dosyaları servis edin
   	php_server {
   		try_files {path} index.php
   	}
   }
   ```

3. FrankenPHP'yi Laravel projenizin kök dizininden başlatın: `frankenphp run`

## Laravel Octane

Octane, Composer paket yöneticisi aracılığıyla kurulabilir:

```console
composer require laravel/octane
```

Octane'ı kurduktan sonra, Octane'ın yapılandırma dosyasını uygulamanıza yükleyecek olan `octane:install` Artisan komutunu çalıştırabilirsiniz:

```console
php artisan octane:install --server=frankenphp
```

Octane sunucusu `octane:frankenphp` Artisan komutu aracılığıyla başlatılabilir.

```console
php artisan octane:frankenphp
```

`octane:frankenphp` komutu aşağıdaki seçenekleri alabilir:

- `--host`: Sunucunun bağlanması gereken IP adresi (varsayılan: `127.0.0.1`)
- `--port`: Sunucunun erişilebilir olması gereken port (varsayılan: `8000`)
- `--admin-port`: Yönetici sunucusunun erişilebilir olması gereken port (varsayılan: `2019`)
- `--workers`: İstekleri işlemek için hazır olması gereken worker sayısı (varsayılan: `auto`)
- `--max-requests`: Sunucu yeniden yüklenmeden önce işlenecek istek sayısı (varsayılan: `500`)
- `--caddyfile`: FrankenPHP `Caddyfile` dosyasının yolu (varsayılan: [Laravel Octane içinde bulunan şablon `Caddyfile`](https://github.com/laravel/octane/blob/2.x/src/Commands/stubs/Caddyfile))
- `--https`: HTTPS, HTTP/2 ve HTTP/3'ü etkinleştirin ve sertifikaları otomatik olarak oluşturup yenileyin
- `--http-redirect`: HTTP'den HTTPS'ye yeniden yönlendirmeyi etkinleştir (yalnızca --https ile birlikte geçilirse etkinleşir)
- `--watch`: Uygulama değiştirildiğinde sunucuyu otomatik olarak yeniden yükle
- `--poll`: Dosyaları bir ağ üzerinden izlemek için izleme sırasında dosya sistemi yoklamasını kullanın
- `--log-level`: Yerel Caddy günlüğünü kullanarak belirtilen günlük seviyesinde veya üzerinde mesajları kaydedin

> [!TIP]
> Yapılandırılmış JSON günlükleri elde etmek için (log analitik çözümleri kullanırken faydalıdır), `--log-level` seçeneğini açıkça geçin.

[Laravel Octane hakkında daha fazla bilgiyi resmi belgelerde bulabilirsiniz](https://laravel.com/docs/octane).

## Laravel Uygulamalarını Bağımsız Çalıştırılabilir Dosyalar Olarak Dağıtma

[FrankenPHP'nin uygulama gömme özelliğini](embed.md) kullanarak, Laravel
uygulamalarını bağımsız çalıştırılabilir dosyalar olarak dağıtmak mümkündür.

Linux için Laravel uygulamanızı bağımsız bir çalıştırılabilir olarak paketlemek için şu adımları izleyin:

1. Uygulamanızın deposunda `static-build.Dockerfile` adında bir dosya oluşturun:

   ```dockerfile
   FROM --platform=linux/amd64 dunglas/frankenphp:static-builder-gnu
   # İkiliyi musl-libc sistemlerinde çalıştırmayı düşünüyorsanız, bunun yerine static-builder-musl kullanın

   # Uygulamanızı kopyalayın
   WORKDIR /go/src/app/dist/app
   COPY . .

   # Yer kaplamamak için testleri ve diğer gereksiz dosyaları kaldırın
   # Alternatif olarak, bu dosyaları bir .dockerignore dosyasına ekleyin
   RUN rm -Rf tests/

   # .env dosyasını kopyalayın
   RUN cp .env.example .env
   # APP_ENV ve APP_DEBUG değerlerini production için uygun hale getirin
   RUN sed -i'' -e 's/^APP_ENV=.*/APP_ENV=production/' -e 's/^APP_DEBUG=.*/APP_DEBUG=false/' .env

   # Gerekirse .env dosyanıza diğer değişiklikleri yapın

   # Bağımlılıkları yükleyin
   RUN composer install --ignore-platform-reqs --no-dev -a

   # Statik ikiliyi derleyin
   WORKDIR /go/src/app/
   RUN EMBED=dist/app/ ./build-static.sh
   ```

   > [!CAUTION]
   > Bazı `.dockerignore` dosyaları
   > `vendor/` dizinini ve `.env` dosyalarını yok sayar. Derlemeden önce `.dockerignore` dosyasını buna göre ayarladığınızdan veya kaldırdığınızdan emin olun.

2. İmajı oluşturun:

   ```console
   docker build -t static-laravel-app -f static-build.Dockerfile .
   ```

3. İkili dosyayı dışa aktarın:

   ```console
   docker cp $(docker create --name static-laravel-app-tmp static-laravel-app):/go/src/app/dist/frankenphp-linux-x86_64 frankenphp ; docker rm static-laravel-app-tmp
   ```

4. Önbellekleri doldurun:

   ```console
   frankenphp php-cli artisan optimize
   ```

5. Veritabanı migration'larını çalıştırın (varsa):

   ```console
   frankenphp php-cli artisan migrate
   ```

6. Uygulamanın gizli anahtarını oluşturun:

   ```console
   frankenphp php-cli artisan key:generate
   ```

7. Sunucuyu başlatın:

   ```console
   frankenphp php-server
   ```

Uygulamanız artık hazır!

Mevcut seçenekler hakkında daha fazla bilgi edinin ve diğer işletim sistemleri için nasıl ikili derleneceğini [uygulama gömme](embed.md)
belgelerinde öğrenin.

### Depolama Yolunu Değiştirme

Varsayılan olarak, Laravel yüklenen dosyaları, önbellekleri, logları vb. uygulamanın `storage/` dizininde saklar.
Gömülü uygulamalar için bu uygun değildir, çünkü her yeni sürüm farklı bir geçici dizine çıkarılacaktır.

Geçici dizin dışında bir dizin kullanmak için `LARAVEL_STORAGE_PATH` ortam değişkenini ayarlayın (örneğin, `.env` dosyanızda) veya `Illuminate\Foundation\Application::useStoragePath()` metodunu çağırın.

### Bağımsız Çalıştırılabilir Dosyalarla Octane'i Çalıştırma

Laravel Octane uygulamalarını bağımsız çalıştırılabilir dosyalar olarak paketlemek bile mümkündür!

Bunu yapmak için, [Octane'i doğru şekilde kurun](#laravel-octane) ve [önceki bölümde](#laravel-uygulamalarını-bağımsız-çalıştırılabilir-dosyalar-olarak-dağıtma) açıklanan adımları izleyin.

Ardından, Octane üzerinden FrankenPHP'yi worker modunda başlatmak için şunu çalıştırın:

```console
PATH="$PWD:$PATH" frankenphp php-cli artisan octane:frankenphp
```

> [!CAUTION]
> Komutun çalışması için, bağımsız ikili dosya mutlaka `frankenphp` olarak adlandırılmış olmalıdır,
> çünkü Octane, yol üzerinde `frankenphp` adlı bir programın mevcut olmasını bekler.
