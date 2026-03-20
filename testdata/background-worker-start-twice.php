<?php

frankenphp_handle_request(function () {
    try {
        $vars = frankenphp_worker_get_vars('dedup-worker', 5.0);
        echo $vars['WORKER_NAME'] ?? 'MISSING_KEY';
    } catch (\Throwable $e) {
        echo 'ERROR:' . $e->getMessage();
    }
});
