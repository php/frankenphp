#!/bin/sh

if command -v rc-service >/dev/null 2>&1; then
	rc-service frankenphp stop || true
fi

if command -v rc-update >/dev/null 2>&1; then
	rc-update del frankenphp default || true
fi

exit 0
