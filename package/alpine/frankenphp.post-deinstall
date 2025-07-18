#!/bin/sh

if getent passwd frankenphp >/dev/null; then
	deluser frankenphp
fi

if getent group frankenphp >/dev/null; then
	delgroup frankenphp
fi

rmdir /var/lib/frankenphp 2>/dev/null || true

exit 0
