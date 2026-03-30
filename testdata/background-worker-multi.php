<?php

frankenphp_handle_request(function () {
    try {
        $all = frankenphp_get_vars(['worker-a', 'worker-b'], 5.0);
        ksort($all);
        $parts = [];
        foreach ($all as $name => $vars) {
            foreach ($vars as $k => $v) {
                $parts[] = "$name:$k=$v";
            }
        }
        echo implode(',', $parts);
    } catch (\Throwable $e) {
        echo 'ERROR:' . $e->getMessage();
    }
});
