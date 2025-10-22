# syntax=docker/dockerfile:1
#checkov:skip=CKV_DOCKER_2
#checkov:skip=CKV_DOCKER_3
#checkov:skip=CKV_DOCKER_7
FROM golang-base

ARG TARGETARCH

ARG FRANKENPHP_VERSION=''
ENV FRANKENPHP_VERSION=${FRANKENPHP_VERSION}

ARG PHP_VERSION=''
ENV PHP_VERSION=${PHP_VERSION}

ARG PHP_EXTENSIONS=''
ARG PHP_EXTENSION_LIBS=''
ARG XCADDY_ARGS=''
ARG CLEAN=''
ARG EMBED=''
ARG DEBUG_SYMBOLS=''
ARG MIMALLOC=''
ARG NO_COMPRESS=''
ARG INCLUDE_CLI=''
ENV INCLUDE_CLI=${INCLUDE_CLI}

ENV GOTOOLCHAIN=local

SHELL ["/bin/ash", "-eo", "pipefail", "-c"]

LABEL org.opencontainers.image.title=FrankenPHP
LABEL org.opencontainers.image.description="The modern PHP app server"
LABEL org.opencontainers.image.url=https://frankenphp.dev
LABEL org.opencontainers.image.source=https://github.com/php/frankenphp
LABEL org.opencontainers.image.licenses=MIT
LABEL org.opencontainers.image.vendor="Kévin Dunglas"

RUN apk update; \
	apk add --no-cache \
		alpine-sdk \
		autoconf \
		automake \
		bash \
		binutils \
		bison \
		build-base \
		cmake \
		curl \
		file \
		flex \
		g++ \
		gcc \
		git \
		jq \
		libgcc \
		libstdc++ \
		libtool \
		linux-headers \
		m4 \
		make \
		pkgconfig \
		php84 \
		php84-common \
		php84-ctype \
		php84-curl \
		php84-dom \
		php84-iconv \
		php84-mbstring \
		php84-openssl \
		php84-pcntl \
		php84-phar \
		php84-posix \
		php84-session \
		php84-sodium \
		php84-tokenizer \
		php84-xml \
		php84-xmlwriter \
		upx \
		wget \
		xz ; \
	ln -sf /usr/bin/php84 /usr/bin/php && \
	go install github.com/caddyserver/xcaddy/cmd/xcaddy@latest

# https://getcomposer.org/doc/03-cli.md#composer-allow-superuser
ENV COMPOSER_ALLOW_SUPERUSER=1
COPY --from=composer/composer:2-bin /composer /usr/bin/composer

WORKDIR /go/src/app
COPY go.mod go.sum ./
RUN go mod download

WORKDIR /go/src/app/caddy
COPY caddy/go.mod caddy/go.sum ./
RUN go mod download

WORKDIR /go/src/app
COPY --link . ./

ENV SPC_DEFAULT_C_FLAGS='-fPIE -fPIC -O3'
ENV SPC_LIBC='musl'
ENV SPC_CMD_VAR_PHP_MAKE_EXTRA_LDFLAGS_PROGRAM='-Wl,-O3 -pie'
ENV SPC_OPT_BUILD_ARGS='--with-config-file-path=/etc/frankenphp --with-config-file-scan-dir=/etc/frankenphp/php.d'
ENV SPC_REL_TYPE='binary'
ENV EXTENSION_DIR='/usr/lib/frankenphp/modules'

RUN --mount=type=secret,id=github-token \
	GITHUB_TOKEN=$(cat /run/secrets/github-token) ./build-static.sh && \
	rm -Rf dist/static-php-cli/source/*
