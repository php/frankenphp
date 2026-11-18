<?php

require_once __DIR__.'/../_executor.php';

return function () {
    if (isset($_GET['put'])) {
        register_shutdown_function(static fn () => putenv('FRANKENPHP_LATE_PUTENV=1'));
    }

    var_export(getenv('FRANKENPHP_LATE_PUTENV'));
};
