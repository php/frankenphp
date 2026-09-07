<?php

require_once __DIR__ . '/../_executor.php';

return function () {
    echo "before=" . var_export(getenv('FRANKENPHP_PREPARED'), true) . "\n";
    putenv('FRANKENPHP_PUT=put_value');
    echo "prepared=" . var_export(getenv('FRANKENPHP_PREPARED'), true) . "\n";
    echo "put=" . var_export(getenv('FRANKENPHP_PUT'), true) . "\n";
};
