<?php

// Boots successfully, then terminates with a non-zero exit status while
// handling its second request, simulating a worker crash after boot.
$requests = 0;
$handler = static function () use (&$requests) {
    if (++$requests >= 2) {
        exit(1);
    }

    echo 'ok';
};

while (frankenphp_handle_request($handler)) {
}
