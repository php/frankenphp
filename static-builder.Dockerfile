# syntax=docker/dockerfile:1
#checkov:skip=CKV_DOCKER_2
#checkov:skip=CKV_DOCKER_3
#checkov:skip=CKV_DOCKER_7

# Toolchain image for the static builds, shared by the musl and the glibc flavor.
# It ships glibc 2.28 (RHEL 8) and GCC 16.
#
# The build happens when the container *runs*, not while the image is built, so
# that the static-php-cli logs can be copied out of the container when it fails.
#
#     docker run --name static-builder-gnu dunglas/frankenphp:static-builder-gnu
#     docker cp static-builder-gnu:/go/src/app/dist/frankenphp-linux-x86_64 frankenphp
#
# The musl flavor links with zig, which opens a lot of file descriptors when many
# extensions are built statically; give it `--ulimit nofile=8192:8192`.
#
# Every variable build-static.sh understands (PHP_VERSION, PHP_EXTENSIONS,
# XCADDY_ARGS, EMBED, MIMALLOC, ...) is passed with `docker run -e`; there is no
# need to rebuild the image to change them.
FROM ghcr.io/static-php/packages-builder-rhel-8:latest

LABEL org.opencontainers.image.title=FrankenPHP
LABEL org.opencontainers.image.description="The modern PHP app server"
LABEL org.opencontainers.image.url=https://frankenphp.dev
LABEL org.opencontainers.image.source=https://github.com/php/frankenphp
LABEL org.opencontainers.image.licenses=MIT
LABEL org.opencontainers.image.vendor="Kévin Dunglas"

ARG FRANKENPHP_VERSION=''
ENV FRANKENPHP_VERSION=${FRANKENPHP_VERSION}

ARG LIBC=gnu
ENV LIBC=${LIBC}

# static-php-cli downloads the Go toolchain it needs, don't let go.mod fetch another one.
ENV GOTOOLCHAIN=local

# we build with OPENSSLDIR=/etc/ssl which doesn't exist in RHEL
RUN ln -s ../pki/tls/cert.pem /etc/ssl/cert.pem

WORKDIR /go/src/app
COPY --link . ./

CMD ["./build-static.sh"]
