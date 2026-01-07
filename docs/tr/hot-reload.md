# Anında Yeniden Yükleme

FrankenPHP, geliştirici deneyimini büyük ölçüde iyileştirmek için tasarlanmış yerleşik bir **anında yeniden yükleme** (hot reload) özelliğine sahiptir.

![Mercure](hot-reload.png)

Bu özellik, modern JavaScript araçlarında (Vite veya webpack gibi) bulunan **Anında Modül Değişimi (HMR)**'ye benzer bir iş akışı sağlar.
Her dosya değişikliğinden sonra (PHP kodu, şablonlar, JavaScript ve CSS dosyaları...) tarayıcıyı manuel olarak yenilemek yerine,
FrankenPHP içeriği gerçek zamanlı olarak günceller.

Anında Yeniden Yükleme, WordPress, Laravel, Symfony ve diğer tüm PHP uygulamaları veya framework'leri ile yerel olarak çalışır.

Etkinleştirildiğinde, FrankenPHP dosya sistemi değişiklikleri için mevcut çalışma dizininizi izler.
Bir dosya değiştirildiğinde, tarayıcıya bir [Mercure](mercure.md) güncellemesi gönderir.

Kurulumunuza bağlı olarak, tarayıcı şunları yapar:

- [Idiomorph](https://github.com/bigskysoftware/idiomorph) yüklüyse **DOM'u biçimlendirir** (kaydırma konumunu ve giriş durumunu korur).
- Idiomorph mevcut değilse **sayfayı yeniden yükler** (standart canlı yeniden yükleme).

## Yapılandırma

Anında yeniden yüklemeyi etkinleştirmek için Mercure'ü etkinleştirin, ardından `Caddyfile` dosyanızdaki `php_server` yönergesine `hot_reload` alt yönergesini ekleyin.

> [!UYARI]
> Bu özellik **yalnızca geliştirme ortamları** için tasarlanmıştır.
> `hot_reload`'u üretimde etkinleştirmeyin, çünkü dosya sistemini izlemek performans düşüşüne neden olur ve dahili uç noktaları açığa çıkarır.

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

Glob sözdizimini kullanarak izlenecek dosyaları açıkça belirtmek mümkündür:

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

Mercure konusunu ve hangi dizin veya dosyaların izleneceğini belirtmek için `hot_reload` seçeneğine yollar sağlayarak uzun formu kullanın:

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
FrankenPHP, dosya değişikliklerine abone olmak için kullanılacak Mercure Hub URL'sini `$_SERVER['FRANKENPHP_HOT_RELOAD']` ortam değişkeni aracılığıyla açığa çıkarır.

İstemci tarafı mantığını işlemek için kullanışlı bir JavaScript kütüphanesi olan [frankenphp-hot-reload](https://www.npmjs.com/package/frankenphp-hot-reload) da mevcuttur.
Kullanmak için ana şablonunuza aşağıdakileri ekleyin:

```php
<!DOCTYPE html>
<title>FrankenPHP Anında Yeniden Yükleme</title>
<?php if (isset($_SERVER['FRANKENPHP_HOT_RELOAD'])): ?>
<meta name="frankenphp-hot-reload:url" content="<?=$_SERVER['FRANKENPHP_HOT_RELOAD']?>">
<script src="https://cdn.jsdelivr.net/npm/idiomorph"></script>
<script src="https://cdn.jsdelivr.net/npm/frankenphp-hot-reload/+esm" type="module"></script>
<?php endif ?>
```

Kütüphane, Mercure hub'ına otomatik olarak abone olacak, bir dosya değişikliği algılandığında arka planda mevcut URL'yi çekecek ve DOM'u biçimlendirecektir.
Bir [npm](https://www.npmjs.com/package/frankenphp-hot-reload) paketi olarak ve [GitHub](https://github.com/dunglas/frankenphp-hot-reload) üzerinde mevcuttur.

Alternatif olarak, `EventSource` yerel JavaScript sınıfını kullanarak doğrudan Mercure hub'ına abone olarak kendi istemci tarafı mantığınızı uygulayabilirsiniz.

### Çalışan Modu

Uygulamanızı [Çalışan Modunda](https://frankenphp.dev/docs/worker/) çalıştırıyorsanız, uygulama betiğiniz bellekte kalır.
Bu, tarayıcı yeniden yüklense bile PHP kodunuzdaki değişikliklerin hemen yansımayacağı anlamına gelir.

En iyi geliştirici deneyimi için, `hot_reload`'u [çalışan yönergesindeki `watch` alt yönergesi](config.md#watching-for-file-changes) ile birleştirmelisiniz.

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

### Nasıl Çalışır

1. **İzleme**: FrankenPHP, arka planda [e-dant/watcher kütüphanesini](https://github.com/e-dant/watcher) kullanarak dosya sistemindeki değişiklikleri izler (Go bağlamasını biz geliştirdik).
2. **Yeniden Başlatma (Çalışan Modu)**: Eğer çalışan yapılandırmasında `watch` etkinleştirilmişse, yeni kodu yüklemek için PHP çalışanı yeniden başlatılır.
3. **İtme**: Değişen dosyaların listesini içeren bir JSON yükü, yerleşik [Mercure hub](https://mercure.rocks)'ına gönderilir.
4. **Alma**: JavaScript kütüphanesi aracılığıyla dinleyen tarayıcı, Mercure olayını alır.
5. **Güncelleme**:

- Eğer **Idiomorph** algılanırsa, güncellenmiş içeriği çeker ve mevcut HTML'i yeni duruma uyması için biçimlendirir, durumu kaybetmeden değişiklikleri anında uygular.
- Aksi takdirde, sayfayı yenilemek için `window.location.reload()` çağrılır.
