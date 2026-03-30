<?php

frankenphp_handle_request(function () {
    $name = $_GET['name'] ?? 'restart-worker';
    $vars = frankenphp_get_vars($name, 5.0);
    echo $vars['GENERATION'] ?? 'NOT_SET';
});
