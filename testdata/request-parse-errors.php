<?php

require_once __DIR__ . '/_executor.php';

// Throwing handler mimicking symfony/error-handler: before the fix, parse
// warnings raised between requests escaped as an uncaught ErrorException and
// killed the worker script (https://github.com/php/frankenphp/issues/2631).
set_error_handler(function ($severity, $message, $file, $line) {
    throw new \ErrorException($message, 0, $severity, $file, $line);
});

$requests = 0;

return function () use (&$requests) {
    $lastError = error_get_last();
    if ($lastError && $lastError['type'] === E_WARNING) {
        http_response_code(400);
        echo $lastError['message'];
    }

    ++$requests;
    echo "requests:{$requests}\n";
};
