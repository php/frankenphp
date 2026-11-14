<?php

try {
    new \FrankenPHP\WorkerHandle();
    echo 'no exception';
} catch (\LogicException $e) {
    echo $e->getMessage();
}
