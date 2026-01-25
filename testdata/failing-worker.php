<?php

if (rand(1, 100) <= 50) {
    throw new Exception('this exception is expected to fail the worker');
}

// frankenphp_handle_request() has not been reached (also a failure)
