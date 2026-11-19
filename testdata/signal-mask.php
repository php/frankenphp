<?php

require_once __DIR__.'/_executor.php';

// Prints the signal mask of the calling thread, as hex
return function () {
    preg_match('/^SigBlk:\s*([0-9a-f]+)/m', file_get_contents('/proc/thread-self/status'), $mask);

    echo $mask[1];
};
