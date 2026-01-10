# Компиляция из исходников

Этот документ объясняет, как создать бинарный файл FrankenPHP, который будет загружать PHP как динамическую библиотеку.  
Это рекомендуемый способ.

Альтернативно можно создать [статическую сборку](static.md).

## Установка PHP

FrankenPHP совместим с PHP версии 8.2 и выше.

Сначала [загрузите исходники PHP](https://www.php.net/downloads.php) и распакуйте их:

```console
tar xf php-*
cd php-*/
```

Далее выполните скрипт `configure` с параметрами, необходимыми для вашей платформы.  
Следующие флаги `./configure` обязательны, но вы можете добавить и другие, например, для компиляции расширений или дополнительных функций.

### Linux

```console
./configure \
    --enable-embed \
    --enable-zts \
    --disable-zend-signals \
    --enable-zend-max-execution-timers
```

### Mac

Используйте пакетный менеджер [Homebrew](https://brew.sh/) для установки
`libiconv`, `bison`, `re2c` и `pkg-config`:

```console
brew install libiconv bison brotli re2c pkg-config
echo 'export PATH="/opt/homebrew/opt/bison/bin:$PATH"' >> ~/.zshrc
```

Затем выполните скрипт configure:

```console
./configure \
    --enable-embed=static \
    --enable-zts \
    --disable-zend-signals \
    --disable-opcache-jit \
    --enable-static \
    --enable-shared=no \
    --with-iconv=/opt/homebrew/opt/libiconv/
```

## Компиляция PHP

Наконец, скомпилируйте и установите PHP:

```console
make -j"$(getconf _NPROCESSORS_ONLN)"
sudo make install
```

## Установка дополнительных зависимостей

Некоторые функции FrankenPHP зависят от опциональных системных зависимостей.  
Альтернативно, эти функции можно отключить, передав соответствующие теги сборки компилятору Go.

| Функция                                         | Зависимость                                                           | Тег сборки для отключения |
| ----------------------------------------------- | --------------------------------------------------------------------- | ------------------------- |
| Сжатие Brotli                                   | [Brotli](https://github.com/google/brotli)                            | nobrotli                  |
| Перезапуск worker-скриптов при изменении файлов | [Watcher C](https://github.com/e-dant/watcher/tree/release/watcher-c) | nowatcher                 |

## Компиляция Go-приложения

Теперь можно собрать итоговый бинарный файл:

```console
curl -L https://github.com/php/frankenphp/archive/refs/heads/main.tar.gz | tar xz
cd frankenphp-main/caddy/frankenphp
CGO_CFLAGS=$(php-config --includes) CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" go build -tags=nobadger,nomysql,nopgx
```

### Использование xcaddy

Альтернативно, используйте [xcaddy](https://github.com/caddyserver/xcaddy) для компиляции FrankenPHP с [пользовательскими модулями Caddy](https://caddyserver.com/docs/modules/):

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/dunglas/frankenphp/caddy \
    --with github.com/dunglas/mercure/caddy \
    --with github.com/dunglas/vulcain/caddy
    # Добавьте дополнительные модули Caddy здесь
```

> [!TIP]
>
> Если вы используете musl libc (по умолчанию в Alpine Linux) и Symfony,  
> возможно, потребуется увеличить размер стека.  
> В противном случае вы можете столкнуться с ошибками вроде  
> `PHP Fatal error: Maximum call stack size of 83360 bytes reached during compilation. Try splitting expression`.
>
> Для этого измените значение переменной окружения `XCADDY_GO_BUILD_FLAGS`, например:
> `XCADDY_GO_BUILD_FLAGS=$'-ldflags "-w -s -extldflags \'-Wl,-z,stack-size=0x80000\'"'`  
> (измените значение размера стека в зависимости от требований вашего приложения).
