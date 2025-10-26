<?php

require_once __DIR__.'/../_executor.php';

return function () {
    $taskCount = $_GET['count'] ?? 0;
    $workerName = $_GET['worker'] ?? '';
    for ($i = 0; $i < $taskCount; $i++) {
        frankenphp_send_request("task$i", $workerName);
    }
    echo "dispatched $taskCount tasks\n";
};
