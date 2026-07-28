<?php
$codes = [200, 201, 202, 204, 301, 302, 304, 400, 401, 403, 404, 410, 418, 500, 502, 503];
$code = $codes[mt_rand(0, count($codes) - 1)];

http_response_code($code);
header('Content-Type: text/plain; charset=utf-8');
header('X-Status: ' . $code);

if ($code === 204 || $code === 304) {
    return;
}
echo "code=$code\n";
