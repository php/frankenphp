<?php

frankenphp_handle_request(function () {
    try {
        $vars = frankenphp_get_vars('crash-worker', 5.0);
        echo $vars['WORKER_STATUS'] ?? 'MISSING_KEY';
    } catch (\Throwable $e) {
        echo 'ERROR:' . $e->getMessage();
    }
});
