<?php

try {
    frankenphp_set_vars(['a' => 1]);
    echo 'no exception';
} catch (\Throwable $e) {
    echo get_class($e) . ': ' . $e->getMessage();
}
