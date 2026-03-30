<?php

// Background worker for Caddy integration test
frankenphp_set_vars(['CADDY_TEST' => 'hello from background worker']);

$stream = frankenphp_get_worker_handle();
while (true) {
    $r = [$stream];
    $w = $e = [];
    if (false === @stream_select($r, $w, $e, 30)) { break; }
    if ($r && false === fgets($stream)) { break; } // EOF = stop
}
