<?php

require_once __DIR__.'/../_executor.php';

return function () {
    $taskCount = $_GET['count'] ?? 0;
    $workerName = $_GET['worker'] ?? '';
    for ($i = 0; $i < $taskCount; $i++) {
        $c = new DateTime();
        $c->setTimestamp(time()+123);
        $c->setTimezone(new Datetimezone('America/New_York'));
        #$c = serialize($c);
        frankenphp_dispatch_task($c, $workerName);
    }
    echo "dispatched $taskCount tasks\n";
};
