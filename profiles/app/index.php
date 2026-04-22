<?php

$handler = static function (): void {
    $s = $_GET['s'] ?? '';
    if (!preg_match('/^[a-z_]+$/', $s)) {
        http_response_code(404);
        return;
    }
    $file = __DIR__ . DIRECTORY_SEPARATOR . $s . '.php';
    if (!is_file($file)) {
        http_response_code(404);
        return;
    }
    require $file;
};

if (isset($_SERVER['FRANKENPHP_WORKER'])) {
    while (frankenphp_handle_request($handler)) {}
} else {
    $handler();
}
