<?php

// HTTP fixture exercising input validation paths of the batch ensure().
// The query string selects which scenario to trigger; the script catches
// the resulting throwable and echoes its class so the test can assert the
// expected error type without parsing PHP's error log.
$mode = $_GET['mode'] ?? '';

try {
    switch ($mode) {
        case 'empty':
            frankenphp_ensure_background_worker([]);
            break;
        case 'nonstring':
            frankenphp_ensure_background_worker(['ok-name', 42]);
            break;
        case 'duplicate':
            frankenphp_ensure_background_worker(['dup', 'dup']);
            break;
        case 'empty-string':
            frankenphp_ensure_background_worker(['', 'other']);
            break;
        default:
            echo "no-mode\n";
            return;
    }
    echo "no-throw\n";
} catch (\Throwable $e) {
    echo get_class($e), ': ', $e->getMessage(), "\n";
}
