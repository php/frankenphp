<?php

require __DIR__ . '/background-worker-helper.php';

$name = $_SERVER['FRANKENPHP_WORKER_NAME'] ?? $_SERVER['argv'][1] ?? 'unknown';

frankenphp_set_vars(['NAME_' . strtoupper(str_replace('-', '_', $name)) => $name]);

while (!background_worker_should_stop(30)) {
}
