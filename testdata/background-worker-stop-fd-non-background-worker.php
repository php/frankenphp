<?php

frankenphp_handle_request(function () {
    try {
        frankenphp_get_worker_handle();
        echo 'no_error';
    } catch (\RuntimeException $e) {
        echo 'thrown';
    }
});
