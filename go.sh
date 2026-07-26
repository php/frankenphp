#!/bin/sh
# Runs the go command with the proper Go and cgo flags.

PHP_CONFIG=${PHP_CONFIG:-php-config}

GOFLAGS="$GOFLAGS -tags=nobadger,nomysql,nopgx" \
	CGO_CFLAGS="$CGO_CFLAGS $(${PHP_CONFIG} --includes) $(sh "$(dirname "$0")/mtls-cflags.sh")" \
	CGO_LDFLAGS="$CGO_LDFLAGS $(${PHP_CONFIG} --ldflags) $(${PHP_CONFIG} --libs)" \
	go "$@"
