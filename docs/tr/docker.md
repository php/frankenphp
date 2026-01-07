# Özel Docker İmajı Oluşturma

[FrankenPHP Docker imajları](https://hub.docker.com/r/dunglas/frankenphp), [resmi PHP imajları](https://hub.docker.com/_/php/) temel alınarak hazırlanmıştır.
Popüler mimariler için Debian ve Alpine Linux varyantları sağlanmıştır.
Debian varyantları tavsiye edilir.

PHP 8.2, 8.3, 8.4 ve 8.5 için varyantlar sağlanmıştır.

Etiketler şu deseni takip eder: `dunglas/frankenphp:<frankenphp-version>-php<php-version>-<os>`

- `<frankenphp-version>` ve `<php-version>` sırasıyla FrankenPHP ve PHP'nin, ana (örn. `1`), ikincil (örn. `1.2`) ila yama sürümlerine (örn. `1.2.3`) kadar değişen sürüm numaralarıdır.
- `<os>` ise `trixie` (Debian Trixie için), `bookworm` (Debian Bookworm için) veya `alpine` (Alpine'ın en son kararlı sürümü için) olabilir.

[Etiketlere göz atın](https://hub.docker.com/r/dunglas/frankenphp/tags).

## İmajlar Nasıl Kullanılır

Projenizde bir `Dockerfile` oluşturun:

```dockerfile
FROM dunglas/frankenphp

COPY . /app/public
```

Ardından, Docker imajını oluşturmak ve çalıştırmak için bu komutları çalıştırın:

```console
docker build -t my-php-app .
docker run -it --rm --name my-running-app my-php-app
```

## Yapılandırma Nasıl Ayarlanır

Kolaylık sağlaması açısından, faydalı ortam değişkenleri içeren [varsayılan bir `Caddyfile`](https://github.com/php/frankenphp/blob/main/caddy/frankenphp/Caddyfile) imajda sağlanmıştır.

## Daha Fazla PHP Eklentisi Nasıl Kurulur

[`docker-php-extension-installer`](https://github.com/mlocati/docker-php-extension-installer) betiği temel imajda sağlanmıştır.
Ek PHP eklentileri eklemek basittir:

```dockerfile
FROM dunglas/frankenphp

# buraya ek eklentileri ekleyin:
RUN install-php-extensions \
	pdo_mysql \
	gd \
	intl \
	zip \
	opcache
```

## Daha Fazla Caddy Modülü Nasıl Kurulur

FrankenPHP, Caddy'nin üzerine inşa edilmiştir ve tüm [Caddy modülleri](https://caddyserver.com/docs/modules/) FrankenPHP ile kullanılabilir.

Özel Caddy modüllerini kurmanın en kolay yolu [xcaddy](https://github.com/caddyserver/xcaddy) kullanmaktır:

```dockerfile
FROM dunglas/frankenphp:builder AS builder

# xcaddy'yi oluşturucu imajına kopyalayın
COPY --from=caddy:builder /usr/bin/xcaddy /usr/bin/xcaddy

# FrankenPHP'yi derlemek için CGO etkinleştirilmelidir
RUN CGO_ENABLED=1 \
    XCADDY_SETCAP=1 \
    XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
    CGO_CFLAGS=$(php-config --includes) \
    CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
    xcaddy build \
        --output /usr/local/bin/frankenphp \
        --with github.com/dunglas/frankenphp=./ \
        --with github.com/dunglas/frankenphp/caddy=./caddy/ \
        --with github.com/dunglas/caddy-cbrotli \
        # Mercure ve Vulcain resmi derlemeye dahil edilmiştir, ancak bunları kaldırmaktan çekinmeyin
        --with github.com/dunglas/mercure/caddy \
        --with github.com/dunglas/vulcain/caddy
        # Buraya ekstra Caddy modülleri ekleyin

FROM dunglas/frankenphp AS runner

# Resmi ikiliyi, özel modüllerinizi içeren ile değiştirin
COPY --from=builder /usr/local/bin/frankenphp /usr/local/bin/frankenphp
```

FrankenPHP tarafından sağlanan `builder` imajı `libphp`'nin derlenmiş bir sürümünü içerir.
[Builder imajları](https://hub.docker.com/r/dunglas/frankenphp/tags?name=builder) hem Debian hem de Alpine için FrankenPHP ve PHP'nin tüm sürümleri için sağlanmıştır.

> [!TIP]
>
> Eğer Alpine Linux ve Symfony kullanıyorsanız,
> [varsayılan yığın boyutunu artırmanız](compile.md#using-xcaddy) gerekebilir.

## Varsayılan Olarak Worker Modunun Etkinleştirilmesi

FrankenPHP'yi bir worker betiği ile başlatmak için `FRANKENPHP_CONFIG` ortam değişkenini ayarlayın:

```dockerfile
FROM dunglas/frankenphp

# ...

ENV FRANKENPHP_CONFIG="worker ./public/index.php"
```

## Geliştirme Sürecinde Birim (Volume) Kullanma

FrankenPHP ile kolayca geliştirme yapmak için, uygulamanın kaynak kodunu içeren dizini ana bilgisayarınızdan Docker konteynerine bir birim (volume) olarak bağlayın:

```console
docker run -v $PWD:/app/public -p 80:80 -p 443:443 -p 443:443/udp --tty my-php-app
```

> [!TIP]
>
> `--tty` seçeneği JSON günlükleri yerine insan tarafından okunabilir güzel günlüklere sahip olmayı sağlar.

Docker Compose ile:

```yaml
# compose.yaml

services:
  php:
    image: dunglas/frankenphp
    # özel bir Dockerfile kullanmak istiyorsanız aşağıdaki yorum satırını kaldırın
    #build: .
    # bunu bir production ortamında çalıştırmak istiyorsanız aşağıdaki yorum satırını kaldırın
    # restart: always
    ports:
      - "80:80" # HTTP
      - "443:443" # HTTPS
      - "443:443/udp" # HTTP/3
    volumes:
      - ./:/app/public
      - caddy_data:/data
      - caddy_config:/config
    # production ortamda aşağıdaki satırı yorum satırı yapın, geliştirme ortamında insan tarafından okunabilir güzel günlüklere sahip olmanızı sağlar
    tty: true

# Caddy sertifikaları ve yapılandırması için gereken birimler (volumes)
volumes:
  caddy_data:
  caddy_config:
```

## Root Olmayan Kullanıcı Olarak Çalıştırma

FrankenPHP, Docker'da root olmayan kullanıcı olarak çalışabilir.

İşte bunu yapan örnek bir `Dockerfile`:

```dockerfile
FROM dunglas/frankenphp

ARG USER=appuser

RUN \
	# Alpine tabanlı dağıtımlar için "adduser -D ${USER}" kullanın
	useradd ${USER}; \
	# 80 ve 443 numaralı bağlantı noktalarına bağlanmak için ek yetenek ekleyin
	setcap CAP_NET_BIND_SERVICE=+eip /usr/local/bin/frankenphp; \
	# /config/caddy ve /data/caddy dizinlerine yazma erişimi verin
	chown -R ${USER}:${USER} /config/caddy /data/caddy

USER ${USER}
```

### Yetenekler Olmadan Çalıştırma

Root yetkisi olmadan çalıştırıldığında bile, FrankenPHP web sunucusunu ayrıcalıklı bağlantı noktalarına (80 ve 443) bağlamak için `CAP_NET_BIND_SERVICE` yeteneğine ihtiyaç duyar.

FrankenPHP'yi ayrıcalıklı olmayan bir bağlantı noktasında (1024 ve üzeri) ifşa ederseniz, web sunucusunu root olmayan bir kullanıcı olarak ve herhangi bir yeteneğe ihtiyaç duymadan çalıştırmak mümkündür:

```dockerfile
FROM dunglas/frankenphp

ARG USER=appuser

RUN \
	# Alpine tabanlı dağıtımlar için "adduser -D ${USER}" kullanın
	useradd ${USER}; \
	# Varsayılan yeteneği kaldırın
	setcap -r /usr/local/bin/frankenphp; \
	# /config/caddy ve /data/caddy dizinlerine yazma erişimi verin
	chown -R ${USER}:${USER} /config/caddy /data/caddy

USER ${USER}
```

Ardından, ayrıcalıklı olmayan bir bağlantı noktası kullanmak için `SERVER_NAME` ortam değişkenini ayarlayın.
Örnek: `:8000`

## Güncellemeler

Docker imajları oluşturulur:

- Yeni bir sürüm etiketlendiğinde
- Her gün UTC ile saat 4'te, resmi PHP imajlarının yeni sürümleri mevcutsa

## Geliştirme Sürümleri

Geliştirme sürümleri [`dunglas/frankenphp-dev`](https://hub.docker.com/repository/docker/dunglas/frankenphp-dev) Docker deposunda mevcuttur.
GitHub deposunun ana dalına her commit yapıldığında yeni bir derleme tetiklenir.

`latest*` etiketleri `main` dalının başına işaret eder.
``sha-<git-commit-hash>`` biçimindeki etiketler de mevcuttur.
