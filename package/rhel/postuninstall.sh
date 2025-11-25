#!/bin/bash

if [ "$1" -ge 1 ] && [ -x "/usr/lib/systemd/systemd-update-helper" ]; then
	# Package upgrade, not uninstall
	/usr/lib/systemd/systemd-update-helper mark-restart-system-units frankenphp.service || :
fi

if [ "$1" -eq 0 ]; then
	if [ -x /usr/sbin/getsebool ]; then
		# connect to ACME endpoint to request certificates
		setsebool -P httpd_can_network_connect off
	fi
	if [ -x /usr/sbin/semanage ]; then
		# file contexts
		semanage fcontext --delete --type httpd_exec_t '/usr/bin/frankenphp' 2>/dev/null || :
		semanage fcontext --delete --type httpd_sys_content_t '/usr/share/frankenphp(/.*)?' 2>/dev/null || :
		semanage fcontext --delete --type httpd_config_t '/etc/frankenphp(/.*)?' 2>/dev/null || :
		semanage fcontext --delete --type httpd_sys_rw_content_t '/var/lib/frankenphp(/.*)?' 2>/dev/null || :
		# QUIC
		semanage port --delete --type http_port_t --proto udp 80 2>/dev/null || :
		semanage port --delete --type http_port_t --proto udp 443 2>/dev/null || :
		# admin endpoint
		semanage port --delete --type http_port_t --proto tcp 2019 2>/dev/null || :
	fi
fi
