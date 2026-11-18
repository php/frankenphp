<?php

while (frankenphp_handle_request(function ($message) {
    return var_export($message, true);
})) {
    // continue handling requests
}
