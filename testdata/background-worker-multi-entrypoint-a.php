<?php

require __DIR__ . '/background-worker-helper.php';

frankenphp_set_vars(['SOURCE' => 'entrypoint-a', 'NAME' => $_SERVER['FRANKENPHP_WORKER_NAME'] ?? 'unknown']);

while (!background_worker_should_stop(30)) {
}
