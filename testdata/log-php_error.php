<?php

require_once __DIR__.'/_executor.php';

return function () {
    trigger_error("request {$_GET['i']}", E_USER_WARNING);
};
