# Эффективная отдача больших статических файлов (`X-Sendfile`/`X-Accel-Redirect`)

Обычно статические файлы могут быть отданы напрямую веб-сервером,
но иногда необходимо выполнить некоторый PHP-код перед их отправкой:
контроль доступа, сбор статистики, установка пользовательских HTTP-заголовков...

К сожалению, использование PHP для отдачи больших статических файлов неэффективно по сравнению с
прямым использованием веб-сервера (перегрузка памяти, снижение производительности...).

FrankenPHP позволяет делегировать отправку статических файлов веб-серверу
**после** выполнения пользовательского PHP-кода.

Для этого ваше PHP-приложение просто должно определить пользовательский HTTP-заголовок,
содержащий путь к файлу, который нужно отдать. FrankenPHP позаботится обо всем остальном.

Эта функция известна как **`X-Sendfile`** для Apache и **`X-Accel-Redirect`** для NGINX.

В следующих примерах мы предполагаем, что корневой каталог проекта — это директория `public/`,
и что мы хотим использовать PHP для отдачи файлов, хранящихся вне директории `public/`,
из директории с названием `private-files/`.

## Конфигурация

Сначала добавьте следующую конфигурацию в ваш `Caddyfile`, чтобы включить эту функцию:

```patch
	root public/
	# ...

+	# Требуется для Symfony, Laravel и других проектов, использующих компонент Symfony HttpFoundation
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
+			# Удаляет заголовок X-Accel-Redirect, установленный PHP, для повышения безопасности
+			header -X-Accel-Redirect
+
+			file_server
+		}
+	}

	php_server
```

## Чистый PHP

Установите относительный путь к файлу (относительно `private-files/`) в качестве значения заголовка `X-Accel-Redirect`:

```php
header('X-Accel-Redirect: file.txt');
```

## Проекты, использующие компонент Symfony HttpFoundation (Symfony, Laravel, Drupal...)

Symfony HttpFoundation [нативно поддерживает эту функцию](https://symfony.com/doc/current/components/http_foundation.html#serving-files).
Он автоматически определит правильное значение для заголовка `X-Accel-Redirect` и добавит его в ответ.

```php
use Symfony\Component\HttpFoundation\BinaryFileResponse;

BinaryFileResponse::trustXSendfileTypeHeader();
$response = new BinaryFileResponse(__DIR__.'/../private-files/file.txt');

// ...
```
