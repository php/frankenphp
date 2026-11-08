<?php

// de-DE is the name on Windows
$workerLocale = setlocale(LC_NUMERIC, ['C.UTF-8', 'de-DE']);

while (frankenphp_handle_request(function () use ($workerLocale) {
    echo $workerLocale, '|', setlocale(LC_NUMERIC, '0');
    setlocale(LC_NUMERIC, 'C');
})) {
}
