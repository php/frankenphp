<?php

enum TestStatus {
    case Active;
    case Inactive;
}

frankenphp_handle_request(function () {
    // Retry until we see the final snapshot with RESULTS
    for ($i = 0; $i < 100; $i++) {
        $vars = frankenphp_worker_get_vars('type-validation');
        if (isset($vars['RESULTS'])) break;
        usleep(10_000);
    }

    $results = $vars['RESULTS'] ?? 'NOT_SET';

    if (isset($vars['status'])) {
        $results .= $vars['status'] === TestStatus::Active ? ',ENUM_RESTORED:match' : ',ENUM_RESTORED:mismatch(' . get_debug_type($vars['status']) . ')';
    }

    echo $results;
});
