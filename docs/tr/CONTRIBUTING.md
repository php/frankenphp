# Katkıda Bulunmak

## PHP Derleme

### Docker ile (Linux)

Geliştirme Ortamı için Docker İmajını Oluşturun:

```console
docker build -t frankenphp-dev -f dev.Dockerfile .
docker run --cap-add=SYS_PTRACE --security-opt seccomp=unconfined -p 8080:8080 -p 443:443 -p 443:443/udp -v $PWD:/go/src/app -it frankenphp-dev
```

İmaj genel geliştirme araçlarını (Go, GDB, Valgrind, Neovim...) içerir ve aşağıdaki php ayar konumlarını kullanır

- php.ini: `/etc/frankenphp/php.ini` Varsayılan olarak geliştirme ön ayarlarına sahip bir php.ini dosyası sağlanır.
- ek yapılandırma dosyaları: `/etc/frankenphp/php.d/*.ini`
- php uzantıları: `/usr/lib/frankenphp/modules/`

Docker sürümünüz 23.0'dan düşükse, derleme dockerignore [pattern issue](https://github.com/moby/moby/pull/42676) nedeniyle başarısız olacaktır. Dizinleri `.dockerignore` dosyasına ekleyin.

```patch
 !testdata/*.php
 !testdata/*.txt
+!caddy
+!internal
```

### Docker olmadan (Linux ve macOS)

[Kaynaklardan derlemek için talimatları izleyin](https://frankenphp.dev/docs/compile/) ve `--debug` yapılandırma seçeneğini geçirin.

## Test senaryolarını çalıştırma

```console
go test -tags watcher -race -v ./...
```

## Caddy modülü

FrankenPHP Caddy modülü ile Caddy'yi oluşturun:

```console
cd caddy/frankenphp/
go build
cd ../../
```

Caddy'yi FrankenPHP Caddy modülü ile çalıştırın:

```console
cd testdata/
../caddy/frankenphp/frankenphp run
```

Sunucu `127.0.0.1:8080` adresini dinliyor:

```console
curl -vk https://localhost/phpinfo.php
```

## Minimal test sunucusu

Minimal test sunucusunu oluşturun:

```console
cd internal/testserver/
go build
cd ../../
```

Test sunucusunu çalıştırın:

```console
cd testdata/
../internal/testserver/testserver
```

Sunucu `127.0.0.1:8080` adresini dinliyor:

```console
curl -v http://127.0.0.1:8080/phpinfo.php
```

## Docker İmajlarını Yerel Olarak Oluşturma

Bake (pişirme) planını yazdırın:

```console
docker buildx bake -f docker-bake.hcl --print
```

Yerel olarak amd64 için FrankenPHP görüntüleri oluşturun:

```console
docker buildx bake -f docker-bake.hcl --pull --load --set "*.platform=linux/amd64"
```

Yerel olarak arm64 için FrankenPHP görüntüleri oluşturun:

```console
docker buildx bake -f docker-bake.hcl --pull --load --set "*.platform=linux/arm64"
```

FrankenPHP imajlarını arm64 ve amd64 için sıfırdan oluşturun ve Docker Hub'a gönderin:

```console
docker buildx bake -f docker-bake.hcl --pull --no-cache --push
```

## Statik Derlemelerle Segmentasyon Hatalarında Hata Ayıklama

1. FrankenPHP binary dosyasının hata ayıklama sürümünü GitHub'dan indirin veya hata ayıklama seçeneklerini kullanarak özel statik derlemenizi oluşturun:

   ```console
   docker buildx bake \
       --load \
       --set static-builder.args.DEBUG_SYMBOLS=1 \
       --set "static-builder.platform=linux/amd64" \
       static-builder
   docker cp $(docker create --name static-builder-musl dunglas/frankenphp:static-builder-musl):/go/src/app/dist/frankenphp-linux-$(uname -m) frankenphp
   ```

2. Mevcut `frankenphp` sürümünüzü hata ayıklama FrankenPHP çalıştırılabilir dosyasıyla değiştirin
3. FrankenPHP'yi her zamanki gibi başlatın (alternatif olarak FrankenPHP'yi doğrudan GDB ile başlatabilirsiniz: `gdb --args frankenphp run`)
4. GDB ile sürece bağlanın:

   ```console
   gdb -p `pidof frankenphp`
   ```

