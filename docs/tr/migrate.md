# Nginx/PHP-FPM'den geçiş

FrankenPHP hem web sunucunuzu (Nginx, Apache) hem PHP-FPM'i tek bir ikili dosyayla değiştirir.
Bu kılavuz tipik bir PHP uygulaması için temel geçişi kapsar.

## Temel farklar

| PHP-FPM kurulumu                  | FrankenPHP karşılığı                                     |
| --------------------------------- | -------------------------------------------------------- |
| Nginx/Apache + PHP-FPM            | Tek `frankenphp` ikili dosyası                           |
| `php-fpm.conf` havuz ayarları     | [`frankenphp` genel seçeneği](config.md#caddyfile-config) |
| Nginx `server {}` bloğu           | `Caddyfile` site bloğu                                   |
| `php_value` / `php_admin_value`   | [`php_ini` Caddyfile yönergesi](config.md#php-config)    |
| `pm = static` / `pm.max_children` | `num_threads`                                            |
| `pm = dynamic`                    | [`max_threads auto`](performance.md#max_threads)         |
| Web sunucusu istek süzme          | Caddy route'ları ve eşleştiriciler                       |

## HTTP istek süzme

Stok bir Nginx veya Apache paketinden geçiş yaparken, PHP-FPM isteği almadan önce web sunucusunun uyguladığı istek süzmeyi kontrol edin.
FrankenPHP Caddy üzerine kuruludur; geçerli HTTP metodları ve başlıkları, `Caddyfile`'ınız veya başka bir vekil onları önce reddetmedikçe Caddy'nin olağan yönlendirmesinden geçer.

Örneğin bazı web sunucuları `TRACE` isteklerini varsayılan olarak reddeder.
Uygulamanız bu davranışı korumalıysa `php_server` öncesine açık bir eşleştirici ekleyin:

```caddyfile
example.com {
    @trace method TRACE
    respond @trace 405

    root /var/www/app/public
    php_server
}
```

Önceki web sunucunuzun uyguladığı daha katı metod veya başlık politikalarına aynı yaklaşımı uygulayın.
Aynı durum istek başlık adları için de geçerlidir: bazı web sunucuları veya vekiller geçerli ama alışılmadık HTTP alan adlarını PHP-FPM görmeden reddeder veya düşürür; FrankenPHP ise bunları PHP uygulamasına geçirebilir.

## 1. adım: web sunucusu yapılandırmasını değiştirin

Tipik bir Nginx + PHP-FPM yapılandırması:

```nginx
# /etc/nginx/sites-available/example.com
server {
    listen 80;
    server_name example.com;
    root /var/www/app/public;
    index index.php;

    location / {
        try_files $uri $uri/ /index.php$is_args$args;
    }

    location ~ \.php$ {
        fastcgi_pass unix:/run/php/php-fpm.sock;
        fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
        include fastcgi_params;
    }
}
```

Tek bir `Caddyfile` olur:

```caddyfile
example.com {
    root /var/www/app/public
    php_server
}
```

Bu kadar. `php_server` yönergesi PHP yönlendirmesini, `try_files` benzeri davranışı ve statik dosya sunumunu halleder.

## 2. adım: PHP yapılandırmasını taşıyın

Mevcut `php.ini` dosyanız olduğu gibi çalışır. Kurulum yöntemine göre nereye koyacağınız için [Konfigürasyon](config.md) belgesine bakın.

Yönergeleri doğrudan `Caddyfile` içinde de ayarlayabilirsiniz:

```caddyfile
{
    frankenphp {
        php_ini memory_limit 256M
        php_ini max_execution_time 30
    }
}

example.com {
    root /var/www/app/public
    php_server
}
```

## 3. adım: havuz boyutunu ayarlayın

PHP-FPM'de worker süreç sayısını denetlemek için `pm.max_children` ayarlanır.
FrankenPHP'de karşılığı `num_threads`'tir:

```caddyfile
{
    frankenphp {
        num_threads 16
    }
}
```

Varsayılan olarak FrankenPHP CPU başına 2 iş parçacığı başlatır. PHP-FPM'nin `pm = dynamic` ayarına benzer dinamik ölçekleme için:

```caddyfile
{
    frankenphp {
        num_threads 4
        max_threads auto
    }
}
```

## 4. adım: Docker geçişi

Tipik bir PHP-FPM Docker kurulumu (Nginx + PHP-FPM, iki konteyner) tek bir konteynerle değiştirilebilir:

**Önce:**

```yaml
# compose.yaml
services:
  nginx:
    image: nginx:1
    volumes:
      # Nginx yapılandırmasını konteynere bağlayın
      - ./config:/etc/nginx/conf.d
      - .:/var/www/app
    ports:
      - "80:80"
      - "443:443"

  php:
    image: php:8.5-fpm
    volumes:
      - .:/var/www/app
```

**Sonra:**

```yaml
# compose.yaml
services:
  php:
    image: dunglas/frankenphp:1-php8.5
    volumes:
      - .:/var/www/app
      # Caddyfile'ı konteynere bağlayın
      - ./config:/etc/frankenphp
      - caddy_data:/data
      - caddy_config:/config
    ports:
      - "80:80"
      - "443:443"
      - "443:443/udp"

volumes:
  caddy_data:
  caddy_config:
```

Ek PHP eklentilerine ihtiyacınız varsa [özel Docker imajı oluşturma](docker.md#how-to-install-more-php-extensions) belgesine bakın.

Çerçeveye özel Docker kurulumları için [Symfony Docker](https://github.com/dunglas/symfony-docker) ve [Laravel'i FrankenPHP Docker imajıyla çalıştırma](laravel.md#running-laravel-with-the-frankenphp-docker-image) belgelerine bakın.

## 5. adım: worker modunu değerlendirin (isteğe bağlı)

[Klasik modda](classic.md) FrankenPHP PHP-FPM gibi çalışır: her istek uygulamayı sıfırdan başlatır. Geçiş için güvenli bir başlangıç noktasıdır.

Daha iyi performans için [worker moduna](worker.md) geçebilirsiniz; bu mod uygulamayı bir kez başlatır ve bellekte tutar:

```caddyfile
example.com {
    root /var/www/app/public
    php_server {
        root /var/www/app/public
        worker index.php 4
    }
}
```

> [!CAUTION]
>
> Worker modu uygulamanızı istekler arasında bellekte tutar. Kodunuzun istekler arasında global durumun sıfırlanmasına dayanmadığından emin olun. [Symfony](worker.md#worker-mode-for-symfony), [Laravel](laravel.md#laravel-octane) ve [API Platform](https://api-platform.com) gibi çerçeveler bu modu yerel olarak destekler.

## Neleri kaldırabilirsiniz

Geçişten sonra şunlara artık ihtiyacınız yoktur:

- Nginx veya Apache
- PHP-FPM (`php-fpm` servisi/süreci)
- FastCGI yapılandırması
- Kendi yönettiğiniz TLS sertifikaları (Caddy bunları otomatik halleder)
