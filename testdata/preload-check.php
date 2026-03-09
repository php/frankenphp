<?php

require_once __DIR__.'/_executor.php';

return function () {
    if (function_exists('preloaded_function')) {
        echo preloaded_function();
    } else {
        echo 'not preloaded';
    }    
};