5. Gerekirse, GDB kabuğuna `continue` yazın
6. FrankenPHP'nin çökmesini sağlayın
7. GDB kabuğuna `bt` yazın
8. Çıktıyı kopyalayın

## GitHub Eylemlerinde Segmentasyon Hatalarında Hata Ayıklama

1. `.github/workflows/tests.yml` dosyasını açın
2. PHP hata ayıklama seçeneklerini etkinleştirin

   ```patch
       - uses: shivammathur/setup-php@v2
         # ...
         env:
           phpts: ts
   +       debug: true
   ```

3. Konteynere bağlanmak için `tmate`i etkinleştirin

   ```patch
       -
         name: Set CGO flags
         run: echo "CGO_CFLAGS=$(php-config --includes)" >> "$GITHUB_ENV"
   +   -
   +     run: |
   +       sudo apt install gdb
   +       mkdir -p /home/runner/.config/gdb/
   +       printf "set auto-load safe-path /\nhandle SIG34 nostop noprint pass" > /home/runner/.config/gdb/gdbinit
   +   -
   +     uses: mxschmitt/action-tmate@v3
   ```

4. Konteynere bağlanın
5. `frankenphp.go` dosyasını açın
6. `cgosymbolizer`'ı etkinleştirin

   ```patch
   -	//_ "github.com/ianlancetaylor/cgosymbolizer"
   +	_ "github.com/ianlancetaylor/cgosymbolizer"
   ```

7. Modülü indirin: `go get`
8. Konteynerde GDB ve benzerlerini kullanabilirsiniz:

   ```console
   go test -tags watcher -c -ldflags=-w
   gdb --args frankenphp.test -test.run ^MyTest$
   ```

9. Hata düzeltildiğinde, tüm bu değişiklikleri geri alın

## Misc Dev Resources

- [uWSGI içine PHP gömme](https://github.com/unbit/uwsgi/blob/master/plugins/php/php_plugin.c)
- [NGINX Unit'te PHP gömme](https://github.com/nginx/unit/blob/master/src/nxt_php_sapi.c)
- [Go (go-php) içinde PHP gömme](https://github.com/deuill/go-php)
- [Go'da PHP gömme (GoEmPHP)](https://github.com/mikespook/goemphp)
- [C++'da PHP gömme](https://gist.github.com/paresy/3cbd4c6a469511ac7479aa0e7c42fea7)
- [Sara Golemon tarafından PHP'yi Genişletme ve Yerleştirme](https://books.google.fr/books?id=zMbGvK17_tYC&pg=PA254&lpg=PA254#v=onepage&q&f=false)
- [TSRMLS_CC de neyin nesi?](http://blog.golemon.com/2006/06/what-heck-is-tsrmlscc-anyway.html)
- [Mac'te PHP gömme](https://gist.github.com/jonnywang/61427ffc0e8dde74fff40f479d147db4)
- [SDL bağları](https://pkg.go.dev/github.com/veandco/go-sdl2@v0.4.21/sdl#Main)

## Docker ile İlgili Kaynaklar

- [Pişirme (bake) dosya tanımı](https://docs.docker.com/build/customize/bake/file-definition/)
- [`docker buildx build`](https://docs.docker.com/engine/reference/commandline/buildx_build/)

## Faydalı Komut

```console
apk add strace util-linux gdb
strace -e 'trace=!futex,epoll_ctl,epoll_pwait,tgkill,rt_sigreturn' -p 1
```
