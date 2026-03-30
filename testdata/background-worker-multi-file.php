<?php

frankenphp_handle_request(function () {
    $a = frankenphp_get_vars('worker-from-a');
    $b = frankenphp_get_vars('worker-from-b');

    echo $a['SOURCE'] . ':' . $a['NAME'] . ',' . $b['SOURCE'] . ':' . $b['NAME'];
});
