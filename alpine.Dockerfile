# syntax=docker/dockerfile:1
#checkov:skip=CKV_DOCKER_2
#checkov:skip=CKV_DOCKER_3
#checkov:skip=CKV_DOCKER_7
FROM php-base AS common

ARG TARGETARCH

WORKDIR /app

RUN apk add --no-cache \
	ca-certificates \
	libcap \
	mailcap

RUN set -eux; \
	mkdir -p \
		/app/public \
		/config/caddy \
		/data/caddy \
		/etc/caddy \
		/etc/frankenphp; \
	sed -i 's/php/frankenphp run/g' /usr/local/bin/docker-php-entrypoint; \
	echo '<?php phpinfo();' > /app/public/index.php

COPY --link caddy/frankenphp/Caddyfile /etc/caddy/Caddyfile

RUN ln /etc/caddy/Caddyfile /etc/frankenphp/Caddyfile && \
	curl -sSLf \
		-o /usr/local/bin/install-php-extensions \
		https://github.com/mlocati/docker-php-extension-installer/releases/latest/download/install-php-extensions && \
	chmod +x /usr/local/bin/install-php-extensions

CMD ["--config", "/etc/frankenphp/Caddyfile", "--adapter", "caddyfile"]
HEALTHCHECK CMD curl -f http://localhost:2019/metrics || exit 1

# See https://caddyserver.com/docs/conventions#file-locations for details
ENV XDG_CONFIG_HOME=/config
ENV XDG_DATA_HOME=/data

EXPOSE 80
EXPOSE 443
EXPOSE 443/udp
EXPOSE 2019

LABEL org.opencontainers.image.title=FrankenPHP
LABEL org.opencontainers.image.description="The modern PHP app server"
LABEL org.opencontainers.image.url=https://frankenphp.dev
LABEL org.opencontainers.image.source=https://github.com/php/frankenphp
LABEL org.opencontainers.image.licenses=MIT
LABEL org.opencontainers.image.vendor="Kévin Dunglas"


FROM common AS builder

ARG FRANKENPHP_VERSION='dev'
ARG NO_COMPRESS=''
SHELL ["/bin/ash", "-eo", "pipefail", "-c"]

COPY --link --from=golang-base /usr/local/go /usr/local/go

ENV PATH=/usr/local/go/bin:$PATH
ENV GOTOOLCHAIN=local

# hadolint ignore=SC2086
RUN apk add --no-cache --virtual .build-deps \
	$PHPIZE_DEPS \
	argon2-dev \
	# Needed for the custom Go build \
	bash \
	brotli-dev \
	coreutils \
	curl-dev \
	# Needed for the custom Go build \
	git \
	gnu-libiconv-dev \
	libsodium-dev \
	# Needed for the file watcher \
	cmake \
	libstdc++ \
	libxml2-dev \
	linux-headers \
	oniguruma-dev \
	openssl-dev \
	readline-dev \
	sqlite-dev \
	upx

# Install e-dant/watcher (necessary for file watching)
WORKDIR /usr/local/src/watcher
RUN --mount=type=secret,id=github-token \
		if [ -f /run/secrets/github-token ] && [ -s /run/secrets/github-token ]; then \
				curl -s -H "Authorization: Bearer $(cat /run/secrets/github-token)" https://api.github.com/repos/e-dant/watcher/releases/latest; \
		else \
				curl -s https://api.github.com/repos/e-dant/watcher/releases/latest; \
		fi | \
		grep tarball_url | \
		awk '{ print $2 }' | \
		sed 's/,$//' | \
		sed 's/"//g' | \
		xargs curl -L | \
		tar xz --strip-components 1 && \
		cmake -S . -B build -DCMAKE_BUILD_TYPE=Release && \
		cmake --build build && \
		cmake --install build

WORKDIR /go/src/app

COPY --link go.mod go.sum ./
RUN go mod download

WORKDIR /go/src/app/caddy
COPY caddy/go.mod caddy/go.sum ./
RUN go mod download

WORKDIR /go/src/app
COPY --link . ./

# See https://github.com/docker-library/php/blob/master/8.3/alpine3.20/zts/Dockerfile#L53-L55
ENV CGO_CFLAGS="-DFRANKENPHP_VERSION=$FRANKENPHP_VERSION $PHP_CFLAGS"
ENV CGO_CPPFLAGS=$PHP_CPPFLAGS
ENV CGO_LDFLAGS="-lssl -lcrypto -lreadline -largon2 -lcurl -lonig -lz $PHP_LDFLAGS"

WORKDIR /go/src/app/caddy/frankenphp
RUN GOBIN=/usr/local/bin \
		../../go.sh install -ldflags "-w -s -extldflags '-Wl,-z,stack-size=0x80000' -X 'github.com/caddyserver/caddy/v2.CustomVersion=FrankenPHP $FRANKENPHP_VERSION PHP $PHP_VERSION Caddy'" -buildvcs=true && \
	setcap cap_net_bind_service=+ep /usr/local/bin/frankenphp && \
	([ -z "${NO_COMPRESS}" ] && upx --best /usr/local/bin/frankenphp || true) && \
	frankenphp version && \
	frankenphp build-info

WORKDIR /go/src/app


FROM common AS runner

ENV GODEBUG=cgocheck=0

# copy watcher shared library (libgcc and libstdc++ are needed for the watcher)
COPY --from=builder /usr/local/lib/libwatcher* /usr/local/lib/
RUN apk add --no-cache libstdc++ && \
	ldconfig /usr/local/lib

COPY --from=builder /usr/local/bin/frankenphp /usr/local/bin/frankenphp
RUN setcap cap_net_bind_service=+ep /usr/local/bin/frankenphp && \
	frankenphp version && \
	frankenphp build-info
