---
title: Yii3'ü FrankenPHP ile Çalıştırma (Docker, worker modu)
description: Bir Yii3 uygulamasını FrankenPHP ile Docker imajı, yerel kurulum veya yii-runner-frankenphp paketi ile worker modu kullanarak nasıl çalıştıracağınız.
---

# Yii3

## Yii3'ü FrankenPHP Docker İmajıyla Çalıştırma

Bir [Yii](https://www.yiiframework.com/) web uygulamasını FrankenPHP ile sunmak, projeyi resmi Docker imajının `/app` dizinine bağlamak kadar kolaydır.

Bu komutu Yii uygulamanızın ana dizininden çalıştırın:

```console
docker run -p 80:80 -p 443:443 -p 443:443/udp -v $PWD:/app dunglas/frankenphp
```

Ve tadını çıkarın!

## Yii3'ü FrankenPHP ile Yerel Kurma

Alternatif olarak, Yii projelerinizi FrankenPHP ile yerel makinenizden çalıştırabilirsiniz:

1. [Sisteminize karşılık gelen ikili dosyayı indirin](../#standalone-binary)
2. Aşağıdaki yapılandırmayı Yii projenizin kök dizinindeki `Caddyfile` adlı bir dosyaya ekleyin:

   ```caddyfile
   {
   	frankenphp
   }

   # Sunucunuzun alan adı
   localhost {
   	# Sıkıştırmayı etkinleştir (isteğe bağlı)
   	encode zstd br gzip
   	# public/ dizininden PHP dosyalarını çalıştırın ve varlıkları sunun
   	php_server {
   		root public/
   		try_files {path} index.php
   	}
   }
   ```

3. FrankenPHP'yi Yii projenizin kök dizininden başlatın: `frankenphp run`

## Yii3 Worker Modu

Yii uygulamanızı [worker modunda](worker.md) çalıştırmak için [Yii FrankenPHP çalıştırıcısı](https://github.com/yiisoft/yii-runner-frankenphp) paketini kurun:

```console
composer require yiisoft/yii-runner-frankenphp
```

Ardından, uygulamanızın kök dizininde `worker.php` adlı bir dosya oluşturun:

```php
<?php

declare(strict_types=1);

use Yiisoft\Yii\Runner\FrankenPHP\FrankenPHPApplicationRunner;

require_once __DIR__ . '/vendor/autoload.php';

(new FrankenPHPApplicationRunner(rootPath: __DIR__))->run();
```

Uygulamayı worker modunda başlatmak için `Caddyfile` dosyanızı güncelleyin:

```caddyfile
{
	frankenphp
}

localhost {
	encode zstd br gzip
	php_server {
		root public/
		worker ./worker.php {
			# Tüm istekleri worker'a gönder
			match *
			# PHP dosyaları değiştiğinde worker'ları yeniden yükle (yalnızca geliştirme)
			watch ./**/*.php
		}
	}
}
```

Uygulamanız [resmi Yii uygulama şablonuna](https://github.com/yiisoft/app) dayanıyorsa, debug, ortam ve hata işleyici yapılandırması içeren eksiksiz bir `worker.php` örneği için [paket benioku](https://github.com/yiisoft/yii-runner-frankenphp) dosyasına bakın.

Bir worker'ın yeniden başlatılmadan önce işlediği istek sayısını sınırlamak için (bellek sızıntılarını azaltmakta yararlıdır) `MAX_REQUESTS` ortam değişkenini ayarlayın. Varsayılan olarak, worker'lar istekleri süresiz işler.

Worker modunu kullanırken, durum tutan servislerin her istekten sonra sıfırlandığından emin olun. Ayrıntılar için [worker modu belgelerine](worker.md) ve [Yii DI `StateResetter` belgelerine](https://github.com/yiisoft/di#resetting-services-state) bakın.
