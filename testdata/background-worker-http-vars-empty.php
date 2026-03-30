<?php

frankenphp_handle_request(function () {
    // get_vars(null) without any prior set_vars should return empty array
    $vars = frankenphp_get_vars(null);
    echo is_array($vars) ? 'array:' . count($vars) : 'not_array';
});
