<?php
$cookieCount = count($_COOKIE);
$ua = $_SERVER['HTTP_USER_AGENT'] ?? '';
$accept = $_SERVER['HTTP_ACCEPT'] ?? '';
$auth = $_SERVER['HTTP_AUTHORIZATION'] ?? '';

header('Content-Type: text/plain; charset=utf-8');
header('Cache-Control: private, no-cache, must-revalidate, max-age=0');
header('Pragma: no-cache');
header('X-Frame-Options: DENY');
header('X-Content-Type-Options: nosniff');
header('X-XSS-Protection: 1; mode=block');
header('Strict-Transport-Security: max-age=31536000; includeSubDomains');
header('Referrer-Policy: strict-origin-when-cross-origin');
header('Permissions-Policy: camera=(), microphone=(), geolocation=()');
header('Set-Cookie: trace=' . substr(md5((string)microtime(true)), 0, 16) . '; Path=/; HttpOnly; SameSite=Lax');
header('X-Request-Id: ' . substr(md5((string)microtime(true)), 0, 16));

echo "cookies=$cookieCount ua_len=" . strlen($ua) . " auth_len=" . strlen($auth) . "\n";
