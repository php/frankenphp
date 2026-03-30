<?php

frankenphp_handle_request(function () {
    $results = [];

    // set_vars from non-background-worker context should throw RuntimeException
    try {
        frankenphp_set_vars(['KEY' => 'val']);
        $results[] = 'NON_BACKGROUND:no_error';
    } catch (\RuntimeException $e) {
        $results[] = 'NON_BACKGROUND:blocked';
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
