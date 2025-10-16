<?php

while (frankenphp_handle_request(function ($message) {
    echo $message;
    return "received message: $message";
})) {
    // continue handling requests
}
