# syntax=docker/dockerfile:1
#checkov:skip=CKV_DOCKER_2
#checkov:skip=CKV_DOCKER_3
FROM golang:1.25

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
	pkg-config \
	re2c"

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

# hadolint ignore=DL3009
RUN apt-get update && \
	apt-get -y --no-install-recommends install \
	$PHPIZE_DEPS \
	libargon2-dev \
	libbrotli-dev \
	libcurl4-openssl-dev \
	libonig-dev \
	libreadline-dev \
	libsodium-dev \
	libsqlite3-dev \
	libssl-dev \
	libxml2-dev \
	zlib1g-dev \
	bison \
	libnss3-tools \
	# Dev tools \
	git \
	clang \
	cmake \
	llvm \
	gdb \
	valgrind \
	neovim \
	zsh \
	libtool-bin && \
	echo 'set auto-load safe-path /' > /root/.gdbinit && \
	echo '* soft core unlimited' >> /etc/security/limits.conf \
	&& \
	apt-get clean

WORKDIR /usr/local/src/php
RUN git clone --branch=PHP-8.4 https://github.com/php/php-src.git . && \
	# --enable-embed is only necessary to generate libphp.so, we don't use this SAPI directly
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
	ldconfig && \
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
	cmake --install build && \
	cp build/libwatcher-c.so /usr/local/lib/libwatcher-c.so && \
	ldconfig

WORKDIR /go/src/app
COPY --link . ./

WORKDIR /go/src/app/caddy/frankenphp
RUN ../../go.sh build -buildvcs=false

WORKDIR /go/src/app
CMD [ "zsh" ]
