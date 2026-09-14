<?php

$fn = require $_SERVER['SCRIPT_FILENAME'];
if (!isset($_SERVER['FRANKENPHP_WORKER'])) {
    $fn();
    exit(0);
}

while (frankenphp_handle_request($fn)) {}

exit(0);
