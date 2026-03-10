# Konfigürasyon

FrankenPHP, Caddy'nin yanı sıra [Mercure](mercure.md) ve [Vulcain](https://vulcain.rocks) modülleri [Caddy tarafından desteklenen formatlar](https://caddyserver.com/docs/getting-started#your-first-config) kullanılarak yapılandırılabilir.

En yaygın format, basit, insan tarafından okunabilir bir metin formatı olan `Caddyfile`'dır. Varsayılan olarak, FrankenPHP mevcut dizinde bir `Caddyfile` arar. Özel bir yol belirtmek için `-c` veya `--config` seçeneğini kullanabilirsiniz.

Bir PHP uygulamasını sunmak için minimal bir `Caddyfile` aşağıda gösterilmiştir:

```caddyfile
# Yanıt verilecek ana bilgisayar adı
localhost

# İsteğe bağlı olarak, dosyaların sunulacağı dizin, aksi takdirde mevcut dizin varsayılan olarak kullanılır
#root public/
php_server
```

Daha fazla özellik sağlayan ve kullanışlı ortam değişkenleri sunan daha gelişmiş bir `Caddyfile`, [FrankenPHP deposunda](https://github.com/php/frankenphp/blob/main/caddy/frankenphp/Caddyfile) ve Docker imajlarıyla birlikte sağlanır.

PHP'nin kendisi [bir `php.ini` dosyası kullanılarak yapılandırılabilir](https://www.php.net/manual/en/configuration.file.php).

Kurulum yönteminize bağlı olarak, FrankenPHP ve PHP yorumlayıcısı, aşağıda açıklanan konumlardaki yapılandırma dosyalarını arayacaktır.

## Docker

FrankenPHP:

- `/etc/frankenphp/Caddyfile`: ana yapılandırma dosyası
- `/etc/frankenphp/Caddyfile.d/*.caddyfile`: otomatik olarak yüklenen ek yapılandırma dosyaları

PHP:

- `php.ini`: `/usr/local/etc/php/php.ini` (varsayılan olarak bir `php.ini` sağlanmaz)
- ek yapılandırma dosyaları: `/usr/local/etc/php/conf.d/*.ini`
- PHP uzantıları: `/usr/local/lib/php/extensions/no-debug-zts-<YYYYMMDD>/`
- PHP projesi tarafından sağlanan resmi bir şablonu kopyalamalısınız:

```dockerfile
FROM dunglas/frankenphp

# Production:
RUN cp $PHP_INI_DIR/php.ini-production $PHP_INI_DIR/php.ini

# Veya development:
RUN cp $PHP_INI_DIR/php.ini-development $PHP_INI_DIR/php.ini
```

## RPM ve Debian paketleri

FrankenPHP:

- `/etc/frankenphp/Caddyfile`: ana yapılandırma dosyası
- `/etc/frankenphp/Caddyfile.d/*.caddyfile`: otomatik olarak yüklenen ek yapılandırma dosyaları

PHP:

- `php.ini`: `/etc/php-zts/php.ini` (varsayılan olarak üretim ön ayarlarına sahip bir `php.ini` dosyası sağlanır)
- ek yapılandırma dosyaları: `/etc/php-zts/conf.d/*.ini`

## Statik ikili

FrankenPHP:

- Mevcut çalışma dizininde: `Caddyfile`

PHP:

- `php.ini`: `frankenphp run` veya `frankenphp php-server` komutunun çalıştırıldığı dizin, ardından `/etc/frankenphp/php.ini`
- ek yapılandırma dosyaları: `/etc/frankenphp/php.d/*.ini`
- PHP uzantıları: yüklenemez, bunları doğrudan ikili dosyaya dahil edin
- [PHP kaynak kodu](https://github.com/php/php-src/) ile birlikte verilen `php.ini-production` veya `php.ini-development` dosyalarından birini kopyalayın.

## Caddyfile Konfigürasyonu

PHP uygulamanızı sunmak için site blokları içinde `php_server` veya `php` [HTTP yönergeleri](https://caddyserver.com/docs/caddyfile/concepts#directives) kullanılabilir.

Minimal örnek:

```caddyfile
localhost {
	# Sıkıştırmayı etkinleştir (isteğe bağlı)
	encode zstd br gzip
	# Geçerli dizindeki PHP dosyalarını çalıştırın ve varlıkları sunun
	php_server
}
```

Alternatif olarak, FrankenPHP'yi [global seçenek](https://caddyserver.com/docs/caddyfile/concepts#global-options) olan `frankenphp` kullanarak açıkça yapılandırabilirsiniz:

```caddyfile
{
	frankenphp {
		num_threads <num_threads> # Başlatılacak PHP iş parçacığı sayısını ayarlar. Varsayılan: Mevcut CPU'ların 2 katı.
		max_threads <num_threads> # Çalışma zamanında başlatılabilecek ek PHP iş parçacığı sayısını sınırlar. Varsayılan: num_threads. 'auto' olarak ayarlanabilir.
		max_wait_time <duration> # Bir isteğin boş bir PHP iş parçacığı bekleme süresinin zaman aşımına uğramadan önceki maksimum süresini ayarlar. Varsayılan: devre dışı.
		max_idle_time <duration> # Otomatik ölçeklenen bir iş parçacığının devre dışı bırakılmadan önce ne kadar süre boş kalabileceğini ayarlar. Varsayılan: 5s.
		php_ini <key> <value> # Bir php.ini yönergesi ayarlar. Birden fazla yönerge ayarlamak için birden fazla kez kullanılabilir.
		worker {
			file <path> # Çalışan komut dosyasının yolunu ayarlar.
			num <num> # Başlatılacak PHP iş parçacığı sayısını ayarlar, varsayılan değer mevcut CPU'ların 2 katıdır.
			env <key> <value> # Ek bir ortam değişkenini verilen değere ayarlar. Birden fazla ortam değişkeni için birden fazla kez belirtilebilir.
			watch <path> # Dosya değişikliklerini izlemek için yolu ayarlar. Birden fazla yol için birden fazla kez belirtilebilir.
			name <name> # İşçinin adını ayarlar, günlüklerde ve metriklerde kullanılır. Varsayılan: işçi dosyasının mutlak yolu
			max_consecutive_failures <num> # İşçinin sağlıksız kabul edilmeden önceki maksimum ardışık hata sayısını ayarlar, -1 işçinin her zaman yeniden başlayacağı anlamına gelir. Varsayılan: 6.
		}
	}
}

# ...
```

Alternatif olarak, `worker` seçeneğinin tek satırlık kısa formunu kullanabilirsiniz:

```caddyfile
{
	frankenphp {
		worker <file> <num>
	}
}

# ...
```

Aynı sunucuda birden fazla uygulamaya hizmet veriyorsanız birden fazla işçi de tanımlayabilirsiniz:

```caddyfile
app.example.com {
    root /path/to/app/public
	php_server {
		root /path/to/app/public # daha iyi önbelleğe almayı sağlar
		worker index.php <num>
	}
}

other.example.com {
    root /path/to/other/public
	php_server {
		root /path/to/other/public
		worker index.php <num>
	}
}

# ...
```

Genellikle ihtiyacınız olan şey `php_server` yönergesini kullanmaktır,
ancak tam kontrole ihtiyacınız varsa, daha düşük seviyeli `php` yönergesini kullanabilirsiniz.
`php` yönergesi, önce bir PHP dosyası olup olmadığını kontrol etmek yerine tüm girdiyi PHP'ye iletir. Daha fazla bilgiyi [performans sayfasında](performance.md#try_files) okuyun.

`php_server` yönergesini kullanmak bu yapılandırmayla aynıdır:

```caddyfile
route {
	# Dizin istekleri için sondaki eğik çizgiyi ekleyin
	@canonicalPath {
		file {path}/index.php
		not path */
	}
	redir @canonicalPath {path}/ 308
	# İstenen dosya mevcut değilse, dizin dosyalarını deneyin
	@indexFiles file {
		try_files {path} {path}/index.php index.php
		split_path .php
	}
	rewrite @indexFiles {http.matchers.file.relative}
	# FrankenPHP!
	@phpFiles path *.php
	php @phpFiles
	file_server
}
```

`php_server` ve `php` yönergeleri aşağıdaki seçeneklere sahiptir:

```caddyfile
php_server [<matcher>] {
	root <directory> # Sitenin kök klasörünü ayarlar. Varsayılan: `root` yönergesi.
	split_path <delim...> # URI'yi iki parçaya bölmek için alt dizgeleri ayarlar. İlk eşleşen alt dizge "yol bilgisini" yoldan ayırmak için kullanılır. İlk parça eşleşen alt dizeyle sonlandırılır ve gerçek kaynak (CGI betiği) adı olarak kabul edilir. İkinci parça betiğin kullanması için PATH_INFO olarak ayarlanacaktır. Varsayılan: `.php`
	resolve_root_symlink false # Varsa, sembolik bir bağlantıyı değerlendirerek `root` dizininin gerçek değerine çözümlenmesini devre dışı bırakır (varsayılan olarak etkindir).
	env <key> <value> # Ek bir ortam değişkenini verilen değere ayarlar. Birden fazla ortam değişkeni için birden fazla kez belirtilebilir.
	file_server off # Yerleşik file_server yönergesini devre dışı bırakır.
	worker { # Bu sunucuya özgü bir worker oluşturur. Birden fazla worker için birden fazla kez belirtilebilir.
		file <path> # Worker betiğinin yolunu ayarlar, php_server köküne göre göreceli olabilir
		num <num> # Başlatılacak PHP iş parçacığı sayısını ayarlar, varsayılan değer mevcut CPU'ların 2 katıdır
		name <name> # Worker için günlüklerde ve metriklerde kullanılan bir ad ayarlar. Varsayılan: worker dosyasının mutlak yolu. Bir php_server bloğunda tanımlandığında her zaman m# ile başlar.
		watch <path> # Dosya değişikliklerini izlemek için yolu ayarlar. Birden fazla yol için birden fazla kez belirtilebilir.
		env <key> <value> # Ek bir ortam değişkenini verilen değere ayarlar. Birden fazla ortam değişkeni için birden fazla kez belirtilebilir. Bu worker için ortam değişkenleri ayrıca php_server üst öğesinden devralınır, ancak burada geçersiz kılınabilir.
		match <path> # İşçiyi bir yol desenine eşleştirir. try_files'ı geçersiz kılar ve yalnızca php_server yönergesinde kullanılabilir.
	}
	worker <other_file> <num> # Global frankenphp bloğundaki gibi kısa formu da kullanabilirsiniz.
}
```

### Dosya Değişikliklerini İzleme

Workers yalnızca uygulamanızı bir kez başlatır ve bellekte tutar, bu nedenle PHP dosyalarınızdaki herhangi bir değişiklik hemen yansımaz.

Bunun yerine işçiler, `watch` yönergesi aracılığıyla dosya değişikliklerinde yeniden başlatılabilir. Bu, geliştirme ortamları için kullanışlıdır.

```caddyfile
{
	frankenphp {
		worker {
			file  /path/to/app/public/worker.php
			watch
		}
	}
}
```

Bu özellik genellikle [hot reload](hot-reload.md) ile birlikte kullanılır.

Eğer `watch` dizini belirtilmezse, FrankenPHP sürecinin başlatıldığı dizin ve alt dizinlerdeki tüm `.env`, `.php`, `.twig`, `.yaml` ve `.yml` dosyalarını izleyen `./**/*.{env,php,twig,yaml,yml}` değerine geri döner. Bunun yerine, bir veya daha fazla dizini [kabuk dosya adı deseni](https://pkg.go.dev/path/filepath#Match) aracılığıyla da belirtebilirsiniz:

```caddyfile
{
	frankenphp {
		worker {
			file  /path/to/app/public/worker.php
			watch /path/to/app # /path/to/app'ın tüm alt dizinlerindeki tüm dosyaları izler
			watch /path/to/app/*.php # /path/to/app'da .php ile biten dosyaları izler
			watch /path/to/app/**/*.php # /path/to/app ve alt dizinlerindeki PHP dosyalarını izler
			watch /path/to/app/**/*.{php,twig} # /path/to/app ve alt dizinlerindeki PHP ve Twig dosyalarını izler
		}
	}
}
```

- `**` deseni, özyinelemeli izlemeyi belirtir
- Dizinler göreceli de olabilir (FrankenPHP sürecinin başlatıldığı yere göre)
- Birden fazla işçi tanımladıysanız, bir dosya değiştiğinde hepsi yeniden başlatılacaktır.
- Çalışma zamanında oluşturulan dosyaları (günlükler gibi) izlerken dikkatli olun, zira bunlar istenmeyen işçi yeniden başlatmalarına neden olabilir.

Dosya izleyici [e-dant/watcher](https://github.com/e-dant/watcher) üzerine kuruludur.

## İşçiyi Yola Eşleştirme

Geleneksel PHP uygulamalarında, betikler her zaman public dizininde bulunur. Bu, diğer tüm PHP betikleri gibi ele alınan işçi betikleri için de geçerlidir. İşçi betiğini public dizininin dışına koymak isterseniz, bunu `match` yönergesi aracılığıyla yapabilirsiniz.

`match` yönergesi, yalnızca `php_server` ve `php` içinde bulunan `try_files`'a optimize edilmiş bir alternatiftir. Aşağıdaki örnek, varsa her zaman public dizinindeki bir dosyayı sunacak, aksi takdirde isteği yol desenine uyan işçiye iletecektir.

```caddyfile
{
	frankenphp {
		php_server {
			worker {
				file /path/to/worker.php # dosya public yolunun dışında olabilir
				match /api/* # /api/ ile başlayan tüm istekler bu worker tarafından işlenecektir
			}
		}
	}
}
```

## Ortam Değişkenleri

Aşağıdaki ortam değişkenleri `Caddyfile` içinde değişiklik yapmadan Caddy yönergelerini entegre etmek için kullanılabilir:

- `SERVER_NAME`: [dinlenecek adresleri](https://caddyserver.com/docs/caddyfile/concepts#addresses) değiştirir, sağlanan ana bilgisayar adları oluşturulan TLS sertifikası için de kullanılacaktır
- `SERVER_ROOT`: sitenin kök dizinini değiştirir, varsayılan olarak `public/`dir
- `CADDY_GLOBAL_OPTIONS`: [global seçenekleri](https://caddyserver.com/docs/caddyfile/options) entegre eder
- `FRANKENPHP_CONFIG`: `frankenphp` yönergesi altına yapılandırma entegre eder

FPM ve CLI SAPI'lerinde olduğu gibi, ortam değişkenleri varsayılan olarak `$_SERVER` süper globalinde gösterilir.

[`variables_order`'a ait PHP yönergesinin](https://www.php.net/manual/en/ini.core.php#ini.variables-order) `S` değeri bu yönergede `E`'nin başka bir yere yerleştirilmesinden bağımsız olarak her zaman `ES` ile eş değerdir.

## PHP konfigürasyonu

Ek olarak [PHP yapılandırma dosyalarını](https://www.php.net/manual/en/configuration.file.php#configuration.file.scan) yüklemek için,
`PHP_INI_SCAN_DIR` ortam değişkeni kullanılabilir.
Ayarlandığında, PHP verilen dizinlerde bulunan `.ini` uzantılı tüm dosyaları yükleyecektir.

PHP yapılandırmasını `Caddyfile` içindeki `php_ini` yönergesini kullanarak da değiştirebilirsiniz:

```caddyfile
{
    frankenphp {
        php_ini memory_limit 256M

        # veya

        php_ini {
            memory_limit 256M
            max_execution_time 15
        }
    }
}
```

### HTTPS'i Devre Dışı Bırakma

Varsayılan olarak, FrankenPHP tüm ana bilgisayar adları için (localhost dahil) HTTPS'i otomatik olarak etkinleştirir. HTTPS'i devre dışı bırakmak isterseniz (örneğin bir geliştirme ortamında), `SERVER_NAME` ortam değişkenini `http://` veya `:80` olarak ayarlayabilirsiniz:

Alternatif olarak, [Caddy belgelerinde](https://caddyserver.com/docs/automatic-https#activation) açıklanan diğer tüm yöntemleri kullanabilirsiniz.

Eğer `localhost` ana bilgisayar adı yerine `127.0.0.1` IP adresiyle HTTPS kullanmak isterseniz, lütfen [bilinen sorunlar](known-issues.md#using-https127001-with-docker) bölümünü okuyun.

### Tam Çift Yönlü (HTTP/1)

HTTP/1.x kullanırken, tüm gövde okunmadan önce bir yanıt yazmaya izin vermek için tam çift yönlü modun etkinleştirilmesi istenebilir. (örneğin: [Mercure](mercure.md), WebSocket, Sunucu Tarafından Gönderilen Olaylar vb.)

Bu, `Caddyfile`'daki global seçeneklere eklenmesi gereken isteğe bağlı bir yapılandırmadır:

```caddyfile
{
  servers {
    enable_full_duplex
  }
}
```

> [!CAUTION]
>
> Bu seçeneği etkinleştirmek, tam çift yönlü desteği olmayan eski HTTP/1.x istemcilerinin kilitlenmesine neden olabilir.
> Bu, `CADDY_GLOBAL_OPTIONS` ortam yapılandırması kullanılarak da yapılandırılabilir:

```sh
CADDY_GLOBAL_OPTIONS="servers {
  enable_full_duplex
}"
```

Bu ayar hakkında daha fazla bilgiyi [Caddy belgelerinde](https://caddyserver.com/docs/caddyfile/options#enable-full-duplex) bulabilirsiniz.

## Hata Ayıklama Modunu Etkinleştirin

Docker imajını kullanırken, hata ayıklama modunu etkinleştirmek için `CADDY_GLOBAL_OPTIONS` ortam değişkenini `debug` olarak ayarlayın:

```console
docker run -v $PWD:/app/public \
    -e CADDY_GLOBAL_OPTIONS=debug \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
