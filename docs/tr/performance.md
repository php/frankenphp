# Performans

Varsayılan olarak, FrankenPHP performans ve kullanım kolaylığı arasında iyi bir denge sunmaya çalışır. Ancak, uygun bir yapılandırma kullanarak performansı önemli ölçüde artırmak mümkündür.

## İş Parçacığı ve Çalışan Sayısı

Varsayılan olarak, FrankenPHP, mevcut CPU sayısının 2 katı kadar iş parçacığı ve çalışan (worker modunda) başlatır.

Uygun değerler, uygulamanızın nasıl yazıldığına, ne yaptığına ve donanımınıza büyük ölçüde bağlıdır. Bu değerleri değiştirmenizi şiddetle tavsiye ederiz. En iyi sistem kararlılığı için, `num_threads` x `memory_limit` < `available_memory` olmasına sahip olmanız önerilir.

Doğru değerleri bulmak için gerçek trafiği simüle eden yük testleri çalıştırmak en iyisidir. [k6](https://k6.io) ve [Gatling](https://gatling.io) bunun için iyi araçlardır.

İş parçacığı sayısını yapılandırmak için `php_server` ve `php` direktiflerinin `num_threads` seçeneğini kullanın. Çalışan sayısını değiştirmek için `frankenphp` direktifinin `worker` bölümündeki `num` seçeneğini kullanın.

### `max_threads`

Trafiğinizin tam olarak nasıl olacağını bilmek her zaman daha iyi olsa da, gerçek hayattaki uygulamalar daha öngörülemez olma eğilimindedir. `max_threads` [yapılandırması](config.md#caddyfile-config), FrankenPHP'nin belirtilen sınıra kadar çalışma zamanında otomatik olarak ek iş parçacıkları oluşturmasına olanak tanır. `max_threads`, trafiğinizi yönetmek için kaç iş parçacığına ihtiyacınız olduğunu belirlemenize yardımcı olabilir ve sunucuyu gecikme artışlarına karşı daha dirençli hale getirebilir. `auto` olarak ayarlanırsa, limit `php.ini` dosyanızdaki `memory_limit` değerine göre tahmin edilecektir. Bunu yapamazsa, `auto` bunun yerine varsayılan olarak `num_threads`'in 2 katına ayarlanır. `auto`'nun ihtiyaç duyulan iş parçacığı sayısını büyük ölçüde hafife alabileceğini unutmayın. `max_threads`, PHP FPM'nin [pm.max_children](https://www.php.net/manual/en/install.fpm.configuration.php#pm.max-children) özelliğine benzer. Temel fark, FrankenPHP'nin işlemler yerine iş parçacıkları kullanması ve bunları gerektiğinde farklı çalışan betiklerine ve 'klasik moda' otomatik olarak devretmesidir.

## Çalışan Modu

[Çalışan modunu](worker.md) etkinleştirmek performansı önemli ölçüde artırır, ancak uygulamanızın bu modla uyumlu olacak şekilde uyarlanması gerekir: bir çalışan betiği oluşturmanız ve uygulamanın bellek sızıntısı yapmadığından emin olmanız gerekir.

## musl Kullanmayın

Resmi Docker imajlarının Alpine Linux varyantı ve sağladığımız varsayılan ikili dosyalar [musl libc](https://musl.libc.org) kullanmaktadır.

PHP'nin, geleneksel GNU kütüphanesi yerine bu alternatif C kütüphanesini kullanırken [daha yavaş](https://gitlab.alpinelinux.org/alpine/aports/-/issues/14381) olduğu bilinmektedir, özellikle FrankenPHP için gerekli olan ZTS modunda (iş parçacığı güvenli) derlendiğinde. Fark, yoğun iş parçacıklı bir ortamda önemli olabilir.

Ayrıca, [bazı hatalar sadece musl kullanılırken](https://github.com/php/php-src/issues?q=sort%3Aupdated-desc+is%3Aissue+is%3Aopen+label%3ABug+musl) ortaya çıkar.

Üretim ortamlarında, glibc'ye bağlanmış, uygun bir optimizasyon seviyesiyle derlenmiş FrankenPHP kullanmanızı öneririz.

Bu, Debian Docker imajlarını kullanarak, sürdürücülerimizin [.deb](https://debs.henderkes.com) veya [.rpm](https://rpms.henderkes.com) paketlerini kullanarak veya [FrankenPHP'yi kaynak koddan derleyerek](compile.md) başarılabilir.

## Go Çalışma Zamanı Yapılandırması

FrankenPHP Go dilinde yazılmıştır.

Genel olarak, Go çalışma zamanı özel bir yapılandırma gerektirmez, ancak belirli durumlarda, özel yapılandırma performansı artırır.

`GODEBUG` ortam değişkenini `cgocheck=0` olarak ayarlamak isteyebilirsiniz (FrankenPHP Docker imajlarında varsayılan değerdir).

FrankenPHP'yi kapsayıcılarda (Docker, Kubernetes, LXC...) çalıştırıyorsanız ve kapsayıcılar için ayrılan belleği sınırlıyorsanız, `GOMEMLIMIT` ortam değişkenini kullanılabilir bellek miktarına ayarlayın.

Daha fazla ayrıntı için, çalışma zamanından en iyi şekilde yararlanmak için [bu konuya ayrılmış Go dokümantasyon sayfası](https://pkg.go.dev/runtime#hdr-Environment_Variables) mutlaka okunmalıdır.

## `file_server`

Varsayılan olarak, `php_server` direktifi, kök dizinde depolanan statik dosyaları (varlıkları) sunmak için otomatik olarak bir dosya sunucusu kurar.

Bu özellik kullanışlıdır, ancak bir maliyeti vardır. Devre dışı bırakmak için aşağıdaki yapılandırmayı kullanın:

```caddyfile
php_server {
    file_server off
}
```

## `try_files`

Statik dosyalar ve PHP dosyalarının yanı sıra, `php_server` uygulamanızın dizin ve dizin dizin dosyalarını (`/path/` -> `/path/index.php`) da sunmaya çalışacaktır. Dizin dizinlerine ihtiyacınız yoksa, `try_files`'ı aşağıdaki gibi açıkça tanımlayarak bunları devre dışı bırakabilirsiniz:

```caddyfile
php_server {
    try_files {path} index.php
    root /root/to/your/app # buraya kökü açıkça eklemek daha iyi önbelleğe alma sağlar
}
```

Bu, gereksiz dosya işlemlerinin sayısını önemli ölçüde azaltabilir.

0 gereksiz dosya sistemi işlemi ile alternatif bir yaklaşım, bunun yerine `php` direktifini kullanmak ve dosyaları PHP'den yola göre ayırmaktır. Bu yaklaşım, tüm uygulamanızın tek bir giriş dosyası tarafından sunulması durumunda iyi çalışır. `/assets` klasörünün arkasındaki statik dosyaları sunan örnek bir [yapılandırma](config.md#caddyfile-config) şöyle görünebilir:

```caddyfile
route {
    @assets {
        path /assets/*
    }

    # /assets arkasındaki her şey dosya sunucusu tarafından işlenir
    file_server @assets {
        root /root/to/your/app
    }

    # /assets içinde olmayan her şey index veya worker PHP dosyanız tarafından işlenir
    rewrite index.php
    php {
        root /root/to/your/app # buraya kökü açıkça eklemek daha iyi önbelleğe alma sağlar
    }
}
```

## Yer Tutucular

`root` ve `env` direktiflerinde [yer tutucular](https://caddyserver.com/docs/conventions#placeholders) kullanabilirsiniz. Ancak bu, bu değerlerin önbelleğe alınmasını engeller ve önemli bir performans maliyeti getirir.

Mümkünse, bu direktiflerde yer tutuculardan kaçının.

## `resolve_root_symlink`

Varsayılan olarak, belge kökü sembolik bir bağlantıysa, FrankenPHP tarafından otomatik olarak çözümlenir (bu, PHP'nin düzgün çalışması için gereklidir). Belge kökü bir sembolik bağlantı değilse, bu özelliği devre dışı bırakabilirsiniz.

```caddyfile
php_server {
    resolve_root_symlink false
}
```

Bu, `root` direktifi [yer tutucular](https://caddyserver.com/docs/conventions#placeholders) içeriyorsa performansı artıracaktır. Diğer durumlarda kazanç ihmal edilebilir olacaktır.

## Günlükler

Günlükleme açıkça çok faydalıdır, ancak tanımı gereği, önemli ölçüde performansı düşüren G/Ç işlemleri ve bellek tahsisleri gerektirir. [Günlükleme seviyesini](https://caddyserver.com/docs/caddyfile/options#log) doğru ayarladığınızdan ve yalnızca gerekli olanı günlüğe kaydettiğinizden emin olun.

## PHP Performansı

FrankenPHP resmi PHP yorumlayıcısını kullanır. Tüm olağan PHP ile ilgili performans optimizasyonları FrankenPHP ile uygulanır.

Özellikle:

- [OPcache](https://www.php.net/manual/en/book.opcache.php)'in kurulu, etkin ve doğru şekilde yapılandırılmış olduğundan emin olun
- [Composer otomatik yükleyici optimizasyonlarını](https://getcomposer.org/doc/articles/autoloader-optimization.md) etkinleştirin
- `realpath` önbelleğinin uygulamanızın ihtiyaçları için yeterince büyük olduğundan emin olun
- [ön yükleme (preloading)](https://www.php.net/manual/en/opcache.preloading.php) kullanın

Daha fazla ayrıntı için, [Symfony'nin bu konuya ayrılmış dokümantasyon girişini](https://symfony.com/doc/current/performance.html) okuyun (çoğu ipucu Symfony kullanmasanız bile faydalıdır).

## İş Parçacığı Havuzunu Ayırma

Uygulamaların, yüksek yük altında güvenilmez olma eğiliminde olan veya sürekli olarak 10 saniyeden fazla yanıt veren bir API gibi yavaş harici servislerle etkileşime girmesi yaygındır. Bu gibi durumlarda, iş parçacığı havuzunu özel "yavaş" havuzlara ayırmak faydalı olabilir. Bu, yavaş uç noktaların tüm sunucu kaynaklarını/iş parçacıklarını tüketmesini önler ve bir bağlantı havuzuna benzer şekilde, yavaş uç noktaya giden isteklerin eşzamanlılığını sınırlar.

```caddyfile
{
    frankenphp {
        max_threads 100 # tüm çalışanlar tarafından paylaşılan maksimum 100 iş parçacığı
    }
}

example.com {
    php_server {
        root /app/public # uygulamanızın kökü
        worker index.php {
            match /slow-endpoint/* # /slow-endpoint/* yoluyla gelen tüm istekler bu iş parçacığı havuzu tarafından işlenir
            num 10 # /slow-endpoint/* ile eşleşen istekler için minimum 10 iş parçacığı
        }
        worker index.php {
            match * # diğer tüm istekler ayrı ayrı işlenir
            num 20 # yavaş uç noktalar asılı kalmaya başlasa bile diğer istekler için minimum 20 iş parçacığı
        }
    }
}
```

Genellikle, çok yavaş uç noktaları, mesaj kuyrukları gibi ilgili mekanizmalar kullanarak eşzamansız olarak ele almak da tavsiye edilir.
