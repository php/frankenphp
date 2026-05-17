# Sıcak Yeniden Yükleme

FrankenPHP, geliştirici deneyimini büyük ölçüde iyileştirmek için tasarlanmış yerleşik bir **sıcak yeniden yükleme** özelliğine sahiptir.

![Hot Reload](hot-reload.png)

Bu özellik, Vite veya webpack gibi modern JavaScript araçlarındaki **Sıcak Modül Değişimi (HMR)** ile benzer bir iş akışı sunar.
Her dosya değişikliğinden (PHP kodu, şablonlar, JavaScript ve CSS dosyaları...) sonra tarayıcıyı manuel olarak yenilemek yerine,
FrankenPHP sayfa içeriğini gerçek zamanlı olarak günceller.

Sıcak Yeniden Yükleme, WordPress, Laravel, Symfony ve diğer tüm PHP uygulamaları veya framework'leri ile yerel olarak çalışır.

Etkinleştirildiğinde, FrankenPHP dosya sistemi değişiklikleri için mevcut çalışma dizininizi izler.
Bir dosya değiştirildiğinde, tarayıcıya bir [Mercure](mercure.md) güncellemesi gönderir.

Kurulumunuza bağlı olarak, tarayıcı ya:

- [Idiomorph](https://github.com/bigskysoftware/idiomorph) yüklüyse **DOM'u dönüştürür** (kaydırma konumunu ve girdi durumunu koruyarak).
- Idiomorph mevcut değilse **sayfayı yeniden yükler** (standart canlı yeniden yükleme).

## Yapılandırma

Sıcak yeniden yüklemeyi etkinleştirmek için Mercure'ü etkinleştirin, ardından `Caddyfile` dosyanızdaki `php_server` yönergesine `hot_reload` alt yönergesini ekleyin.

> [!WARNING]
>
> Bu özellik **yalnızca geliştirme ortamları** içindir.
> `hot_reload`'u üretimde etkinleştirmeyin, zira bu özellik güvenli değildir (hassas dahili ayrıntıları açığa çıkarır) ve uygulamanın yavaşlamasına neden olur.

```caddyfile
localhost

mercure {
    anonymous
}

root public/
php_server {
    hot_reload
}
```

Varsayılan olarak, FrankenPHP mevcut çalışma dizinindeki şu glob desenine uyan tüm dosyaları izleyecektir: `./**/*.{css,env,gif,htm,html,jpg,jpeg,js,mjs,php,png,svg,twig,webp,xml,yaml,yml}`

İzlenecek dosyaları glob sözdizimi kullanarak açıkça ayarlamak mümkündür:

```caddyfile
localhost

mercure {
    anonymous
}

root public/
php_server {
    hot_reload src/**/*{.php,.js} config/**/*.yaml
}
```

Kullanılacak Mercure konusunu ve izlenecek dizin veya dosyaları belirtmek için `hot_reload`'un uzun biçimini kullanın:

```caddyfile
localhost

mercure {
    anonymous
}

root public/
php_server {
    hot_reload {
        topic hot-reload-topic
        watch src/**/*.php
        watch assets/**/*.{ts,json}
        watch templates/
        watch public/css/
    }
}
```

## İstemci Tarafı Entegrasyonu

Sunucu değişiklikleri algılarken, tarayıcının sayfayı güncellemek için bu olaylara abone olması gerekir.
FrankenPHP, dosya değişikliklerine abone olmak için kullanılacak Mercure Hub URL'sini `$_SERVER['FRANKENPHP_HOT_RELOAD']` ortam değişkeni aracılığıyla gösterir.

İstemci tarafı mantığını yönetmek için kullanışlı bir JavaScript kütüphanesi olan [frankenphp-hot-reload](https://www.npmjs.com/package/frankenphp-hot-reload) da mevcuttur.
Kullanmak için ana düzeninize aşağıdakileri ekleyin:

```php
<!DOCTYPE html>
<title>FrankenPHP Hot Reload</title>
<?php if (isset($_SERVER['FRANKENPHP_HOT_RELOAD'])): ?>
<meta name="frankenphp-hot-reload:url" content="<?=$_SERVER['FRANKENPHP_HOT_RELOAD']?>">
<script src="https://cdn.jsdelivr.net/npm/idiomorph"></script>
<script src="https://cdn.jsdelivr.net/npm/frankenphp-hot-reload/+esm" type="module"></script>
<?php endif ?>
```

Kütüphane otomatik olarak Mercure hub'ına abone olacak, bir dosya değişikliği algılandığında arka planda mevcut URL'yi getirecek ve DOM'u dönüştürecektir.
Bir [npm](https://www.npmjs.com/package/frankenphp-hot-reload) paketi olarak ve [GitHub](https://github.com/dunglas/frankenphp-hot-reload) üzerinden edinilebilir.

Alternatif olarak, `EventSource` yerel JavaScript sınıfını kullanarak doğrudan Mercure hub'ına abone olarak kendi istemci tarafı mantığınızı uygulayabilirsiniz.

### Mevcut DOM Düğümlerini Korumak

Nadir durumlarda, [Symfony web hata ayıklama araç çubuğu gibi](https://github.com/symfony/symfony/pull/62970) geliştirme araçları kullanırken olduğu gibi,
belirli DOM düğümlerini korumak isteyebilirsiniz.
Bunu yapmak için, ilgili HTML öğesine `data-frankenphp-hot-reload-preserve` özniteliğini ekleyin:

```html
<div data-frankenphp-hot-reload-preserve><!-- Hata ayıklama çubuğum --></div>
```

## Çalışan Modu

Uygulamanızı [Çalışan Modunda](https://frankenphp.dev/docs/worker/) çalıştırıyorsanız, uygulama betiğiniz bellekte kalır.
Bu, tarayıcı yeniden yüklense bile PHP kodunuzdaki değişikliklerin hemen yansımayacağı anlamına gelir.

En iyi geliştirici deneyimi için `hot_reload`'u [worker yönergesindeki `watch` alt yönergesiyle](config.md#watching-for-file-changes) birleştirmelisiniz.

- `hot_reload`: dosyalar değiştiğinde **tarayıcıyı** yeniler
- `worker.watch`: dosyalar değiştiğinde çalışanı yeniden başlatır

```caddy
localhost

mercure {
    anonymous
}

root public/
php_server {
    hot_reload
    worker {
        file /path/to/my_worker.php
        watch
    }
}
```

## Nasıl Çalışır

1. **İzleme**: FrankenPHP, arka planda [`e-dant/watcher` kütüphanesini](https://github.com/e-dant/watcher) kullanarak (Go bağlayıcısını biz geliştirdik) dosya sistemindeki değişiklikleri izler.
2. **Yeniden Başlatma (Çalışan Modu)**: Çalışan yapılandırmasında `watch` etkinse, yeni kodu yüklemek için PHP çalışanı yeniden başlatılır.
3. **Gönderme**: Değiştirilen dosyaların listesini içeren bir JSON yükü, yerleşik [Mercure hub'ına](https://mercure.rocks) gönderilir.
4. **Alma**: JavaScript kütüphanesi aracılığıyla dinleyen tarayıcı, Mercure olayını alır.
5. **Güncelleme**:

- **Idiomorph** algılanırsa, güncellenmiş içeriği getirir ve mevcut HTML'i yeni duruma uydurmak için dönüştürerek, durum kaybetmeden değişiklikleri anında uygular.
- Aksi takdirde, sayfayı yenilemek için `window.location.reload()` çağrılır.
