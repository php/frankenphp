<?php

require_once __DIR__.'/../_executor.php';

return function () {
    $count = $_GET['count'] ?? 0;
    for ($i = 0; $i < $count; $i++) {
        frankenphp_dispatch_task("task$i");
    }
    echo "dispatched $count tasks\n";
};
