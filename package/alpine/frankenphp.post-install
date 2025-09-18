#!/bin/sh

if ! getent group frankenphp >/dev/null; then
	addgroup -S frankenphp
fi

if ! getent passwd frankenphp >/dev/null; then
	adduser -S -h /var/lib/frankenphp -s /sbin/nologin -G frankenphp -g "FrankenPHP web server" frankenphp
fi

chown -R frankenphp:frankenphp /var/lib/frankenphp
chmod 755 /var/lib/frankenphp

# allow binding to privileged ports
if command -v setcap >/dev/null 2>&1; then
	setcap cap_net_bind_service=+ep /usr/bin/frankenphp || true
fi

# trust FrankenPHP certificates
if [ -x /usr/bin/frankenphp ]; then
	HOME=/var/lib/frankenphp /usr/bin/frankenphp trust || true
fi

if command -v rc-update >/dev/null 2>&1; then
	rc-update add frankenphp default
	rc-service frankenphp start
fi

exit 0
