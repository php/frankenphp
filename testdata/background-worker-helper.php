<?php

function background_worker_should_stop(float $timeout = 0): bool
{
    static $signalingStream;
    $signalingStream ??= frankenphp_worker_get_signaling_stream();
    $s = (int) $timeout;

    return match (@stream_select(...[[$signalingStream], [], [], $s, (int) (($timeout - $s) * 1e6)])) {
        0 => false, // timeout
        false => true, // error (pipe closed) = stop
        default => "stop\n" === fgets($signalingStream),
    };
}
