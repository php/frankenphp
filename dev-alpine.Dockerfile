# syntax=docker/dockerfile:1
#checkov:skip=CKV_DOCKER_2
#checkov:skip=CKV_DOCKER_3
FROM golang:1.25-alpine

ENV GOTOOLCHAIN=local
ENV CFLAGS="-ggdb3"
ENV PHPIZE_DEPS="\
	autoconf \
	dpkg-dev \
	file \
	g++ \
	gcc \
	libc-dev \
	make \
	pkgconfig \
	re2c"

SHELL ["/bin/ash", "-eo", "pipefail", "-c"]

RUN apk add --no-cache \
	$PHPIZE_DEPS \
	argon2-dev \
	brotli-dev \
	curl-dev \
	oniguruma-dev \
	readline-dev \
	libsodium-dev \
	sqlite-dev \
	openssl-dev \
	libxml2-dev \
	zlib-dev \
	bison \
	nss-tools \
	# file watcher \
	libstdc++ \
	linux-headers \
	# Dev tools \
	git \
	clang \
	cmake \
	llvm \
	gdb \
	valgrind \
	neovim \
	zsh \
	libtool && \
	echo 'set auto-load safe-path /' > /root/.gdbinit

WORKDIR /usr/local/src/php
RUN git clone --branch=PHP-8.5 https://github.com/php/php-src.git . && \
	# --enable-embed is necessary to generate libphp.so, but we don't use this SAPI directly
	./buildconf --force && \
	EXTENSION_DIR=/usr/lib/frankenphp/modules ./configure \
		--enable-embed \
		--enable-zts \
		--disable-zend-signals \
		--enable-zend-max-execution-timers \
		--with-config-file-path=/etc/frankenphp/php.ini \
		--with-config-file-scan-dir=/etc/frankenphp/php.d \
		--enable-debug && \
	make -j"$(nproc)" && \
	make install && \
	ldconfig /etc/ld.so.conf.d && \
		mkdir -p /etc/frankenphp/php.d && \
			cp php.ini-development /etc/frankenphp/php.ini && \
			echo "zend_extension=opcache.so" >> /etc/frankenphp/php.ini && \
			echo "opcache.enable=1" >> /etc/frankenphp/php.ini && \
	php --version

# Install e-dant/watcher (necessary for file watching)
WORKDIR /usr/local/src/watcher
RUN git clone https://github.com/e-dant/watcher . && \
		cmake -S . -B build -DCMAKE_BUILD_TYPE=Release && \
	cmake --build build/ && \
	cmake --install build

WORKDIR /go/src/app
COPY . .

WORKDIR /go/src/app/caddy/frankenphp
RUN ../../go.sh build -buildvcs=false

WORKDIR /go/src/app
CMD [ "zsh" ]
