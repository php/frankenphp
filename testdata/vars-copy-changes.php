<?php

// Reads a vars namespace its publisher replaces every few milliseconds, until
// a read returns another tick than the first one: a copy kept never outlives
// its snapshot.
$first = frankenphp_get_vars('ticker')['tick'];
$deadline = microtime(true) + 5;
do {
    usleep(5000);
    $next = frankenphp_get_vars('ticker')['tick'];
} while ($next === $first && microtime(true) < $deadline);

echo $first === $next ? "stuck at $first" : 'changed';
