<?php

require_once __DIR__.'/_executor.php';

return function () {
    echo 'exiting';
    exit((int) $_GET['status']);
};
