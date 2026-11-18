<?php

require_once __DIR__.'/_executor.php';

return function () {
    if (isset($_GET['hard_limit'])) {
        var_export(ini_get('memory_hard_limit_percent') !== false);

        return;
    }

    if (isset($_GET['exhaust'])) {
        $a = [];
        while (true) {
            $a[] = str_repeat('x', 1 << 20);
        }
    }

    echo 'ok';
};
