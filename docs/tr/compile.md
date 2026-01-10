# Kaynak Kodlardan Derleme

Bu doküman, PHP'yi dinamik bir kütüphane olarak yükleyecek bir FrankenPHP yapısının nasıl oluşturulacağını açıklamaktadır.
Önerilen yöntem bu şekildedir.

Alternatif olarak, [statik yapılar oluşturma](static.md) da mümkündür.

## PHP'yi yükleyin

FrankenPHP, PHP 8.2 ve üstü ile uyumludur.

İlk olarak, [PHP'nin kaynaklarını edinin](https://www.php.net/downloads.php) ve bunları çıkarın:

```console
tar xf php-*
cd php-*/
```

Ardından, PHP'yi platformunuz için yapılandırın.

Bu şekilde yapılandırma gereklidir, ancak başka opsiyonlar da ekleyebilirsiniz (örn. ekstra uzantılar)
İhtiyaç halinde.

### Linux

```console
./configure \
    --enable-embed \
    --enable-zts \
    --disable-zend-signals \
    --enable-zend-max-execution-timers
```

### Mac

Yüklemek için [Homebrew](https://brew.sh/) paket yöneticisini kullanın
`libiconv`, `bison`, `re2c` ve `pkg-config`:

```console
brew install libiconv bison re2c pkg-config
echo 'export PATH="/opt/homebrew/opt/bison/bin:$PATH"' >> ~/.zshrc
```

Ardından yapılandırma betiğini çalıştırın:

```console
./configure \
    --enable-embed=static \
    --enable-zts \
    --disable-zend-signals \
    --disable-opcache-jit \
    --enable-static \
    --enable-shared=no \
    --with-iconv=/opt/homebrew/opt/libiconv/
```

## PHP Derleyin

Son olarak, PHP'yi derleyin ve kurun:

```console
make -j"$(getconf _NPROCESSORS_ONLN)"
sudo make install
```

## Go Uygulamasını Derleyin

Artık Go kütüphanesini kullanabilir ve Caddy yapımızı derleyebilirsiniz:

```console
curl -L https://github.com/php/frankenphp/archive/refs/heads/main.tar.gz | tar xz
cd frankenphp-main/caddy/frankenphp
CGO_CFLAGS=$(php-config --includes) CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" go build
```

### Xcaddy kullanımı

Alternatif olarak, FrankenPHP'yi [özel Caddy modülleri](https://caddyserver.com/docs/modules/) ile derlemek için [xcaddy](https://github.com/caddyserver/xcaddy) kullanın:

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags '-w -s'" \
xcaddy build \
    --output frankenphp \
    --with github.com/dunglas/frankenphp/caddy \
    --with github.com/dunglas/caddy-cbrotli \
    --with github.com/dunglas/mercure/caddy \
    --with github.com/dunglas/vulcain/caddy
    # Add extra Caddy modules here
```

> [!TIP]
>
> Eğer musl libc (Alpine Linux'ta varsayılan) ve Symfony kullanıyorsanız,
> varsayılan yığın boyutunu artırmanız gerekebilir.
> Aksi takdirde, şu tarz hatalar alabilirsiniz `PHP Fatal error: Maximum call stack size of 83360 bytes reached during compilation. Try splitting expression`
>
> Bunu yapmak için, `XCADDY_GO_BUILD_FLAGS` ortam değişkenini bu şekilde değiştirin
> `XCADDY_GO_BUILD_FLAGS=$'-ldflags "-w -s -extldflags \'-Wl,-z,stack-size=0x80000\'"'`
> (yığın boyutunun değerini uygulamanızın ihtiyaçlarına göre değiştirin).
