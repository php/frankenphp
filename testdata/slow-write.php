<?php

// Write continuously so a stalled reader eventually blocks the underlying
// socket write. Capped at 10s so an unpatched build fails the test instead of
// hanging it forever.
ob_implicit_flush(true);
$start = microtime(true);
while (microtime(true) - $start < 10) {
    echo str_repeat('x', 65536);
}
echo 'did not time out';
