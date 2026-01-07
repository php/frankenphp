# Конфигурация

FrankenPHP, Caddy, а также модули [Mercure](mercure.md) и [Vulcain](https://vulcain.rocks) могут быть настроены с использованием [форматов, поддерживаемых Caddy](https://caddyserver.com/docs/getting-started#your-first-config).

Наиболее распространенным форматом является `Caddyfile` — простой, человекочитаемый текстовый формат.
По умолчанию FrankenPHP будет искать `Caddyfile` в текущей директории.
Вы можете указать собственный путь с помощью опции `-c` или `--config`.

Минимальный `Caddyfile` для обслуживания PHP-приложения показан ниже:

```caddyfile
# The hostname to respond to
localhost

# Optionaly, the directory to serve files from, otherwise defaults to the current directory
#root public/
php_server
```

Более продвинутый `Caddyfile`, включающий больше возможностей и предоставляющий удобные переменные окружения, находится [в репозитории FrankenPHP](https://github.com/php/frankenphp/blob/main/caddy/frankenphp/Caddyfile),
а также поставляется с Docker-образами.

Сам PHP может быть настроен [с помощью файла `php.ini`](https://www.php.net/manual/en/configuration.file.php).

В зависимости от метода установки, FrankenPHP и PHP-интерпретатор будут искать конфигурационные файлы в местах, описанных ниже.

## Docker

FrankenPHP:

- `/etc/frankenphp/Caddyfile`: основной конфигурационный файл
- `/etc/frankenphp/Caddyfile.d/*.caddyfile`: дополнительные конфигурационные файлы, загружаемые автоматически

PHP:

- `php.ini`: `/usr/local/etc/php/php.ini` (файл `php.ini` по умолчанию не предоставляется)
- дополнительные конфигурационные файлы: `/usr/local/etc/php/conf.d/*.ini`
- PHP-расширения: `/usr/local/lib/php/extensions/no-debug-zts-<YYYYMMDD>/`
- Вам следует скопировать официальный шаблон, предоставляемый проектом PHP:

```dockerfile
FROM dunglas/frankenphp

# Production:
RUN cp $PHP_INI_DIR/php.ini-production $PHP_INI_DIR/php.ini

# Or development:
RUN cp $PHP_INI_DIR/php.ini-development $PHP_INI_DIR/php.ini
```

## RPM и Debian пакеты

FrankenPHP:

- `/etc/frankenphp/Caddyfile`: основной конфигурационный файл
- `/etc/frankenphp/Caddyfile.d/*.caddyfile`: дополнительные конфигурационные файлы, загружаемые автоматически

PHP:

- `php.ini`: `/etc/php-zts/php.ini` (файл `php.ini` с производственными настройками предоставляется по умолчанию)
- дополнительные конфигурационные файлы: `/etc/php-zts/conf.d/*.ini`

## Статический бинарный файл

FrankenPHP:

- В текущей рабочей директории: `Caddyfile`

PHP:

- `php.ini`: Директория, в которой выполняется `frankenphp run` или `frankenphp php-server`, затем `/etc/frankenphp/php.ini`
- дополнительные конфигурационные файлы: `/etc/frankenphp/php.d/*.ini`
- PHP-расширения: не могут быть загружены, их следует встраивать в сам бинарный файл
- скопируйте один из `php.ini-production` или `php.ini-development`, предоставленных [в исходниках PHP](https://github.com/php/php-src/).

## Конфигурация Caddyfile

[HTTP-директивы](https://caddyserver.com/docs/caddyfile/concepts#directives) `php_server` или `php` могут быть использованы в блоках сайта для обслуживания вашего PHP-приложения.

Минимальный пример:

```caddyfile
localhost {
	# Включить сжатие (опционально)
	encode zstd br gzip
	# Выполнять PHP-файлы в текущей директории и обслуживать ресурсы
	php_server
}
```

Вы также можете явно настроить FrankenPHP с помощью [глобальной опции](https://caddyserver.com/docs/caddyfile/concepts#global-options) `frankenphp`:

```caddyfile
{
	frankenphp {
		num_threads <num_threads> # Устанавливает количество PHP-потоков для запуска. По умолчанию: 2x от числа доступных CPU.
		max_threads <num_threads> # Ограничивает количество дополнительных PHP-потоков, которые могут быть запущены во время выполнения. По умолчанию: num_threads. Может быть установлено в 'auto'.
		max_wait_time <duration> # Устанавливает максимальное время, в течение которого запрос может ожидать свободный PHP-поток до тайм-аута. По умолчанию: отключено.
		php_ini <key> <value> # Устанавливает директиву php.ini. Может быть использовано несколько раз для установки нескольких директив.
		worker {
			file <path> # Устанавливает путь к worker-скрипту.
			num <num> # Устанавливает количество PHP-потоков для запуска, по умолчанию 2x от числа доступных CPU.
			env <key> <value> # Устанавливает дополнительную переменную окружения с указанным значением. Может быть указано несколько раз для нескольких переменных окружения.
			watch <path> # Устанавливает путь для отслеживания изменений файлов. Может быть указано несколько раз для нескольких путей.
			name <name> # Устанавливает имя worker, используемое в логах и метриках. По умолчанию: абсолютный путь к файлу worker.
			max_consecutive_failures <num> # Устанавливает максимальное количество последовательных сбоев, после которых worker считается неработоспособным; -1 означает, что worker будет всегда перезапускаться. По умолчанию: 6.
		}
	}
}

# ...
```

В качестве альтернативы можно использовать однострочную краткую форму для опции `worker`:

```caddyfile
{
	frankenphp {
		worker <file> <num>
	}
}

# ...
```

Вы также можете определить несколько workers, если обслуживаете несколько приложений на одном сервере:

```caddyfile
app.example.com {
    root /path/to/app/public
	php_server {
		root /path/to/app/public # позволяет лучше кэшировать
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

Использование директивы `php_server` — это то, что нужно в большинстве случаев,
но если вам нужен полный контроль, вы можете использовать более низкоуровневую директиву `php`.
Директива `php` передает все входные данные в PHP, вместо того чтобы сначала проверять, является ли это PHP-файлом или нет. Подробнее об этом читайте на [странице производительности](performance.md#try_files).

Использование директивы `php_server` эквивалентно следующей конфигурации:

```caddyfile
route {
	# Add trailing slash for directory requests
	@canonicalPath {
		file {path}/index.php
		not path */
	}
	redir @canonicalPath {path}/ 308
	# If the requested file does not exist, try index files
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

Директивы `php_server` и `php` имеют следующие опции:

```caddyfile
php_server [<matcher>] {
	root <directory> # Устанавливает корневую папку сайта. По умолчанию: директива `root`.
	split_path <delim...> # Устанавливает подстроки для разделения URI на две части. Первая соответствующая подстрока будет использована для разделения "path info" от пути. Первая часть будет дополнена соответствующей подстрокой и будет считаться именем фактического ресурса (CGI-скрипта). Вторая часть будет установлена как PATH_INFO для использования скриптом. По умолчанию: `.php`.
	resolve_root_symlink false # Отключает разрешение корневой директории до ее фактического значения путем оценки символической ссылки, если таковая существует (включено по умолчанию).
	env <key> <value> # Устанавливает дополнительную переменную окружения с указанным значением. Может быть указано несколько раз для нескольких переменных окружения.
	file_server off # Отключает встроенную директиву file_server.
	worker { # Создает worker, специфичный для этого сервера. Может быть указано несколько раз для нескольких workers.
		file <path> # Устанавливает путь к worker-скрипту, может быть относительным к корню php_server.
		num <num> # Устанавливает количество PHP-потоков для запуска, по умолчанию 2x от числа доступных CPU.
		name <name> # Устанавливает имя для worker, используемое в логах и метриках. По умолчанию: абсолютный путь к файлу worker. Всегда начинается с m# при определении в блоке php_server.
		watch <path> # Устанавливает путь для отслеживания изменений файлов. Может быть указано несколько раз для нескольких путей.
		env <key> <value> # Устанавливает дополнительную переменную окружения с указанным значением. Может быть указано несколько раз для нескольких переменных окружения. Переменные окружения для этого worker также наследуются от родительского php_server, но могут быть переопределены здесь.
		match <path> # сопоставляет worker с шаблоном пути. Переопределяет try_files и может быть использовано только в директиве php_server.
	}
	worker <other_file> <num> # Также можно использовать краткую форму как в глобальном блоке frankenphp.
}
```

### Отслеживание изменений файлов

Поскольку workers запускают ваше приложение только один раз и держат его в памяти, любые изменения
в ваших PHP-файлах не будут отражены немедленно.

Workers могут быть перезапущены при изменении файлов с помощью директивы `watch`.
Это полезно для сред разработки.

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

Эта функция часто используется в сочетании с [горячей перезагрузкой](hot-reload.md).

Если директория `watch` не указана, по умолчанию будет использоваться `./**/*.{env,php,twig,yaml,yml}`,
который отслеживает все файлы `.env`, `.php`, `.twig`, `.yaml` и `.yml` в директории и поддиректориях,
где был запущен процесс FrankenPHP. Вы также можете указать одну или несколько директорий с помощью
[шаблона имени файла оболочки](https://pkg.go.dev/path/filepath#Match):

```caddyfile
{
	frankenphp {
		worker {
			file  /path/to/app/public/worker.php
			watch /path/to/app # отслеживает все файлы во всех поддиректориях /path/to/app
			watch /path/to/app/*.php # отслеживает файлы, заканчивающиеся на .php в /path/to/app
			watch /path/to/app/**/*.php # отслеживает PHP-файлы в /path/to/app и поддиректориях
			watch /path/to/app/**/*.{php,twig} # отслеживает PHP- и Twig-файлы в /path/to/app и поддиректориях
		}
	}
}
```

- Шаблон `**` означает рекурсивное отслеживание.
- Директории также могут быть относительными (относительно места запуска процесса FrankenPHP).
- Если у вас определено несколько workers, все они будут перезапущены при изменении файла.
- Будьте осторожны с отслеживанием файлов, которые создаются во время выполнения (например, логов), так как это может вызвать нежелательные перезапуски worker.

Наблюдатель за файлами основан на [e-dant/watcher](https://github.com/e-dant/watcher).

## Сопоставление Worker с путем

В традиционных PHP-приложениях скрипты всегда размещаются в публичной директории.
Это также относится и к worker-скриптам, которые обрабатываются как любые другие PHP-скрипты.
Если вы хотите разместить worker-скрипт вне публичной директории, вы можете сделать это с помощью директивы `match`.

Директива `match` — это оптимизированная альтернатива `try_files`, доступная только внутри `php_server` и `php`.
Следующий пример всегда будет отдавать файл из публичной директории, если он присутствует,
и в противном случае перенаправлять запрос worker, соответствующему шаблону пути.

```caddyfile
{
	frankenphp {
		php_server {
			worker {
				file /path/to/worker.php # файл может находиться за пределами публичного пути
				match /api/* # все запросы, начинающиеся с /api/, будут обрабатываться этим worker
			}
		}
	}
}
```

## Переменные окружения

Следующие переменные окружения могут быть использованы для внедрения директив Caddy в `Caddyfile` без его изменения:

- `SERVER_NAME`: изменение [адресов для прослушивания](https://caddyserver.com/docs/caddyfile/concepts#addresses); предоставленные хостнеймы также будут использованы для генерации TLS-сертификата
- `SERVER_ROOT`: изменение корневой директории сайта, по умолчанию `public/`
- `CADDY_GLOBAL_OPTIONS`: внедрение [глобальных опций](https://caddyserver.com/docs/caddyfile/options)
- `FRANKENPHP_CONFIG`: внедрение конфигурации под директивой `frankenphp`

Как и для FPM и CLI SAPIs, переменные окружения по умолчанию доступны в суперглобальной переменной `$_SERVER`.

Значение `S` в [директиве PHP `variables_order`](https://www.php.net/manual/en/ini.core.php#ini.variables-order) всегда эквивалентно `ES` независимо от расположения `E` в этой директиве.

## Конфигурация PHP

Для загрузки [дополнительных конфигурационных файлов PHP](https://www.php.net/manual/en/configuration.file.php#configuration.file.scan)
можно использовать переменную окружения `PHP_INI_SCAN_DIR`.
Когда она установлена, PHP загрузит все файлы с расширением `.ini`, присутствующие в указанных директориях.

Вы также можете изменить конфигурацию PHP с помощью директивы `php_ini` в `Caddyfile`:

```caddyfile
{
    frankenphp {
        php_ini memory_limit 256M

        # or

        php_ini {
            memory_limit 256M
            max_execution_time 15
        }
    }
}
```

### Отключение HTTPS

По умолчанию FrankenPHP автоматически включает HTTPS для всех хостнеймов, включая `localhost`.
Если вы хотите отключить HTTPS (например, в среде разработки), вы можете установить переменную окружения `SERVER_NAME` в `http://` или `:80`:

В качестве альтернативы вы можете использовать все другие методы, описанные в [документации Caddy](https://caddyserver.com/docs/automatic-https#activation).

Если вы хотите использовать HTTPS с IP-адресом `127.0.0.1` вместо хостнейма `localhost`, пожалуйста, прочтите раздел [известных проблем](known-issues.md#using-https127001-with-docker).

### Полный дуплекс (HTTP/1)

При использовании HTTP/1.x может быть желательно включить режим полного дуплекса, чтобы разрешить запись ответа до того, как будет прочитано все тело запроса. (например: [Mercure](mercure.md), WebSocket, Server-Sent Events и т.д.)

Это опциональная конфигурация, которую необходимо добавить в глобальные опции в `Caddyfile`:

```caddyfile
{
  servers {
    enable_full_duplex
  }
}
```

> [!CAUTION]
>
> Включение этой опции может привести к зависанию старых HTTP/1.x клиентов, которые не поддерживают полный дуплекс.
> Это также можно настроить с помощью переменной окружения `CADDY_GLOBAL_OPTIONS`:

```sh
CADDY_GLOBAL_OPTIONS="servers {
  enable_full_duplex
}"
```

Дополнительную информацию об этой настройке можно найти в [документации Caddy](https://caddyserver.com/docs/caddyfile/options#enable-full-duplex).

## Включение режима отладки

При использовании Docker-образа установите переменную окружения `CADDY_GLOBAL_OPTIONS` в `debug`, чтобы включить режим отладки:

```console
docker run -v $PWD:/app/public \
    -e CADDY_GLOBAL_OPTIONS=debug \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```
