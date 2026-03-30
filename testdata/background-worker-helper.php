<?php

function background_worker_should_stop(float $timeout = 0): bool
{
    static $signalingStream;
    $signalingStream ??= frankenphp_get_worker_handle();
    $s = (int) $timeout;

    return match (@stream_select(...[[$signalingStream], [], [], $s, (int) (($timeout - $s) * 1e6)])) {
        0 => false, // timeout
        false => true, // error = stop
        default => false === fgets($signalingStream), // EOF = stop
    };
}
