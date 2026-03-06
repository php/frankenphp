# Участие в проекте

## Компиляция PHP

### С помощью Docker (Linux)

Создайте образ Docker для разработки:

```console
docker build -t frankenphp-dev -f dev.Dockerfile .
docker run --cap-add=SYS_PTRACE --security-opt seccomp=unconfined -p 8080:8080 -p 443:443 -p 443:443/udp -v $PWD:/go/src/app -it frankenphp-dev
```

Образ содержит стандартные инструменты для разработки (Go, GDB, Valgrind, Neovim и др.) и использует следующие пути для настроек PHP

- php.ini: `/etc/frankenphp/php.ini` По умолчанию предоставляется файл php.ini с настройками для разработки.
- дополнительные файлы конфигурации: `/etc/frankenphp/php.d/*.ini`
- расширения php: `/usr/lib/frankenphp/modules/`

Если ваша версия Docker ниже 23.0, сборка может завершиться ошибкой из-за [проблемы с шаблонами dockerignore](https://github.com/moby/moby/pull/42676). Добавьте в `.dockerignore` следующие директории:

```patch
 !testdata/*.php
 !testdata/*.txt
+!caddy
+!internal
```

### Без Docker (Linux и macOS)

[Следуйте инструкциям по компиляции из исходников](https://frankenphp.dev/docs/compile/) и укажите флаг конфигурации `--debug`.

## Запуск тестов

```console
go test -race -v ./...
```

## Модуль Caddy

Соберите Caddy с модулем FrankenPHP:

```console
cd caddy/frankenphp/
go build -tags nobadger,nomysql,nopgx
cd ../../
```

Запустите Caddy с модулем FrankenPHP:

```console
cd testdata/
../caddy/frankenphp/frankenphp run
```

Сервер будет доступен по адресу `127.0.0.1:8080`:

```console
curl -vk https://localhost/phpinfo.php
```

## Минимальный тестовый сервер

Соберите минимальный тестовый сервер:

```console
cd internal/testserver/
go build
cd ../../
```

Запустите тестовый сервер:

```console
cd testdata/
../internal/testserver/testserver
```

Сервер будет доступен по адресу `127.0.0.1:8080`:

```console
curl -v http://127.0.0.1:8080/phpinfo.php
```

## Локальная сборка Docker-образов

Выведите план bake:

```console
docker buildx bake -f docker-bake.hcl --print
```

Соберите образы FrankenPHP для amd64 локально:

```console
docker buildx bake -f docker-bake.hcl --pull --load --set "*.platform=linux/amd64"
```

Соберите образы FrankenPHP для arm64 локально:

```console
docker buildx bake -f docker-bake.hcl --pull --load --set "*.platform=linux/arm64"
```

Соберите образы FrankenPHP с нуля для arm64 и amd64 и отправьте их в Docker Hub:

```console
docker buildx bake -f docker-bake.hcl --pull --no-cache --push
```

## Отладка ошибок сегментации с использованием статических сборок

1. Скачайте отладочную версию бинарного файла FrankenPHP с GitHub или создайте собственную статическую сборку с включённым отладочным режимом:

   ```console
   docker buildx bake \
       --load \
       --set static-builder.args.DEBUG_SYMBOLS=1 \
       --set "static-builder.platform=linux/amd64" \
       static-builder
   docker cp $(docker create --name static-builder-musl dunglas/frankenphp:static-builder-musl):/go/src/app/dist/frankenphp-linux-$(uname -m) frankenphp
   ```

2. Замените текущую версию `frankenphp` на бинарный файл с включенным отладочным режимом.
3. Запустите FrankenPHP как обычно (или сразу запустите FrankenPHP с GDB: `gdb --args frankenphp run`).
4. Подключитесь к процессу через GDB:

   ```console
   gdb -p `pidof frankenphp`
   ```

5. При необходимости введите `continue` в консоли GDB.
6. Вызовите сбой FrankenPHP.
7. Введите `bt` в консоли GDB.
8. Скопируйте вывод.

## Отладка ошибок сегментации в GitHub Actions

1. Откройте файл `.github/workflows/tests.yml`.
2. Включите режим отладки PHP:

   ```patch
       - uses: shivammathur/setup-php@v2
         # ...
         env:
           phpts: ts
   +       debug: true
   ```

3. Настройте `tmate` для удалённого подключения к контейнеру:

   ```patch
       -
         name: Set CGO flags
         run: echo "CGO_CFLAGS=$(php-config --includes)" >> "$GITHUB_ENV"
   +   -
   +     run: |
   +       sudo apt install gdb
   +       mkdir -p /home/runner/.config/gdb/
   +       printf "set auto-load safe-path /\nhandle SIG34 nostop noprint pass" > /home/runner/.config/gdb/gdbinit
   +   -
   +     uses: mxschmitt/action-tmate@v3
   ```

4. Подключитесь к контейнеру.
5. Откройте файл `frankenphp.go`.
6. Включите `cgosymbolizer`:

   ```patch
   -	//_ "github.com/ianlancetaylor/cgosymbolizer"
   +	_ "github.com/ianlancetaylor/cgosymbolizer"
   ```

7. Загрузите модуль: `go get`.
8. В контейнере используйте GDB и другие инструменты:

   ```console
   go test -c -ldflags=-w
   gdb --args frankenphp.test -test.run ^MyTest$
   ```

9. После исправления ошибки откатите все внесенные изменения.

## Дополнительные ресурсы для разработки

- [Встраивание PHP в uWSGI](https://github.com/unbit/uwsgi/blob/master/plugins/php/php_plugin.c)
- [Встраивание PHP в NGINX Unit](https://github.com/nginx/unit/blob/master/src/nxt_php_sapi.c)
- [Встраивание PHP в Go (go-php)](https://github.com/deuill/go-php)
- [Встраивание PHP в Go (GoEmPHP)](https://github.com/mikespook/goemphp)
- [Встраивание PHP в C++](https://gist.github.com/paresy/3cbd4c6a469511ac7479aa0e7c42fea7)
- [Книга "Extending and Embedding PHP" Сары Големан](https://books.google.fr/books?id=zMbGvK17_tYC&pg=PA254&lpg=PA254#v=onepage&q&f=false)
- [Статья: Что такое TSRMLS_CC?](http://blog.golemon.com/2006/06/what-heck-is-tsrmlscc-anyway.html)
- [SDL bindings](https://pkg.go.dev/github.com/veandco/go-sdl2@v0.4.21/sdl#Main)

## Docker-ресурсы

- [Определение файлов bake](https://docs.docker.com/build/customize/bake/file-definition/)
- [Документация по команде `docker buildx build`](https://docs.docker.com/engine/reference/commandline/buildx_build/)

## Полезные команды

```console
apk add strace util-linux gdb
strace -e 'trace=!futex,epoll_ctl,epoll_pwait,tgkill,rt_sigreturn' -p 1
```

## Перевод документации

Чтобы перевести документацию и сайт на новый язык, выполните следующие шаги:

1. Создайте новую директорию с 2-буквенным ISO-кодом языка в папке `docs/`.
2. Скопируйте все `.md` файлы из корня папки `docs/` в новую директорию (используйте английскую версию как основу для перевода).
3. Скопируйте файлы `README.md` и `CONTRIBUTING.md` из корневой директории в новую папку.
4. Переведите содержимое файлов, но не изменяйте имена файлов. Не переводите строки, начинающиеся с `> [!`, это специальная разметка GitHub.
5. Создайте Pull Request с переводом.
6. В [репозитории сайта](https://github.com/dunglas/frankenphp-website/tree/main) скопируйте и переведите файлы в папках `content/`, `data/` и `i18n/`.
7. Переведите значения в созданных YAML-файлах.
8. Откройте Pull Request в репозитории сайта.
