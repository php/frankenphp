<?php

// Background worker for Caddy integration test
frankenphp_worker_set_vars(['CADDY_TEST' => 'hello from background worker']);

$stream = frankenphp_worker_get_signaling_stream();
while (true) {
    $r = [$stream];
    $w = $e = [];
    if (false === @stream_select($r, $w, $e, 30)) { break; }
    if ($r && "stop\n" === fgets($stream)) { break; }
}
