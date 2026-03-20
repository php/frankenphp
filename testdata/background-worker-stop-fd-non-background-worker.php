<?php

frankenphp_handle_request(function () {
    try {
        frankenphp_worker_get_signaling_stream();
        echo 'no_error';
    } catch (\RuntimeException $e) {
        echo 'thrown';
    }
});
