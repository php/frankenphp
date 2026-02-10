# Performans

Varsayılan olarak, FrankenPHP performans ve kullanım kolaylığı arasında iyi bir denge sunmaya çalışır.
Ancak, uygun bir yapılandırma kullanılarak performansı önemli ölçüde artırmak mümkündür.

## İş Parçacığı ve İşçi Sayısı

Varsayılan olarak, FrankenPHP mevcut CPU çekirdeği sayısının 2 katı kadar iş parçacığı ve işçi (işçi modunda) başlatır.

Uygun değerler, uygulamanızın nasıl yazıldığına, ne yaptığına ve donanımınıza büyük ölçüde bağlıdır.
Bu değerleri değiştirmenizi şiddetle tavsiye ederiz. En iyi sistem kararlılığı için, `num_threads` x `memory_limit` < `available_memory` olması önerilir.

Doğru değerleri bulmak için gerçek trafiği simüle eden yük testleri yapmak en iyisidir.
[k6](https://k6.io) ve [Gatling](https://gatling.io) bunun için iyi araçlardır.

İş parçacığı sayısını yapılandırmak için `php_server` ve `php` yönergelerinin `num_threads` seçeneğini kullanın.
İşçi sayısını değiştirmek için `frankenphp` yönergesinin `worker` bölümünün `num` seçeneğini kullanın.

### `max_threads`

Trafiğinizin neye benzeyeceğini tam olarak bilmek her zaman daha iyi olsa da, gerçek dünya uygulamaları daha
tahmin edilemez olma eğilimindedir. `max_threads` [yapılandırması](config.md#caddyfile-config), FrankenPHP'nin çalışma zamanında belirtilen sınıra kadar ek iş parçacıkları otomatik olarak oluşturmasına olanak tanır.
`max_threads`, trafiğinizi yönetmek için kaç iş parçacığına ihtiyacınız olduğunu anlamanıza yardımcı olabilir ve sunucuyu gecikme artışlarına karşı daha dirençli hale getirebilir.
Eğer `auto` olarak ayarlanırsa, sınır `php.ini` dosyanızdaki `memory_limit` değerine göre tahmin edilecektir. Bunu yapamazsa,
`auto` bunun yerine varsayılan olarak 2x `num_threads` olacaktır. `auto`'nun ihtiyaç duyulan iş parçacığı sayısını büyük ölçüde küçümseyebileceğini unutmayın.
`max_threads`, PHP FPM'nin [pm.max_children](https://www.php.net/manual/en/install.fpm.configuration.php#pm.max-children) ile benzerdir. Temel fark, FrankenPHP'nin süreçler yerine
iş parçacıkları kullanması ve gerektiğinde bunları farklı işçi komut dosyaları ve 'klasik mod' arasında otomatik olarak devretmesidir.

## İşçi Modu

[İşçi modunu](worker.md) etkinleştirmek performansı önemli ölçüde artırır,
ancak uygulamanızın bu modla uyumlu olacak şekilde uyarlanması gerekir:
bir işçi komut dosyası oluşturmanız ve uygulamanın bellek sızdırmadığından emin olmanız gerekir.

## musl Kullanmayın

Resmi Docker imajlarının Alpine Linux varyantı ve sağladığımız varsayılan ikili dosyalar [musl libc](https://musl.libc.org) kullanmaktadır.

PHP'nin, geleneksel GNU kitaplığı yerine bu alternatif C kitaplığını kullandığında [daha yavaş olduğu](https://gitlab.alpinelinux.org/alpine/aports/-/issues/14381) bilinmektedir,
özellikle de FrankenPHP için gerekli olan ZTS modunda (iş parçacığı güvenli) derlendiğinde. Fark, yoğun iş parçacıklı bir ortamda önemli olabilir.

Ayrıca, [bazı hatalar yalnızca musl kullanıldığında ortaya çıkar](https://github.com/php/php-src/issues?q=sort%3Aupdated-desc+is%3Aissue+is%3Aopen+label%3ABug+musl).

Üretim ortamlarında, glibc'ye bağlı, uygun bir optimizasyon seviyesiyle derlenmiş FrankenPHP kullanmanızı öneririz.

Bu, Debian Docker imajlarını kullanarak, [bakımcılarımızın .deb, .rpm veya .apk paketlerini](https://pkgs.henderkes.com) kullanarak veya [FrankenPHP'yi kaynak koddan derleyerek](compile.md) başarılabilir.

Daha yalın veya daha güvenli konteynerler için Alpine yerine [güçlendirilmiş bir Debian imajı](docker.md#hardening-images) kullanmayı düşünebilirsiniz.

## Go Çalışma Zamanı Yapılandırması

FrankenPHP Go ile yazılmıştır.

Genel olarak, Go çalışma zamanı özel bir yapılandırma gerektirmez, ancak belirli durumlarda,
özel yapılandırma performansı artırır.

Muhtemelen `GODEBUG` ortam değişkenini `cgocheck=0` olarak ayarlamak isteyeceksiniz (FrankenPHP Docker imajlarındaki varsayılan değer).

FrankenPHP'yi konteynerlerde (Docker, Kubernetes, LXC...) çalıştırıyorsanız ve konteynerler için ayrılan belleği sınırlıyorsanız,
`GOMEMLIMIT` ortam değişkenini mevcut bellek miktarına ayarlayın.

Daha fazla ayrıntı için, [Go dokümantasyon sayfasının bu konuya ayrılmış bölümünü](https://pkg.go.dev/runtime#hdr-Environment_Variables) okumanız, çalışma zamanından en iyi şekilde yararlanmak için zorunludur.

## `file_server`

Varsayılan olarak, `php_server` yönergesi,
kök dizinde depolanan statik dosyaları (varlıkları) sunmak için otomatik olarak bir dosya sunucusu kurar.

Bu özellik kullanışlıdır, ancak bir maliyeti vardır.
Bunu devre dışı bırakmak için aşağıdaki yapılandırmayı kullanın:

```caddyfile
php_server {
    file_server off
}
```

## `try_files`

Statik dosyalar ve PHP dosyaları dışında, `php_server` uygulamanızın dizin dizini ve dizin dizini dosyalarını (`/path/` -> `/path/index.php`) da sunmaya çalışacaktır. Dizin dizinlerine ihtiyacınız yoksa,
`try_files` değerini açıkça şu şekilde tanımlayarak bunları devre dışı bırakabilirsiniz:

```caddyfile
php_server {
    try_files {path} index.php
    root /root/to/your/app # kökü buraya açıkça eklemek daha iyi önbelleğe alma sağlar
}
```

Bu, gereksiz dosya işlemlerinin sayısını önemli ölçüde azaltabilir.
Önceki yapılandırmanın bir işçi eşdeğeri şöyle olacaktır:

```caddyfile
route {
    php_server { # dosya sunucusuna hiç ihtiyacınız yoksa "php_server" yerine "php" kullanın
        root /root/to/your/app
        worker /path/to/worker.php {
            match * # tüm istekleri doğrudan işçiye gönder
        }
    }
}
```

0 gereksiz dosya sistemi işlemiyle alternatif bir yaklaşım, bunun yerine `php` yönergesini kullanmak ve dosyaları PHP'den yola göre ayırmaktır. Bu yaklaşım, tüm uygulamanızın tek bir giriş dosyası tarafından sunulması durumunda iyi çalışır.
Statik dosyaları bir `/assets` klasörünün arkasında sunan bir örnek [yapılandırma](config.md#caddyfile-config) şöyle görünebilir:

```caddyfile
route {
    @assets {
        path /assets/*
    }

    # /assets arkasındaki her şey dosya sunucusu tarafından işlenir
    file_server @assets {
        root /root/to/your/app
    }

    # /assets içinde olmayan her şey dizininiz veya işçi PHP dosyanız tarafından işlenir
    rewrite index.php
    php {
        root /root/to/your/app # kökü buraya açıkça eklemek daha iyi önbelleğe alma sağlar
    }
}
```

## Yer Tutucular

`root` ve `env` yönergelerinde [yer tutucular](https://caddyserver.com/docs/conventions#placeholders) kullanabilirsiniz.
Ancak bu, bu değerlerin önbelleğe alınmasını engeller ve önemli bir performans maliyetiyle birlikte gelir.

Mümkünse, bu yönergelerde yer tutuculardan kaçının.

## `resolve_root_symlink`

Varsayılan olarak, belge kökü sembolik bir bağlantıysa, FrankenPHP tarafından otomatik olarak çözümlenir (PHP'nin düzgün çalışması için bu gereklidir).
Belge kökü bir sembolik bağlantı değilse, bu özelliği devre dışı bırakabilirsiniz.

```caddyfile
php_server {
    resolve_root_symlink false
}
```

Bu, `root` yönergesi [yer tutucular](https://caddyserver.com/docs/conventions#placeholders) içeriyorsa performansı artıracaktır.
Diğer durumlarda kazanç ihmal edilebilir olacaktır.

## Günlükler

Günlük kaydı açıkça çok faydalıdır, ancak tanım gereği,
giriş/çıkış işlemleri ve bellek ayırmaları gerektirir, bu da performansı önemli ölçüde azaltır.
[Günlük seviyesini](https://caddyserver.com/docs/caddyfile/options#log) doğru bir şekilde ayarladığınızdan emin olun,
ve yalnızca gerekli olanı günlüğe kaydedin.

## PHP Performansı

FrankenPHP resmi PHP yorumlayıcısını kullanır.
Tüm olağan PHP ile ilgili performans optimizasyonları FrankenPHP ile de geçerlidir.

Özellikle:

- [OPcache](https://www.php.net/manual/en/book.opcache.php)'in kurulu, etkin ve doğru şekilde yapılandırıldığını kontrol edin
- [Composer otomatik yükleyici optimizasyonlarını](https://getcomposer.org/doc/articles/autoloader-optimization.md) etkinleştirin
- `realpath` önbelleğinin uygulamanızın ihtiyaçları için yeterince büyük olduğundan emin olun
- [ön yüklemeyi](https://www.php.net/manual/en/opcache.preloading.php) kullanın

Daha fazla ayrıntı için, [Symfony'nin bu konuya ayrılmış dokümantasyon girişini](https://symfony.com/doc/current/performance.html) okuyun
(ipuçlarının çoğu Symfony kullanmasanız bile faydalıdır).

## İş Parçacığı Havuzunu Bölme

Uygulamaların, yüksek yük altında güvenilmez olma eğiliminde olan veya sürekli olarak 10 saniyeden fazla yanıt veren bir
API gibi yavaş harici hizmetlerle etkileşime girmesi yaygındır.
Bu gibi durumlarda, özel "yavaş" havuzlara sahip olmak için iş parçacığı havuzunu bölmek faydalı olabilir.
Bu, yavaş uç noktaların tüm sunucu kaynaklarını/iş parçacıklarını tüketmesini önler ve
bağlantı havuzuna benzer şekilde, yavaş uç noktaya giden isteklerin eş zamanlılığını sınırlar.

```caddyfile
example.com {
    php_server {
        root /app/public # uygulamanızın kök dizini
        worker index.php {
            match /slow-endpoint/* # /slow-endpoint/* yoluyla eşleşen tüm istekler bu iş parçacığı havuzu tarafından işlenir
            num 1 # /slow-endpoint/* ile eşleşen istekler için minimum 1 iş parçacığı
            max_threads 20 # /slow-endpoint/* ile eşleşen istekler için gerektiğinde 20 iş parçacığına kadar izin ver
        }
        worker index.php {
            match * # diğer tüm istekler ayrı ayrı işlenir
            num 1 # diğer istekler için minimum 1 iş parçacığı, yavaş uç noktalar asılı kalmaya başlasa bile
            max_threads 20 # diğer istekler için gerektiğinde 20 iş parçacığına kadar izin ver
        }
    }
}
```

Genel olarak, çok yavaş uç noktaları, mesaj kuyrukları gibi ilgili mekanizmalar kullanarak eşzamansız olarak ele almak da tavsiye edilir.
