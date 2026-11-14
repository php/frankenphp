<?php

require_once __DIR__.'/_executor.php';

// Tells whether the calling thread blocks SIGURG, the signal the Go runtime
// preempts goroutines with: bit 22 of the mask the kernel reports.
return function () {
    preg_match('/^SigBlk:\s*([0-9a-f]+)/m', file_get_contents('/proc/thread-self/status'), $mask);

    echo hexdec($mask[1]) >> 22 & 1 ? 'blocked' : 'not blocked';
};
