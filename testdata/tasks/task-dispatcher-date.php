<?php

require_once __DIR__ . '/../_executor.php';

return function () {
    $date = new DateTime('2024-01-01 12:00:00', new DateTimeZone('Europe/Vienna'));
    frankenphp_dispatch_task($date);

    echo "dispatched tasks\n";
};
