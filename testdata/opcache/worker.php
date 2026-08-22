<?php

// Keeps references into opcache shared memory alive across requests: the
// literal is an interned string and the const array is IS_ARRAY_IMMUTABLE.
// Both live in memory that opcache rewinds on a restart, so if a restart is
// performed while this worker runs, the process dies.
const OPCACHE_CANARY = ['a' => 1, 'b' => [2, 3], 'c' => 'literal'];

$str = 'opcache_canary_interned_string';
$arr = OPCACHE_CANARY;

// Fingerprints are plain integers on the thread heap, so a rewind cannot
// touch them.
$expLen = strlen($str);
$expCrc = crc32($str);
$expArr = crc32(serialize($arr));

$handler = static function () use (&$str, &$arr, $expLen, $expCrc, $expArr) {
    if (strlen($str) !== $expLen || crc32($str) !== $expCrc || crc32(serialize($arr)) !== $expArr) {
        echo 'CORRUPT';

        return;
    }

    echo 'OK';
};

for ($running = true; $running;) {
    $running = frankenphp_handle_request($handler);
}
