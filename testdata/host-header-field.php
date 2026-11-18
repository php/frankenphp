<?php

require_once __DIR__.'/_executor.php';

return function () {
    echo "HTTP_HOST: {$_SERVER['HTTP_HOST']}, SERVER_NAME: {$_SERVER['SERVER_NAME']}, Host: ".apache_request_headers()['Host'];
};
