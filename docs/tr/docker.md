# Özel Docker İmajı Oluşturma

[Resmi PHP imajları](https://hub.docker.com/_/php/) temel alınarak [FrankenPHP Docker imajları](https://hub.docker.com/r/dunglas/frankenphp) hazırlanmıştır. Popüler mimariler için Debian ve Alpine Linux varyantları sağlanmıştır. Debian dağıtımı tavsiye edilir.

PHP 8.2, 8.3, 8.4 ve 8.5 için varyantlar sağlanmıştır. [Etiketlere göz atın](https://hub.docker.com/r/dunglas/frankenphp/tags).

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

## Daha Fazla PHP Eklentisi Nasıl Kurulur

[Docker-php-extension-installer`](https://github.com/mlocati/docker-php-extension-installer) betiği temel imajda sağlanmıştır.
Ek PHP eklentileri eklemek ise gerçekten kolaydır:

```dockerfile
FROM dunglas/frankenphp

# buraya istenilen eklentileri ekleyin:
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

# xcaddy'yi derleyen imaja kopyalayın
COPY --from=caddy:builder /usr/bin/xcaddy /usr/bin/xcaddy

# FrankenPHP oluşturmak için CGO etkinleştirilmelidir
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
        # Mercure ve Vulcain resmi yapıya dahil edilmiştir, ancak bunları kaldırmaktan çekinmeyin
        --with github.com/dunglas/mercure/caddy \
        --with github.com/dunglas/vulcain/caddy
        # Buraya ekstra Caddy modülleri ekleyin

FROM dunglas/frankenphp AS runner

# Resmi binary dosyayı özel modüllerinizi içeren binary dosyayla değiştirin
COPY --from=builder /usr/local/bin/frankenphp /usr/local/bin/frankenphp
```

FrankenPHP tarafından sağlanan `builder` imajı `libphp`'nin derlenmiş bir sürümünü içerir.
[Derleyici imajları](https://hub.docker.com/r/dunglas/frankenphp/tags?name=builder) hem Debian hem de Alpine için FrankenPHP ve PHP'nin tüm sürümleri için sağlanmıştır.

> [!TIP]
>
> Eğer Alpine Linux ve Symfony kullanıyorsanız,
> [varsayılan yığın boyutunu artırmanız](compile.md#xcaddy-kullanımı) gerekebilir.

## Varsayılan Olarak Worker Modunun Etkinleştirilmesi

FrankenPHP'yi bir worker betiği ile başlatmak için `FRANKENPHP_CONFIG` ortam değişkenini ayarlayın:

```dockerfile
FROM dunglas/frankenphp

# ...

ENV FRANKENPHP_CONFIG="worker ./public/index.php"
```

## Geliştirme Sürecinde Yığın (Volume) Kullanma

FrankenPHP ile kolayca geliştirme yapmak için, uygulamanın kaynak kodunu içeren dizini ana bilgisayarınızdan Docker konteynerine bir yığın (volume) olarak bağlayın:

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

# Caddy sertifikaları ve yapılandırması için gereken yığınlar (volumes)
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
	# 80 ve 443 numaralı bağlantı noktalarına bağlanmak için ek özellik ekleyin
	setcap CAP_NET_BIND_SERVICE=+eip /usr/local/bin/frankenphp; \
	# /data/caddy ve /config/caddy dosyalarına yazma erişimi verin
	chown -R ${USER}:${USER} /data/caddy && chown -R ${USER}:${USER} /config/caddy;

USER ${USER}
```

## Güncellemeler

Docker imajları oluşturulur:

- Yeni bir sürüm etiketlendiğinde
- Her gün UTC ile saat 4'te Resmi PHP imajlarının yeni sürümleri mevcutsa

## Geliştirme Sürümleri

Geliştirme sürümleri [`dunglas/frankenphp-dev`](https://hub.docker.com/repository/docker/dunglas/frankenphp-dev) Docker deposunda mevcuttur.
GitHub deposunun ana dalına her commit yapıldığında yeni bir derleme tetiklenir.

`latest*` etiketleri `main` dalının başına işaret eder.
`sha-<hash-du-commit-git>` biçimindeki etiketler de kullanılabilir.
