<?php

frankenphp_handle_request(function () {
    frankenphp_set_vars(['KEY' => 'val']);

    // Two calls with null in the same request should return identical arrays
    $a = frankenphp_get_vars(null);
    $b = frankenphp_get_vars(null);
    echo $a === $b ? 'IDENTICAL' : 'DIFFERENT';
});
