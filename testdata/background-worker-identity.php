<?php

frankenphp_handle_request(function () {
    try {
        $a = frankenphp_worker_get_vars('test-worker', 5.0);
        $b = frankenphp_worker_get_vars('test-worker', 5.0);
        echo ($a === $b) ? 'IDENTICAL' : 'DIFFERENT';
    } catch (\Throwable $e) {
        echo 'ERROR:' . $e->getMessage();
    }
});
