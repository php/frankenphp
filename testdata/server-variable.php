<?php

require_once __DIR__.'/_executor.php';

return function () {
    echo print_r($_SERVER, true);
};
