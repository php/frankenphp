<?php

frankenphp_handle_request(function () {
    // First call: publish vars from HTTP worker
    frankenphp_set_vars(['REQUEST_COUNT' => '1', 'SOURCE' => 'http']);

    // Read back via get_vars(null)
    $vars = frankenphp_get_vars(null);
    echo $vars['SOURCE'] . ':' . $vars['REQUEST_COUNT'];
});
