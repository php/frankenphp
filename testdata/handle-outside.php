<?php

try {
    new \FrankenPHP\WorkerHandle();
    echo 'no exception';
} catch (\RuntimeException $e) {
    echo $e->getMessage();
}
