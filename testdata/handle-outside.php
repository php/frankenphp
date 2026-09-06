<?php

try {
    frankenphp_get_worker_handle();
    echo 'no exception';
} catch (\RuntimeException $e) {
    echo $e->getMessage();
}
