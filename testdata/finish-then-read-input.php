<?php

// Finish the request (sends the response, closes the Go-side context) and only
// then read the request body. This exercises go_read_post after the HTTP
// responseWriter has been finalized. See php/frankenphp#2535.
frankenphp_finish_request();

// Give the Go handler goroutine time to return and finalize the HTTP/2
// responseWriter before we touch the request body.
usleep(200000);

file_get_contents('php://input');
