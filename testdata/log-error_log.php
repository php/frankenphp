<?php

require_once __DIR__.'/_executor.php';

return function () {
    error_log("request {$_GET['i']}");
};
