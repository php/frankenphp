#!/bin/sh
# Runs the go command with the proper Go and cgo flags.

GOFLAGS="$GOFLAGS -tags=nobadger,nomysql,nopgx" \
	CGO_CFLAGS="$CGO_CFLAGS $(php-config --includes) $(sh "$(dirname "$0")/mtls-cflags.sh")" \
	CGO_LDFLAGS="$CGO_LDFLAGS $(php-config --ldflags) $(php-config --libs)" \
	go "$@"
