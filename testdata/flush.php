<?php

require_once __DIR__.'/_executor.php';

return function () {
    if (ini_get("output_buffering") !== "0") {
        // Disable output buffering if not already done
        while (@ob_end_flush());
    }

    echo 'He';

    flush();

    echo 'llo '.($_GET['i'] ?? '');
};
