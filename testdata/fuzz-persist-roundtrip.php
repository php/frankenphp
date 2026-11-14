<?php

// Exercises zval.h's persistent_zval_persist/_to_request/_free recursive
// tree walk (FrankenPHP's own worker-state mechanism, not php-src) via the
// FRANKENPHP_TEST-only frankenphp_test_persist_roundtrip hook. Nesting
// depth and width come from the query string so fuzzing controls the shape.

$rt = 'frankenphp_test_persist_roundtrip';
if (!function_exists($rt)) {
    echo 'SKIP';
    return;
}

$depth = max(0, min((int) ($_GET['depth'] ?? 0), 5000));
$width = max(1, min((int) ($_GET['width'] ?? 1), 4));

function frankenphp_fuzz_build_nested(int $depth, int $width): mixed
{
    if ($depth <= 0) {
        return 'leaf';
    }

    // Only the first slot recurses; the rest are cheap leaves. This keeps
    // the total node count linear in depth * width instead of width**depth
    // (a naive every-slot-recurses builder would blow past available
    // memory well before depth=50 at width=2), while still stressing
    // exactly the same nesting depth per recursive C call.
    $arr = ['leaf'];
    for ($i = 1; $i < $width; $i++) {
        $arr[$i] = 'leaf';
    }
    $arr[0] = frankenphp_fuzz_build_nested($depth - 1, $width);

    return $arr;
}

$value = frankenphp_fuzz_build_nested($depth, $width);

try {
    echo $rt($value) === $value ? 'OK' : 'MISMATCH';
} catch (\Throwable $e) {
    echo 'THROWN:'.get_class($e);
}
