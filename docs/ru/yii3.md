---
title: Запуск Yii3 с FrankenPHP (Docker, режим worker)
description: Как запустить приложение Yii3 с FrankenPHP, используя образ Docker, локальную установку или режим worker с пакетом yii-runner-frankenphp.
---

# Yii3

## Запуск Yii3 с образом Docker для FrankenPHP

Обслуживание веб-приложения [Yii](https://www.yiiframework.com/) с помощью FrankenPHP так же просто, как монтирование проекта в директорию `/app` официального образа Docker.

Запустите эту команду из корневой директории вашего приложения Yii:

```console
docker run -p 80:80 -p 443:443 -p 443:443/udp -v $PWD:/app dunglas/frankenphp
```

И наслаждайтесь!

## Локальная установка Yii3 с FrankenPHP

В качестве альтернативы вы можете запускать свои проекты Yii с FrankenPHP со своей локальной машины:

1. [Скачайте бинарный файл, соответствующий вашей системе](../#standalone-binary)
2. Добавьте следующую конфигурацию в файл с именем `Caddyfile` в корневой директории вашего проекта Yii:

   ```caddyfile
   {
   	frankenphp
   }

   # Доменное имя вашего сервера
   localhost {
   	# Включить сжатие (опционально)
   	encode zstd br gzip
   	# Выполнять PHP-файлы из директории public/ и обслуживать статические файлы
   	php_server {
   		root public/
   		try_files {path} index.php
   	}
   }
   ```

3. Запустите FrankenPHP из корневой директории вашего проекта Yii: `frankenphp run`

## Режим worker для Yii3

Чтобы запустить ваше приложение Yii в [режиме worker](worker.md), установите [Yii FrankenPHP runner](https://github.com/yiisoft/yii-runner-frankenphp):

```console
composer require yiisoft/yii-runner-frankenphp
```

Затем создайте файл с именем `worker.php` в корневой директории вашего приложения:

```php
<?php

declare(strict_types=1);

use Yiisoft\Yii\Runner\FrankenPHP\FrankenPHPApplicationRunner;

require_once __DIR__ . '/vendor/autoload.php';

(new FrankenPHPApplicationRunner(rootPath: __DIR__))->run();
```

Обновите ваш `Caddyfile`, чтобы запустить приложение в режиме worker:

```caddyfile
{
	frankenphp
}

localhost {
	encode zstd br gzip
	php_server {
		root public/
		worker ./worker.php {
			# Отправлять все запросы worker-у
			match *
			# Перезагружать worker-ы при изменении PHP-файлов (только для разработки)
			watch ./**/*.php
		}
	}
}
```

Если ваше приложение основано на [официальном шаблоне приложения Yii](https://github.com/yiisoft/app), обратитесь к [README пакета](https://github.com/yiisoft/yii-runner-frankenphp) для полного примера `worker.php` с конфигурацией отладки, окружения и обработчика ошибок.

Чтобы ограничить количество запросов, которые worker обрабатывает перед перезапуском (полезно для уменьшения утечек памяти), установите переменную окружения `MAX_REQUESTS`. По умолчанию worker-ы обрабатывают запросы бесконечно.

При использовании режима worker убедитесь, что службы с состоянием сбрасываются после каждого запроса. Подробности см. в [документации по режиму worker](worker.md) и [документации Yii DI `StateResetter`](https://github.com/yiisoft/di#resetting-services-state) для деталей.
