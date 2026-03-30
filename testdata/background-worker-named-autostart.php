<?php

// Auto-started named background worker
// Publishes its name so HTTP workers can verify it started

require __DIR__ . '/background-worker-helper.php';

frankenphp_set_vars([
    'WORKER_NAME' => $_SERVER['FRANKENPHP_WORKER_NAME'] ?? 'unknown',
    'AUTOSTART' => true,
]);

// Wait for stop signal
$stream = frankenphp_get_worker_handle();
while (true) {
    $r = [$stream]; $w = $e = [];
    if (false === @stream_select($r, $w, $e, 30)) { break; }
    if ($r && false === fgets($stream)) { break; } // EOF = stop
}
