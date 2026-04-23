<?php

// Calls get_vars twice within one request. The cache guarantees pointer
// identity: the two zvals should be === (same HashTable pointer) because
// the underlying worker hasn't published a new version in between.
// ensure() waits for the bg worker's first set_vars so eager-start races
// don't surface before the cache path is exercised.
frankenphp_ensure_background_worker('cache-worker');
$first = frankenphp_get_vars('cache-worker');
$second = frankenphp_get_vars('cache-worker');

echo 'first=', isset($first['marker']) ? $first['marker'] : 'MISSING', "\n";
echo 'second=', isset($second['marker']) ? $second['marker'] : 'MISSING', "\n";
echo 'identical=', ($first === $second) ? 'true' : 'false', "\n";
