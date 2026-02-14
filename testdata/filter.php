<?php
require_once __DIR__.'/_executor.php';

return function () {
    echo strtoupper(filter_input(INPUT_SERVER, "REQUEST_METHOD", FILTER_UNSAFE_RAW, FILTER_FLAG_STRIP_LOW | FILTER_FLAG_STRIP_HIGH) ?? "");
};
