<?php

require_once __DIR__.'/../_executor.php';

return function () {
    $taskCount = $_GET['count'] ?? 0;
    $workerName = $_GET['worker'] ?? null;
    for ($i = 0; $i < $taskCount; $i++) {
        frankenphp_dispatch_task("task$i", $workerName);
    }
    echo "dispatched $taskCount tasks\n";
};
