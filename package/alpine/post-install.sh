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

port_in_use() {
	port_hex=$(printf '%04X' $1)
	grep -q ":${port_hex} " /proc/net/tcp /proc/net/tcp6 2>/dev/null
}

# trust FrankenPHP certificates
if [ -x /usr/bin/frankenphp ]; then
	if ! port_in_use 2019; then
		HOME=/var/lib/frankenphp /usr/bin/frankenphp run >/dev/null 2>&1 &
		FRANKENPHP_PID=$!
		sleep 2
		HOME=/var/lib/frankenphp /usr/bin/frankenphp trust || true
		kill -TERM $FRANKENPHP_PID 2>/dev/null || true
	fi
fi

if command -v rc-update >/dev/null 2>&1; then
	rc-update add frankenphp default
	rc-service frankenphp start
fi

exit 0
