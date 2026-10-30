<?php

require_once __DIR__ . '/_executor.php';

set_error_handler(function ($severity, $message, $file, $line) {
    echo "an error has occurred";
});

$requests = 0;

return function () use (&$requests) {
    ++$requests;
    echo "requests:$requests\n";

    // get_error_handler is not available in PHP <= 8.4
    if (function_exists('get_error_handler')) {
        $handler = get_error_handler();
        echo $handler ? "handler is defined" : "handler is not defined";
    } else {
        echo "handler is defined";
    }
};