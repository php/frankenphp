<?php

require_once __DIR__ . '/../_executor.php';

return function () {
    // Variables declared via the env subdirective in the php(_server) directive
    // (or via WithRequestEnv) must be exposed to both $_SERVER and getenv().
    // See https://github.com/php/frankenphp/issues/1674
    echo "getenv=" . var_export(getenv('FRANKENPHP_TEST_PHP_SERVER_ENV_IN_GETENV'), true) . "\n";
    echo "server=" . var_export($_SERVER['FRANKENPHP_TEST_PHP_SERVER_ENV_IN_GETENV'] ?? null, true) . "\n";
    echo "env=" . var_export($_ENV['FRANKENPHP_TEST_PHP_SERVER_ENV_IN_GETENV'] ?? null, true) . "\n";
};
