<?php

frankenphp_handle_request(function () {
    $vars = frankenphp_worker_get_vars('stop-fd-test');
    echo $vars['STREAM_TYPE'] ?? 'NOT_SET';
});
