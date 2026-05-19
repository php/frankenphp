<?php

// HTTP script that runs NOT inside a worker (non-worker mode) and calls
// ensure() at request time. Exercises the tolerant runtime-mode path.
frankenphp_ensure_background_worker('bg-lazy', 5.0);
$vars = frankenphp_get_vars('bg-lazy');
echo 'ensured-name=', $vars['FRANKENPHP_WORKER'] ?? 'MISSING', "\n";
