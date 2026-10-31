<?php

$handler = static function () {
    echo 'read=' . strlen(file_get_contents('php://input'));
};

while (frankenphp_handle_request($handler)) {
}
