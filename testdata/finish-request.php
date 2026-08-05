<?php

require_once __DIR__.'/_executor.php';

return function () {
    echo 'This is output '.($_GET['i'] ?? '')."\n";

    frankenphp_finish_request();

    echo 'This is not';

    // A write after the request is finished must not be treated as an
    // aborted connection: with the default ignore_user_abort=Off, that
    // would bail out the script here and skip this log line.
    error_log('reached after finish_request '.($_GET['i'] ?? ''));
};
