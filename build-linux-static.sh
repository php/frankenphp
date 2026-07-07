#!/bin/bash

set -o errexit
set -x

# The following environment variables can be passed to docker build and to the build-static.sh script to customize the static build:
  #
  #    FRANKENPHP_VERSION: the version of FrankenPHP to use
  #    PHP_VERSION: the version of PHP to use
  #    PHP_EXTENSIONS: the PHP extensions to build (list of supported extensions)
  #    PHP_EXTENSION_LIBS: extra libraries to build that add features to the extensions
  #    XCADDY_ARGS: arguments to pass to xcaddy, for instance to add extra Caddy modules
  #    EMBED: path of the PHP application to embed in the binary
  #    CLEAN: when set, libphp and all its dependencies are built from scratch (no cache)
  #    COMPRESS: when set to 1, pack the resulting binary with UPX (Linux only; ignored when DEBUG_SYMBOLS is set)
  #    DEBUG_SYMBOLS: when set, debug-symbols will not be stripped and will be added to the binary
  #    MIMALLOC: (experimental, Linux-only) replace musl’s mallocng by mimalloc for improved performance. We only recommend using this for musl targeting builds, for glibc prefer disabling this option and using LD_PRELOAD when you run your binary instead.
  #    RELEASE: (maintainers only) when set, the resulting binary will be uploaded on GitHub

# PHP extensions from composer.json
PHP_EXTENSIONS="bcmath,ctype,curl,dom,fileinfo,filter,gd,hash,intl,json,mbstring,openssl,pcre,pdo,session,tokenizer,xml"
PHP_VERSION=8.4

# Build the static Linux binary using the musl static builder
docker buildx bake --load \
    --set "*.args.PHP_EXTENSIONS=${PHP_EXTENSIONS}" \
    --set "*.args.PHP_VERSION=${PHP_VERSION}" \
    static-builder-musl

# Copy the binary out of the container
docker buildx bake --load static-builder-gnu
docker cp $(docker create --name static-builder-gnu dunglas/frankenphp:static-builder-gnu):/go/src/app/dist/frankenphp-linux-$(uname -m) frankenphp
docker rm static-builder-gnu
