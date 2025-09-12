#!/bin/bash

# build and run FrankenPHP with the k6.Caddyfile
cd /go/src/app/caddy/frankenphp &&
	../../go.sh build --buildvcs=false &&
	cd ../../testdata/performance &&
	/go/src/app/caddy/frankenphp/frankenphp run -c k6.Caddyfile
