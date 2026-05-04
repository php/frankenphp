<?php

// HTTP script that batches three ensures in one call and reads each.
frankenphp_ensure_background_worker(['worker-a', 'worker-b', 'worker-c'], 5.0);

foreach (['worker-a', 'worker-b', 'worker-c'] as $name) {
    $vars = frankenphp_get_vars($name);
    echo $name, '=', $vars['FRANKENPHP_WORKER'] ?? 'MISSING', "\n";
}
