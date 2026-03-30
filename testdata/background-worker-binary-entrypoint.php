<?php

require __DIR__ . '/background-worker-helper.php';

frankenphp_set_vars([
    'BINARY_TEST' => "hello\x00world",
    'UTF8_TEST' => "héllo wörld 🚀",
    'EMPTY_VAL' => "",
]);

while (!background_worker_should_stop(30)) {
}
