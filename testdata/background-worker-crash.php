<?php

require __DIR__ . '/background-worker-helper.php';

$marker = sys_get_temp_dir() . '/background-worker-crash-' . getmypid();
$restarted = file_exists($marker);

if (!$restarted) {
    file_put_contents($marker, '1');
    exit(1);
}

frankenphp_worker_set_vars(['WORKER_STATUS' => 'restarted']);

while (!background_worker_should_stop(30)) {
}

@unlink($marker);
