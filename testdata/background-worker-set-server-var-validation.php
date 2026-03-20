<?php

frankenphp_handle_request(function () {
    $results = [];

    // set_vars from non-background-worker context should throw RuntimeException
    try {
        frankenphp_worker_set_vars(['KEY' => 'val']);
        $results[] = 'NON_BACKGROUND:no_error';
    } catch (\RuntimeException $e) {
        $results[] = 'NON_BACKGROUND:blocked';
    }

    // get_signaling_stream from non-background-worker context should throw
    try {
        frankenphp_worker_get_signaling_stream();
        $results[] = 'STREAM_NON_BACKGROUND:no_error';
    } catch (\RuntimeException $e) {
        $results[] = 'STREAM_NON_BACKGROUND:blocked';
    }

    echo implode("\n", $results);
});
