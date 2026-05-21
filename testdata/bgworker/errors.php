<?php

// Exercise error paths on get_vars / set_vars that don't depend on a
// running worker thread.

try {
    frankenphp_get_vars('does-not-exist');
    echo "FAIL missing worker\n";
} catch (RuntimeException $e) {
    echo "OK missing: ", $e->getMessage(), "\n";
}

try {
    frankenphp_set_vars(['foo' => 'bar']);
    echo "FAIL set_vars from non-background\n";
} catch (RuntimeException $e) {
    echo "OK reject-non-bg: ", $e->getMessage(), "\n";
}

try {
    frankenphp_get_worker_handle();
    echo "FAIL get_worker_handle from non-background\n";
} catch (RuntimeException $e) {
    echo "OK reject-handle: ", $e->getMessage(), "\n";
}
