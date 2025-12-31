# Konfigürasyon

FrankenPHP, Caddy'nin yanı sıra Mercure ve Vulcain modülleri [Caddy tarafından desteklenen formatlar](https://caddyserver.com/docs/getting-started#your-first-config) kullanılarak yapılandırılabilir.

Docker imajlarında] (docker.md), `Caddyfile` `/etc/frankenphp/Caddyfile` adresinde bulunur.
Statik ikili, başlatıldığı dizinde `Caddyfile` dosyasını arayacaktır.

PHP'nin kendisi [bir `php.ini` dosyası kullanılarak yapılandırılabilir](https://www.php.net/manual/tr/configuration.file.php).

PHP yorumlayıcısı aşağıdaki konumlarda arama yapacaktır:

Docker:

- php.ini: `/usr/local/etc/php/php.ini` Varsayılan olarak php.ini sağlanmaz.
- ek yapılandırma dosyaları: `/usr/local/etc/php/conf.d/*.ini`
- php uzantıları: `/usr/local/lib/php/extensions/no-debug-zts-<YYYYMMDD>/`
- PHP projesi tarafından sağlanan resmi bir şablonu kopyalamalısınız:

```dockerfile
FROM dunglas/frankenphp

# Developement:
RUN cp $PHP_INI_DIR/php.ini-development $PHP_INI_DIR/php.ini

# Veya production:
RUN cp $PHP_INI_DIR/php.ini-production $PHP_INI_DIR/php.ini
```

FrankenPHP kurulumu (.rpm veya .deb):

- php.ini: `/etc/frankenphp/php.ini` Varsayılan olarak üretim ön ayarlarına sahip bir php.ini dosyası sağlanır.
- ek yapılandırma dosyaları: `/etc/frankenphp/php.d/*.ini`
- php uzantıları: `/usr/lib/frankenphp/modules/`

Statik ikili:

- php.ini: `frankenphp run` veya `frankenphp php-server` komutunun çalıştırıldığı dizin, ardından `/etc/frankenphp/php.ini`
- ek yapılandırma dosyaları: `/etc/frankenphp/php.d/*.ini`
- php uzantıları: yüklenemez
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

FrankenPHP'yi global seçenek kullanarak açıkça yapılandırabilirsiniz:
`frankenphp` [global seçenek](https://caddyserver.com/docs/caddyfile/concepts#global-options) FrankenPHP'yi yapılandırmak için kullanılabilir.

```caddyfile
{
	frankenphp {
		num_threads <num_threads> # Başlatılacak PHP iş parçacığı sayısını ayarlar. Varsayılan: Mevcut CPU çekirdek sayısının 2 katı.
		worker {
			file <path> # Çalışan komut dosyasının yolunu ayarlar.
			num <num> # Başlatılacak PHP iş parçacığı sayısını ayarlar, varsayılan değer mevcut CPU çekirdek sayısının 2 katıdır.
			env <key> <value> # Ek bir ortam değişkenini verilen değere ayarlar. Birden fazla ortam değişkeni için birden fazla kez belirtilebilir.
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
ancak tam kontrole ihtiyacınız varsa, daha düşük seviyeli `php` yönergesini kullanabilirsiniz:

php_server` yönergesini kullanmak bu yapılandırmay ile aynıdır:

```caddyfile
route {
	# Dizin istekleri için sondaki eğik çizgiyi, diğer adıyla taksim işaretini ekleyin
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

php_server`ve`php` yönergeleri aşağıdaki seçeneklere sahiptir:

```caddyfile
php_server [<matcher>] {
	root <directory> # Sitenin kök klasörünü ayarlar. Öntanımlı: `root` yönergesi.
	split_path <delim...> # URI'yi iki parçaya bölmek için alt dizgeleri ayarlar. İlk eşleşen alt dizge "yol bilgisini" yoldan ayırmak için kullanılır. İlk parça eşleşen alt dizeyle sonlandırılır ve gerçek kaynak (CGI betiği) adı olarak kabul edilir. İkinci parça betiğin kullanması için PATH_INFO olarak ayarlanacaktır. Varsayılan: `.php`
	resolve_root_symlink false # Varsa, sembolik bir bağlantıyı değerlendirerek `root` dizininin gerçek değerine çözümlenmesini devre dışı bırakır (varsayılan olarak etkindir).
	env <key> <value> # Ek bir ortam değişkenini verilen değere ayarlar. Birden fazla ortam değişkeni için birden fazla kez belirtilebilir.
	file_server off # Yerleşik file_server yönergesini devre dışı bırakır.
	worker { # Bu sunucuya özgü bir worker oluşturur. Birden fazla worker için birden fazla kez belirtilebilir.
		file <path> # Worker betiğinin yolunu ayarlar, php_server köküne göre göreceli olabilir
		num <num> # Başlatılacak PHP iş parçacığı sayısını ayarlar, varsayılan değer mevcut CPU çekirdek sayısının 2 katıdır
		name <name> # Worker için günlüklerde ve metriklerde kullanılan bir ad ayarlar. Varsayılan: worker dosyasının mutlak yolu. Bir php_server bloğunda tanımlandığında her zaman m# ile başlar.
		watch <path> # Dosya değişikliklerini izlemek için yolu ayarlar. Birden fazla yol için birden fazla kez belirtilebilir.
		env <key> <value> # Ek bir ortam değişkenini verilen değere ayarlar. Birden fazla ortam değişkeni için birden fazla kez belirtilebilir. Bu worker için ortam değişkenleri ayrıca php_server üst öğesinden devralınır, ancak burada geçersiz kılınabilir.
	}
	worker <other_file> <num> # Global frankenphp bloğundaki gibi kısa formu da kullanabilirsiniz.
}
```

## Ortam Değişkenleri

Aşağıdaki ortam değişkenleri `Caddyfile` içinde değişiklik yapmadan Caddy yönergelerini entegre etmek için kullanılabilir:

- `SERVER_NAME`: değiştirin [dinlenecek adresleri](https://caddyserver.com/docs/caddyfile/concepts#addresses), sağlanan ana bilgisayar adları oluşturulan TLS sertifikası için de kullanılacaktır
- `CADDY_GLOBAL_OPTIONS`: entegre edin [global seçenekler](https://caddyserver.com/docs/caddyfile/options)
- `FRANKENPHP_CONFIG`: `frankenphp` yönergesi altına yapılandırma entegre edin

FPM ve CLI SAPI'lerinde olduğu gibi, ortam değişkenleri varsayılan olarak `$_SERVER` süper globalinde gösterilir.

[`variables_order`'a ait PHP yönergesinin](https://www.php.net/manual/en/ini.core.php#ini.variables-order) `S` değeri bu yönergede `E`'nin başka bir yere yerleştirilmesinden bağımsız olarak her zaman `ES` ile eş değerdir.

## PHP konfigürasyonu

Ek olarak [PHP yapılandırma dosyalarını](https://www.php.net/manual/en/configuration.file.php#configuration.file.scan) yüklemek için
`PHP_INI_SCAN_DIR` ortam değişkeni kullanılabilir.
Ayarlandığında, PHP verilen dizinlerde bulunan `.ini` uzantılı tüm dosyaları yükleyecektir.

## Hata Ayıklama Modunu Etkinleştirin

Docker imajını kullanırken, hata ayıklama modunu etkinleştirmek için `CADDY_GLOBAL_OPTIONS` ortam değişkenini `debug` olarak ayarlayın:

```console
docker run -v $PWD:/app/public \
    -e CADDY_GLOBAL_OPTIONS=debug \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```
