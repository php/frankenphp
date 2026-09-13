<?php

foreach (['frankenphp_get_worker_handle', 'frankenphp_worker_tick'] as $function) {
    try {
        $function();
        echo "$function: no exception\n";
    } catch (\RuntimeException $e) {
        echo $e->getMessage(), "\n";
    }
}
