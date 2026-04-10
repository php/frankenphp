<?php

require __DIR__ . '/background-worker-helper.php';

$argv = $_SERVER['argv'] ?? [];
array_shift($argv);
$name = $argv[0] ?? 'unknown';

frankenphp_worker_set_vars(['WORKER_NAME' => $name]);

while (!background_worker_should_stop(30)) {
}
