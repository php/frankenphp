<?php

frankenphp_log("some debug message", -4, [
    "key int"    => 1,
]);

frankenphp_log("some info message", 0, [
    "key string" => "string",
]);

frankenphp_log("some warn message", 4);

frankenphp_log("some error message", 8, [
    "err" => ["a", "v"],
]);
