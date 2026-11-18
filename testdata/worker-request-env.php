<?php

require_once __DIR__.'/_executor.php';

return function () {
    if (isset($_GET['put'])) {
        putenv("PUT={$_GET['put']}");
    }

    echo json_encode([
        'server' => $_SERVER['SECRET'] ?? false,
        'secret' => getenv('SECRET'),
        'other' => getenv('OTHER'),
        'worker' => getenv('FOO'),
        'put' => getenv('PUT'),
        'all' => isset(getenv()['SECRET']),
        // $_ENV is created when the first file using it is compiled
        'env' => require __DIR__.'/env/read-env.php',
    ]);
};
