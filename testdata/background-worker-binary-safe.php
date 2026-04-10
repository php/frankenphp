<?php

frankenphp_handle_request(function () {
    try {
        $vars = frankenphp_worker_get_vars('binary-worker', 5.0);
    } catch (\Throwable $e) {
        echo 'ERROR:' . $e->getMessage();
        return;
    }

    $results = [];

    $bin = $vars['BINARY_TEST'] ?? 'NOT_SET';
    $results[] = 'BINARY_LEN:' . strlen($bin);
    $results[] = 'BINARY_CONTENT:' . bin2hex($bin);

    $results[] = 'UTF8:' . ($vars['UTF8_TEST'] ?? 'NOT_SET');

    $results[] = 'EMPTY_EXISTS:' . (array_key_exists('EMPTY_VAL', $vars) ? 'yes' : 'no');
    $results[] = 'EMPTY_LEN:' . strlen($vars['EMPTY_VAL'] ?? 'NOT_SET');

    echo implode("\n", $results);
});
