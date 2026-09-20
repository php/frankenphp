# Büyük statik dosyaları verimli sunma (`X-Sendfile`/`X-Accel-Redirect`)

Normalde statik dosyalar doğrudan web sunucusu tarafından sunulabilir,
ancak bazen göndermeden önce biraz PHP kodu çalıştırmak gerekir:
erişim denetimi, istatistikler, özel HTTP başlıkları...

Ne yazık ki büyük statik dosyaları PHP ile sunmak,
web sunucusunu doğrudan kullanmaya kıyasla verimsizdir (bellek aşırı yüklenmesi, düşen performans...).

FrankenPHP, özel PHP kodunu çalıştırdıktan **sonra**
statik dosyaların gönderimini web sunucusuna devretmenize izin verir.

Bunun için PHP uygulamanızın, sunulacak dosyanın yolunu içeren
özel bir HTTP başlığı tanımlaması yeterlidir. Gerisini FrankenPHP halleder.

Bu özellik Apache'de **`X-Sendfile`**, NGINX'te **`X-Accel-Redirect`** olarak bilinir.

Aşağıdaki örneklerde projenin belge kökünün `public/` dizini olduğunu
ve `public/` dışındaki, `private-files/` adlı bir dizinde saklanan dosyaları
PHP ile sunmak istediğimizi varsayıyoruz.

## FrankenPHP Caddyfile'ında X-Accel-Redirect yapılandırma

Önce bu özelliği etkinleştirmek için `Caddyfile`'ınıza şu yapılandırmayı ekleyin:

```patch
	root public/
	# ...

+	# Symfony, Laravel ve Symfony HttpFoundation bileşenini kullanan diğer projeler için gerekli
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
+			# Güvenliği artırmak için PHP tarafından ayarlanan X-Accel-Redirect başlığını kaldırın
+			header -X-Accel-Redirect
+
+			file_server
+		}
+	}

	php_server
```

## Düz PHP

`X-Accel-Redirect` başlığının değeri olarak göreli dosya yolunu (`private-files/` dizininden) ayarlayın:

```php
header('X-Accel-Redirect: file.txt');
```

## Symfony HttpFoundation bileşenini kullanan projeler (Symfony, Laravel, Drupal...)

Bu özelliği Symfony HttpFoundation ile kullanmak için ayrıntılara [Symfony belgelerinden](symfony.md#serving-large-static-files-x-sendfile) bakın.
