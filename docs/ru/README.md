# FrankenPHP: Современный сервер приложений для PHP

<h1 align="center"><a href="https://frankenphp.dev"><img src="../../frankenphp.png" alt="FrankenPHP" width="600"></a></h1>

**FrankenPHP** — это современный сервер приложений для PHP, построенный на базе веб-сервера [Caddy](https://caddyserver.com/).

FrankenPHP добавляет новые возможности вашим PHP-приложениям благодаря следующим функциям: [_Early Hints_](https://frankenphp.dev/docs/early-hints/), [Worker режим](https://frankenphp.dev/docs/worker/), [Real-time режим](https://frankenphp.dev/docs/mercure/), автоматическая поддержка HTTPS, HTTP/2 и HTTP/3.

FrankenPHP совместим с любыми PHP-приложениями и значительно ускоряет ваши проекты на Laravel и Symfony благодаря их официальной поддержке в worker режиме.

FrankenPHP также может использоваться как автономная Go-библиотека для встраивания PHP в любое приложение с использованием `net/http`.

[**Узнайте больше** на сайте _frankenphp.dev_](https://frankenphp.dev) или из этой презентации:

<a href="https://dunglas.dev/2022/10/frankenphp-the-modern-php-app-server-written-in-go/"><img src="https://dunglas.dev/wp-content/uploads/2022/10/frankenphp.png" alt="Slides" width="600"></a>

## Начало работы

В Windows используйте [WSL](https://learn.microsoft.com/windows/wsl/) для запуска FrankenPHP.

### Скрипт установки

Скопируйте и выполните эту команду в терминале, чтобы автоматически установить подходящую версию для вашей платформы:

```console
curl https://frankenphp.dev/install.sh | sh
```

### Автономный бинарный файл

Если вы предпочитаете не использовать Docker, мы предоставляем автономные статические бинарные файлы FrankenPHP для Linux и macOS, включающие [PHP 8.4](https://www.php.net/releases/8.4/en.php) и большинство популярных PHP‑расширений.

[Скачать FrankenPHP](https://github.com/php/frankenphp/releases)

**Установка расширений:** Наиболее распространенные расширения уже включены. Устанавливать дополнительные расширения невозможно.

### Пакеты rpm

Наши мейнтейнеры предлагают rpm‑пакеты для всех систем с `dnf`. Для установки выполните:

```console
sudo dnf install https://rpm.henderkes.com/static-php-1-0.noarch.rpm
sudo dnf module enable php-zts:static-8.4 # доступны 8.2–8.5
sudo dnf install frankenphp
```

**Установка расширений:** `sudo dnf install php-zts-<extension>`

Для расширений, недоступных по умолчанию, используйте [PIE](https://github.com/php/pie):

```console
sudo dnf install pie-zts
sudo pie-zts install asgrim/example-pie-extension
```

### Пакеты deb

Наши мейнтейнеры предлагают deb‑пакеты для всех систем с `apt`. Для установки выполните:

```console
sudo curl -fsSL https://key.henderkes.com/static-php.gpg -o /usr/share/keyrings/static-php.gpg && \
echo "deb [signed-by=/usr/share/keyrings/static-php.gpg] https://deb.henderkes.com/ stable main" | sudo tee /etc/apt/sources.list.d/static-php.list && \
sudo apt update
sudo apt install frankenphp
```

**Установка расширений:** `sudo apt install php-zts-<extension>`

Для расширений, недоступных по умолчанию, используйте [PIE](https://github.com/php/pie):

```console
sudo apt install php-zts-devel
sudo pie-zts install asgrim/example-pie-extension
```

### Docker

```console
docker run -v .:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

Перейдите по адресу `https://localhost` и наслаждайтесь!

> [!TIP]
>
> Не используйте `https://127.0.0.1`. Используйте `https://localhost` и настройте самоподписанный сертификат.  
> Чтобы изменить используемый домен, настройте переменную окружения [`SERVER_NAME`](config.md#переменные-окружения).

### Homebrew

FrankenPHP также доступен как пакет [Homebrew](https://brew.sh) для macOS и Linux.

```console
brew install dunglas/frankenphp/frankenphp
```

**Установка расширений:** Используйте [PIE](https://github.com/php/pie).

### Использование

Для запуска содержимого текущего каталога выполните:

```console
frankenphp php-server
```

Также можно запускать CLI‑скрипты:

```console
frankenphp php-cli /path/to/your/script.php
```

Для пакетов deb и rpm можно запустить сервис systemd:

```console
sudo systemctl start frankenphp
```

## Документация

- [Worker режим](https://frankenphp.dev/docs/worker/)
- [Поддержка Early Hints (103 HTTP статус код)](https://frankenphp.dev/docs/early-hints/)
- [Real-time режим](https://frankenphp.dev/docs/mercure/)
- [Конфигурация](https://frankenphp.dev/docs/config/)
- [Docker-образы](https://frankenphp.dev/docs/docker/)
- [Деплой в продакшен](https://frankenphp.dev/docs/production/)
- [Оптимизация производительности](https://frankenphp.dev/docs/performance/)
- [Создание автономного PHP-приложений](https://frankenphp.dev/docs/embed/)
- [Создание статических бинарных файлов](https://frankenphp.dev/docs/static/)
- [Компиляция из исходников](https://frankenphp.dev/docs/compile/)
- [Интеграция с Laravel](https://frankenphp.dev/docs/laravel/)
- [Известные проблемы](https://frankenphp.dev/docs/known-issues/)
- [Демо-приложение (Symfony) и бенчмарки](https://github.com/dunglas/frankenphp-demo)
- [Документация Go-библиотеки](https://pkg.go.dev/github.com/dunglas/frankenphp)
- [Участие в проекте и отладка](https://frankenphp.dev/docs/contributing/)

## Примеры и шаблоны

- [Symfony](https://github.com/dunglas/symfony-docker)
- [API Platform](https://api-platform.com/docs/symfony)
- [Laravel](https://frankenphp.dev/docs/laravel/)
- [Sulu](https://sulu.io/blog/running-sulu-with-frankenphp)
- [WordPress](https://github.com/StephenMiracle/frankenwp)
- [Drupal](https://github.com/dunglas/frankenphp-drupal)
- [Joomla](https://github.com/alexandreelise/frankenphp-joomla)
- [TYPO3](https://github.com/ochorocho/franken-typo3)
