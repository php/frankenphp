<?php

$stream = frankenphp_get_worker_handle();

frankenphp_set_vars([
    'STREAM_TYPE' => get_resource_type($stream),
]);

$r = [$stream];
$w = $e = [];
stream_select($r, $w, $e, 30);

$signal = fgets($stream);

frankenphp_set_vars([
    'STREAM_TYPE' => get_resource_type($stream),
    'SIGNAL' => $signal,
]);
