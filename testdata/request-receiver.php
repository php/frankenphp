<?php

while (frankenphp_handle_request(function ($message) {
    echo $message;
})) {
    // keep handling requests
}
