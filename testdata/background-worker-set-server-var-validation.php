<?php

frankenphp_handle_request(function () {
    $results = [];

    // set_worker_vars from HTTP worker context should succeed
    try {
        frankenphp_set_vars(['KEY' => 'val']);
        $results[] = 'HTTP_SET_VARS:allowed';
    } catch (\RuntimeException $e) {
        $results[] = 'HTTP_SET_VARS:blocked';
    }

    // get_worker_handle from non-background-worker context should throw
    try {
        frankenphp_get_worker_handle();
        $results[] = 'STREAM_NON_BACKGROUND:no_error';
    } catch (\RuntimeException $e) {
        $results[] = 'STREAM_NON_BACKGROUND:blocked';
    }

    echo implode("\n", $results);
});
