<?php

require __DIR__ . '/background-worker-helper.php';

$name = $_SERVER['FRANKENPHP_WORKER_NAME'] ?? $_SERVER['argv'][1] ?? 'unknown';
$marker = sys_get_temp_dir() . '/background-worker-restart-' . md5($name);
$runningMarker = sys_get_temp_dir() . '/background-worker-restart-running-' . md5($name);
$generation = file_exists($marker) ? ((int) file_get_contents($marker)) + 1 : 1;

file_put_contents($marker, (string) $generation);
file_put_contents($runningMarker, (string) getmypid());
frankenphp_worker_set_vars(['GENERATION' => (string) $generation]);

try {
    while (!background_worker_should_stop(30)) {
    }
} finally {
    @unlink($runningMarker);
}
