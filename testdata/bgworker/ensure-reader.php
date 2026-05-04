<?php

// HTTP script that ensures the bg worker first, then reads its vars.
// Used to exercise the lazy-start path via frankenphp_ensure_background_worker.
$name = $_GET['name'] ?? 'bg-lazy';
frankenphp_ensure_background_worker($name, 5.0);
$vars = frankenphp_get_vars($name);
echo 'name=', $vars['FRANKENPHP_WORKER'] ?? 'MISSING', "\n";
echo 'count=', $vars['count'] ?? 'MISSING', "\n";
