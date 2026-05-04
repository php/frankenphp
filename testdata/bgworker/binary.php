<?php

// Long-lived bg worker: disable PHP max_execution_time so the 30s default
// cannot interrupt the stream_select park. The C side calls
// zend_unset_timeout() too, but the belt-and-suspenders here covers PHP
// builds where that path does not fully disarm the timer.
set_time_limit(0);

// Binary-safe fixture: publishes values with embedded NULs, multibyte
// UTF-8, and an empty string so the reader can confirm the deep-copy
// path preserves bytes verbatim (not truncated at the first \0, not
// mangled through C string conversions).
frankenphp_set_vars([
    'BINARY' => "hello\x00world",
    'UTF8' => "héllo wörld 🚀",
    'EMPTY' => "",
]);

$stream = frankenphp_get_worker_handle();
$read = [$stream];
$write = null;
$except = null;
stream_select($read, $write, $except, null);
