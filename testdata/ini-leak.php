<?php

require_once __DIR__.'/_executor.php';

return function () {
    $action = $_GET['action'] ?? 'default';
    $output = [];

    switch ($action) {
        case 'change_ini':
            // Change several INI values at runtime
            $before = ini_get('display_errors');
            ini_set('display_errors', '0');
            $after = ini_get('display_errors');
            $output[] = "display_errors: before=$before, after=$after";

            $before = ini_get('max_execution_time');
            ini_set('max_execution_time', '999');
            $after = ini_get('max_execution_time');
            $output[] = "max_execution_time: before=$before, after=$after";

            $before = ini_get('precision');
            ini_set('precision', '10');
            $after = ini_get('precision');
            $output[] = "precision: before=$before, after=$after";

            $output[] = "INI_CHANGED";
            break;

        case 'check_ini':
            // Check if INI values from previous request leaked
            $display_errors = ini_get('display_errors');
            $max_execution_time = ini_get('max_execution_time');
            $precision = ini_get('precision');

            $output[] = "display_errors=$display_errors";
            $output[] = "max_execution_time=$max_execution_time";
            $output[] = "precision=$precision";

            // Check for leaks (values set in previous request)
            $leaks = [];
            if ($display_errors === '0') {
                $leaks[] = "display_errors leaked (expected default, got 0)";
            }
            if ($max_execution_time === '999') {
                $leaks[] = "max_execution_time leaked (expected default, got 999)";
            }
            if ($precision === '10') {
                $leaks[] = "precision leaked (expected default, got 10)";
            }

            if (empty($leaks)) {
                $output[] = "NO_LEAKS";
            } else {
                $output[] = "LEAKS_DETECTED";
                foreach ($leaks as $leak) {
                    $output[] = "LEAK: $leak";
                }
            }
            break;

        default:
            $output[] = "UNKNOWN_ACTION";
    }

    echo implode("\n", $output);
};
