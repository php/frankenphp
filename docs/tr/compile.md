# Kaynak Kodlardan Derleme

Bu belge, PHP'yi dinamik bir kütüphane olarak yükleyecek bir FrankenPHP ikili dosyasının nasıl oluşturulacağını açıklamaktadır.
Önerilen yöntem budur.

Alternatif olarak, [tamamen ve çoğunlukla statik yapılar](static.md) da oluşturulabilir.

## PHP'yi Kurun

FrankenPHP, PHP 8.2 ve üstü ile uyumludur.

### Homebrew ile (Linux ve Mac)

FrankenPHP ile uyumlu bir libphp sürümünü kurmanın en kolay yolu, [Homebrew PHP](https://github.com/shivammathur/homebrew-php) tarafından sağlanan ZTS paketlerini kullanmaktır.

İlk olarak, eğer daha önce yapmadıysanız, [Homebrew](https://brew.sh) kurun.

Ardından, PHP'nin ZTS varyantını, Brotli'yi (isteğe bağlı, sıkıştırma desteği için) ve watcher'ı (isteğe bağlı, dosya değişikliği algılama için) kurun:

```console
brew install shivammathur/php/php-zts brotli watcher
brew link --overwrite --force shivammathur/php/php-zts
```

### PHP'yi Kaynak Koddan Derleyerek

Alternatif olarak, aşağıdaki adımları izleyerek PHP'yi FrankenPHP için gerekli seçeneklerle kaynak koddan derleyebilirsiniz.

İlk olarak, [PHP kaynaklarını edinin](https://www.php.net/downloads.php) ve çıkarın:

```console
tar xf php-*
cd php-*/
```

Ardından, platformunuz için gerekli seçeneklerle `configure` betiğini çalıştırın.
Aşağıdaki `./configure` bayrakları zorunludur, ancak örneğin uzantıları veya ek özellikleri derlemek için başka bayraklar ekleyebilirsiniz.

#### Linux

```console
./configure \
    --enable-embed \
    --enable-zts \
    --disable-zend-signals \
    --enable-zend-max-execution-timers
```

#### Mac

Gerekli ve isteğe bağlı bağımlılıkları kurmak için [Homebrew](https://brew.sh/) paket yöneticisini kullanın:

```console
brew install libiconv bison brotli re2c pkg-config watcher
echo 'export PATH="/opt/homebrew/opt/bison/bin:$PATH"' >> ~/.zshrc
```

Ardından yapılandırma betiğini çalıştırın:

```console
./configure \
    --enable-embed \
    --enable-zts \
    --disable-zend-signals \
    --with-iconv=/opt/homebrew/opt/libiconv/
```

#### PHP'yi Derleyin

Son olarak, PHP'yi derleyin ve kurun:

```console
make -j"$(getconf _NPROCESSORS_ONLN)"
sudo make install
```

## İsteğe Bağlı Bağımlılıkları Kurun

Bazı FrankenPHP özellikleri, kurulması gereken isteğe bağlı sistem bağımlılıklarına sahiptir.
Alternatif olarak, bu özellikler Go derleyicisine derleme etiketleri (build tags) geçirilerek devre dışı bırakılabilir.

| Özellik                          | Bağımlılık                                                                                                       | Devre dışı bırakma derleme etiketi |
| -------------------------------- | ---------------------------------------------------------------------------------------------------------------- | ---------------------------------- |
| Brotli sıkıştırma                | [Brotli](https://github.com/google/brotli)                                                                       | nobrotli                           |
| Dosya değişikliğinde işçileri yeniden başlatma | [Watcher C](https://github.com/e-dant/watcher/tree/release/watcher-c)                                            | nowatcher                          |
| [Mercure](mercure.md)            | [Mercure Go kütüphanesi](https://pkg.go.dev/github.com/dunglas/mercure) (otomatik olarak kurulur, AGPL lisanslı) | nomercure                          |

## Go Uygulamasını Derleyin

Artık nihai ikili dosyayı oluşturabilirsiniz.

### Xcaddy Kullanarak

Önerilen yöntem, FrankenPHP'yi derlemek için [xcaddy](https://github.com/caddyserver/xcaddy) kullanmaktır.
`xcaddy` ayrıca [özel Caddy modüllerini](https://caddyserver.com/docs/modules/) ve FrankenPHP uzantılarını kolayca eklemenizi sağlar:

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/dunglas/frankenphp/caddy \
    --with github.com/dunglas/mercure/caddy \
    --with github.com/dunglas/vulcain/caddy \
    --with github.com/dunglas/caddy-cbrotli
    # Buraya ek Caddy modülleri ve FrankenPHP uzantıları ekleyin
    # isteğe bağlı olarak, frankenphp kaynaklarınızdan derlemek isterseniz:
    # --with github.com/dunglas/frankenphp=$(pwd) \
    # --with github.com/dunglas/frankenphp/caddy=$(pwd)/caddy

```

> [!TIP]
>
> Eğer musl libc (Alpine Linux'ta varsayılan) ve Symfony kullanıyorsanız,
> varsayılan yığın boyutunu artırmanız gerekebilir.
> Aksi takdirde, derleme sırasında `PHP Fatal error: Maximum call stack size of 83360 bytes reached during compilation. Try splitting expression` gibi hatalar alabilirsiniz.
>
> Bunu yapmak için, `XCADDY_GO_BUILD_FLAGS` ortam değişkenini şöyle değiştirin:
> `XCADDY_GO_BUILD_FLAGS=$'-ldflags "-w -s -extldflags \'-Wl,-z,stack-size=0x80000\'"'`
> (yığın boyutu değerini uygulamanızın ihtiyaçlarına göre değiştirin).

### Xcaddy Kullanmadan

Alternatif olarak, FrankenPHP'yi `go` komutunu doğrudan kullanarak `xcaddy` olmadan derlemek mümkündür:

```console
curl -L https://github.com/php/frankenphp/archive/refs/heads/main.tar.gz | tar xz
cd frankenphp-main/caddy/frankenphp
CGO_CFLAGS=$(php-config --includes) CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" go build -tags=nobadger,nomysql,nopgx
```
