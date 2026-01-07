# Büyük Statik Dosyaları Verimli Bir Şekilde Sunma (X-Sendfile/X-Accel-Redirect)

Genellikle statik dosyalar doğrudan web sunucusu tarafından sunulabilir,
ancak bazen bunları göndermeden önce bazı PHP kodları çalıştırmak gerekebilir:
erişim kontrolü, istatistikler, özel HTTP başlıkları...

Maalesef, büyük statik dosyaları sunmak için PHP kullanmak,
web sunucusunu doğrudan kullanmaya kıyasla verimsizdir (bellek aşırı yüklenmesi, performans düşüşü...).

FrankenPHP, özelleştirilmiş PHP kodu çalıştırıldıktan **sonra**
statik dosyaların web sunucusuna gönderilmesini devretmenizi sağlar.

Bunu yapmak için, PHP uygulamanızın sunulacak dosyanın yolunu içeren özel bir HTTP başlığı tanımlaması yeterlidir. Gerisini FrankenPHP halleder.

Bu özellik Apache için **`X-Sendfile`**, NGINX için ise **`X-Accel-Redirect`** olarak bilinir.

Aşağıdaki örneklerde, projenin belge kökünün `public/` dizini olduğunu
ve `public/` dizininin dışında, `private-files/` adlı bir dizinde depolanan dosyaları sunmak için PHP kullanmak istediğimizi varsayıyoruz.

## Yapılandırma

İlk olarak, bu özelliği etkinleştirmek için `Caddyfile` dosyanıza aşağıdaki yapılandırmayı ekleyin:

```patch
	root public/
	# ...

+	# Symfony, Laravel ve Symfony HttpFoundation bileşenini kullanan diğer projeler için gereklidir
+	request_header X-Sendfile-Type x-accel-redirect
+	request_header X-Accel-Mapping ../private-files=/private-files
+
+	intercept {
+		@accel header X-Accel-Redirect *
+		handle_response @accel {
+			root private-files/
+			rewrite * {resp.header.X-Accel-Redirect}
+			method * GET
+
+			# Artırılmış güvenlik için PHP tarafından ayarlanan X-Accel-Redirect başlığını kaldırın
+			header -X-Accel-Redirect
+
+			file_server
+		}
+	}

	php_server
```

## Yalın PHP

Göreceli dosya yolunu (`private-files/` dizininden) `X-Accel-Redirect` başlığının değeri olarak ayarlayın:

```php
header('X-Accel-Redirect: file.txt');
```

## Symfony HttpFoundation bileşenini kullanan projeler (Symfony, Laravel, Drupal...)

Symfony HttpFoundation [bu özelliği yerel olarak destekler](https://symfony.com/doc/current/components/http_foundation.html#serving-files).
Bu, `X-Accel-Redirect` başlığı için doğru değeri otomatik olarak belirleyecek ve yanıta ekleyecektir.

```php
use Symfony\Component\HttpFoundation\BinaryFileResponse;

BinaryFileResponse::trustXSendfileTypeHeader();
$response = new BinaryFileResponse(__DIR__.'/../private-files/file.txt');

// ...
```
