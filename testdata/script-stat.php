<?php

require_once __DIR__.'/_executor.php';

return function () {
    echo getmyinode() === fileinode(__FILE__) ? 'script' : 'not the script';
    echo ' ', $_SERVER['PATH_TRANSLATED'] ?? 'unset';
};
