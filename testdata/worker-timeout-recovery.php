<?php

require_once __DIR__.'/_executor.php';

return function () {
    $action = $_GET['action'] ?? 'normal';

    switch ($action) {
        case 'timeout':
            echo 'BEFORE_TIMEOUT';
            set_time_limit(1);
            while (true) {
                // Infinite loop: will be killed by max_execution_time
            }
            break;

        case 'ping':
            echo 'pong';
            break;
    }
};
