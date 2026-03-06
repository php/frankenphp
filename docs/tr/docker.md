# Özel Docker İmajı Oluşturma

[FrankenPHP Docker imajları](https://hub.docker.com/r/dunglas/frankenphp), [resmi PHP imajları](https://hub.docker.com/_/php/) temel alınarak hazırlanmıştır. Popüler mimariler için Debian ve Alpine Linux varyantları sağlanmıştır. Debian varyantları tavsiye edilir.

PHP 8.2, 8.3, 8.4 ve 8.5 için varyantlar sağlanmıştır.

Etiketler şu deseni takip eder: `dunglas/frankenphp:<frankenphp-version>-php<php-version>-<os>`

- `<frankenphp-version>` ve `<php-version>`, sırasıyla FrankenPHP ve PHP'nin ana (örn. `1`), ikincil (örn. `1.2`) ve yama sürümlerine (örn. `1.2.3`) kadar değişen sürüm numaralarıdır.
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

Kolaylık sağlamak için, faydalı ortam değişkenleri içeren [varsayılan bir `Caddyfile`](https://github.com/php/frankenphp/blob/main/caddy/frankenphp/Caddyfile) imajda sağlanmıştır.

## Daha Fazla PHP Eklentisi Nasıl Kurulur

[`docker-php-extension-installer`](https://github.com/mlocati/docker-php-extension-installer) betiği temel imajda sağlanmıştır.
Ek PHP eklentileri eklemek basittir:

```dockerfile
FROM dunglas/frankenphp

# ek eklentileri buraya ekleyin:
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

# xcaddy'yi builder imajına kopyalayın
COPY --from=caddy:builder /usr/bin/xcaddy /usr/bin/xcaddy

# CGO, FrankenPHP oluşturmak için etkinleştirilmelidir
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
        # Mercure ve Vulcain resmi derlemeye dahildir, ancak bunları kaldırmaktan çekinmeyin
        --with github.com/dunglas/mercure/caddy \
        --with github.com/dunglas/vulcain/caddy
        # Ek Caddy modüllerini buraya ekleyin

FROM dunglas/frankenphp AS runner

# Resmi binary dosyayı özel modüllerinizi içeren binary dosyayla değiştirin
COPY --from=builder /usr/local/bin/frankenphp /usr/local/bin/frankenphp
```

FrankenPHP tarafından sağlanan `builder` imajı `libphp`'nin derlenmiş bir sürümünü içerir.
[Builder imajları](https://hub.docker.com/r/dunglas/frankenphp/tags?name=builder) hem Debian hem de Alpine için FrankenPHP ve PHP'nin tüm sürümleri için sağlanmıştır.

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

## Geliştirme Sürecinde Volume Kullanma

FrankenPHP ile kolayca geliştirme yapmak için, uygulamanın kaynak kodunu içeren dizini ana bilgisayarınızdan Docker konteynerine bir volume olarak bağlayın:

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
    # özel bir Dockerfile kullanmak istiyorsanız aşağıdaki satırın yorumunu kaldırın
    #build: .
    # bunu bir üretim ortamında çalıştırmak istiyorsanız aşağıdaki satırın yorumunu kaldırın
    # restart: always
    ports:
      - "80:80" # HTTP
      - "443:443" # HTTPS
      - "443:443/udp" # HTTP/3
    volumes:
      - ./:/app/public
      - caddy_data:/data
      - caddy_config:/config
    # üretimde aşağıdaki satırı yorum satırı yapın; geliştirme ortamında ise güzel, insan tarafından okunabilir günlükler sağlar
    tty: true

# Caddy sertifikaları ve yapılandırması için gereken volume'ler
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
	# /config/caddy ve /data/caddy dosyalarına yazma erişimi verin
	chown -R ${USER}:${USER} /config/caddy /data/caddy

USER ${USER}
```

### Yetenek Olmadan Çalıştırma

FrankenPHP, root yetkisi olmadan çalışırken bile, web sunucusunu ayrıcalıklı bağlantı noktalarında (80 ve 443) bağlamak için `CAP_NET_BIND_SERVICE` yeteneğine ihtiyaç duyar.

FrankenPHP'yi ayrıcalıklı olmayan bir bağlantı noktasında (1024 ve üzeri) çalıştırırsanız, web sunucusunu root olmayan bir kullanıcı olarak ve herhangi bir yeteneğe ihtiyaç duymadan çalıştırmak mümkündür:

```dockerfile
FROM dunglas/frankenphp

ARG USER=appuser

RUN \
	# Alpine tabanlı dağıtımlar için "adduser -D ${USER}" kullanın
	useradd ${USER}; \
	# Varsayılan yeteneği kaldırın
	setcap -r /usr/local/bin/frankenphp; \
	# /config/caddy ve /data/caddy dosyalarına yazma erişimi verin
	chown -R ${USER}:${USER} /config/caddy /data/caddy

USER ${USER}
```

Ardından, ayrıcalıklı olmayan bir bağlantı noktası kullanmak için `SERVER_NAME` ortam değişkenini ayarlayın.
Örnek: `:8000`

## Güncellemeler

Docker imajları oluşturulur:

- Yeni bir sürüm etiketlendiğinde
- Her gün UTC ile sabah 4'te, resmi PHP imajlarının yeni sürümleri mevcutsa

## İmajları Sertleştirme

FrankenPHP Docker imajlarınızın saldırı yüzeyini ve boyutunu daha da azaltmak için, onları [Google distroless](https://github.com/GoogleContainerTools/distroless) veya [Docker hardened](https://www.docker.com/products/hardened-images) bir imaj üzerine inşa etmek de mümkündür.

> [!WARNING]
> Bu minimal temel imajlar, hata ayıklamayı zorlaştıran bir kabuk veya paket yöneticisi içermez.
> Bu nedenle, güvenlik yüksek öncelikliyse yalnızca üretim için önerilirler.

Ek PHP eklentileri eklerken, bir ara derleme aşamasına ihtiyacınız olacaktır:

```dockerfile
FROM dunglas/frankenphp AS builder

# Ek PHP eklentilerini buraya ekleyin
RUN install-php-extensions pdo_mysql pdo_pgsql #...

# frankenphp'nin paylaşılan kütüphanelerini ve kurulu tüm eklentileri geçici bir konuma kopyalayın
# Bu adımı, frankenphp binary'sinin ve her bir eklenti .so dosyasının ldd çıktısını analiz ederek manuel olarak da yapabilirsiniz
RUN apt-get update && apt-get install -y libtree && \
    EXT_DIR="$(php -r 'echo ini_get("extension_dir");')" && \
    FRANKENPHP_BIN="$(which frankenphp)"; \
    LIBS_TMP_DIR="/tmp/libs"; \
    mkdir -p "$LIBS_TMP_DIR"; \
    for target in "$FRANKENPHP_BIN" $(find "$EXT_DIR" -maxdepth 2 -type f -name "*.so"); do \
        libtree -pv "$target" | sed 's/.*── \(.*\) \[.*/\1/' | grep -v "^$target" | while IFS= read -r lib; do \
            [ -z "$lib" ] && continue; \
            base=$(basename "$lib"); \
            destfile="$LIBS_TMP_DIR/$base"; \
            if [ ! -f "$destfile" ]; then \
                cp "$lib" "$destfile"; \
            fi; \
        done; \
    done


# Distroless debian temel imajı, bunun temel imajla aynı debian sürümü olduğundan emin olun
FROM gcr.io/distroless/base-debian13
# Docker hardened imaj alternatifi
# FROM dhi.io/debian:13

# Uygulamanızın ve Caddyfile'ınızın konteynere kopyalanacak konumu
ARG PATH_TO_APP="."
ARG PATH_TO_CADDYFILE="./Caddyfile"

# Uygulamanızı /app'e kopyalayın
# Daha fazla sertleştirme için, yalnızca yazılabilir yolların nonroot kullanıcısına ait olduğundan emin olun
COPY --chown=nonroot:nonroot "$PATH_TO_APP" /app
COPY "$PATH_TO_CADDYFILE" /etc/caddy/Caddyfile

# frankenphp'yi ve gerekli kütüphaneleri kopyalayın
COPY --from=builder /usr/local/bin/frankenphp /usr/local/bin/frankenphp
COPY --from=builder /usr/local/lib/php/extensions /usr/local/lib/php/extensions
COPY --from=builder /tmp/libs /usr/lib

# php.ini yapılandırma dosyalarını kopyalayın
COPY --from=builder /usr/local/etc/php/conf.d /usr/local/etc/php/conf.d
COPY --from=builder /usr/local/etc/php/php.ini-production /usr/local/etc/php/php.ini

# Caddy veri dizinleri — salt okunur bir kök dosya sisteminde bile nonroot için yazılabilir olmalıdır
ENV XDG_CONFIG_HOME=/config \
    XDG_DATA_HOME=/data
COPY --from=builder --chown=nonroot:nonroot /data/caddy /data/caddy
COPY --from=builder --chown=nonroot:nonroot /config/caddy /config/caddy

USER nonroot

WORKDIR /app

# Sağlanan Caddyfile ile frankenphp'yi çalıştırmak için giriş noktası
ENTRYPOINT ["/usr/local/bin/frankenphp", "run", "-c", "/etc/caddy/Caddyfile"]
```

## Geliştirme Sürümleri

Geliştirme sürümleri [`dunglas/frankenphp-dev`](https://hub.docker.com/repository/docker/dunglas/frankenphp-dev) Docker deposunda mevcuttur.
GitHub deposunun `main` dalına her commit gönderildiğinde yeni bir derleme tetiklenir.

`latest*` etiketleri `main` dalının başına işaret eder.
`sha-<git-commit-hash>` biçimindeki etiketler de mevcuttur.
