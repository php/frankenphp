<?php

$fn = require $_SERVER['SCRIPT_FILENAME'];
if ('1' !== ($_SERVER['FRANKENPHP_WORKER'] ?? null)) {
    $fn();
    exit(0);
}

while (frankenphp_handle_request($fn)) {}

exit(0);
