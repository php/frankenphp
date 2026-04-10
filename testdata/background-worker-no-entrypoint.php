<?php

frankenphp_handle_request(function () {
    try {
        $vars = frankenphp_worker_get_vars('should-fail', 1.0);
        echo 'no error';
    } catch (\RuntimeException $e) {
        echo $e->getMessage();
    }
});
