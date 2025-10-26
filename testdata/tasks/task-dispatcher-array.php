<?php

require_once __DIR__ . '/../_executor.php';

return function () {
    $taskCount = $_GET['count'] ?? 0;
    $workerName = $_GET['worker'] ?? '';
    for ($i = 0; $i < $taskCount; $i++) {
        frankenphp_dispatch_request([
            'task' => "array task$i",
            'worker' => $workerName,
            'index' => $i,
        ]);
    }
    echo "dispatched $taskCount tasks\n";
};
