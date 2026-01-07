# Компиляция из исходников

Этот документ объясняет, как создать бинарный файл FrankenPHP, который будет загружать PHP как динамическую библиотеку.
Это рекомендуемый способ.

Альтернативно, могут быть созданы [полностью и преимущественно статические сборки](static.md).

## Установка PHP

FrankenPHP совместим с PHP 8.2 и выше.

### С помощью Homebrew (Linux и Mac)

Самый простой способ установить версию `libphp`, совместимую с FrankenPHP, — использовать ZTS-пакеты, предоставляемые [Homebrew PHP](https://github.com/shivammathur/homebrew-php).

Сначала, если вы еще не сделали этого, установите [Homebrew](https://brew.sh).

Затем установите ZTS-вариант PHP, Brotli (опционально, для поддержки сжатия) и watcher (опционально, для обнаружения изменений файлов):

```console
brew install shivammathur/php/php-zts brotli watcher
brew link --overwrite --force shivammathur/php/php-zts
```

### Путем компиляции PHP

Альтернативно, вы можете скомпилировать PHP из исходников с опциями, необходимыми для FrankenPHP, выполнив следующие шаги.

Сначала [загрузите исходники PHP](https://www.php.net/downloads.php) и распакуйте их:

```console
tar xf php-*
cd php-*/
```

Далее выполните скрипт `configure` с параметрами, необходимыми для вашей платформы.
Следующие флаги `./configure` обязательны, но вы можете добавить и другие, например, для компиляции расширений или дополнительных функций.

#### Linux

```console
./configure \
    --enable-embed \
    --enable-zts \
    --disable-zend-signals \
    --enable-zend-max-execution-timers
```

#### Mac

Используйте пакетный менеджер [Homebrew](https://brew.sh/) для установки необходимых и опциональных зависимостей:

```console
brew install libiconv bison brotli re2c pkg-config watcher
echo 'export PATH="/opt/homebrew/opt/bison/bin:$PATH"' >> ~/.zshrc
```

Затем выполните скрипт configure:

```console
./configure \
    --enable-embed \
    --enable-zts \
    --disable-zend-signals \
    --with-iconv=/opt/homebrew/opt/libiconv/
```

#### Компиляция PHP

Наконец, скомпилируйте и установите PHP:

```console
make -j"$(getconf _NPROCESSORS_ONLN)"
sudo make install
```

## Установка дополнительных зависимостей

Некоторые функции FrankenPHP зависят от опциональных системных зависимостей, которые должны быть установлены.
Альтернативно, эти функции можно отключить, передав теги сборки компилятору Go.

| Функция                                         | Зависимость                                                                                                   | Тег сборки для отключения |
| :---------------------------------------------- | :------------------------------------------------------------------------------------------------------------ | :------------------------ |
| Сжатие Brotli                                   | [Brotli](https://github.com/google/brotli)                                                                   | nobrotli                  |
| Перезапуск worker-скриптов при изменении файлов | [Watcher C](https://github.com/e-dant/watcher/tree/release/watcher-c)                                        | nowatcher                 |
| [Mercure](mercure.md)                           | [Библиотека Mercure Go](https://pkg.go.dev/github.com/dunglas/mercure) (устанавливается автоматически, лицензия AGPL) | nomercure                 |

## Компиляция Go-приложения

Теперь можно собрать итоговый бинарный файл.

### Использование xcaddy

Рекомендуемый способ — использовать [xcaddy](https://github.com/caddyserver/xcaddy) для компиляции FrankenPHP.
`xcaddy` также позволяет легко добавлять [пользовательские модули Caddy](https://caddyserver.com/docs/modules/) и расширения FrankenPHP:

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/dunglas/frankenphp/caddy \
    --with github.com/dunglas/mercure/caddy \
    --with github.com/dunglas/vulcain/caddy \
    --with github.com/dunglas/caddy-cbrotli
    # Добавьте дополнительные модули Caddy и расширения FrankenPHP здесь
    # опционально, если вы хотите скомпилировать из ваших исходников frankenphp:
    # --with github.com/dunglas/frankenphp=$(pwd) \
    # --with github.com/dunglas/frankenphp/caddy=$(pwd)/caddy

```

> [!TIP]
>
> Если вы используете musl libc (по умолчанию в Alpine Linux) и Symfony,
> возможно, потребуется увеличить размер стека по умолчанию.
> В противном случае вы можете столкнуться с ошибками вроде `PHP Fatal error: Maximum call stack size of 83360 bytes reached during compilation. Try splitting expression`
>
> Для этого измените переменную окружения `XCADDY_GO_BUILD_FLAGS`, например, на
> `XCADDY_GO_BUILD_FLAGS=$'-ldflags "-w -s -extldflags \'-Wl,-z,stack-size=0x80000\'"'`
> (измените значение размера стека в зависимости от требований вашего приложения).

### Без xcaddy

Альтернативно, можно скомпилировать FrankenPHP без `xcaddy`, используя команду `go` напрямую:

```console
curl -L https://github.com/php/frankenphp/archive/refs/heads/main.tar.gz | tar xz
cd frankenphp-main/caddy/frankenphp
CGO_CFLAGS=$(php-config --includes) CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" go build -tags=nobadger,nomysql,nopgx
```
