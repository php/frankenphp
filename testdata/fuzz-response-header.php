<?php

// Exercises add_response_header (frankenphp.c) through FrankenPHP's own
// frankenphp_response_headers(), not php-src's header() validation itself.
// The header line is base64-encoded in the query string so arbitrary bytes
// reach header() unmangled by HTTP transport.

$raw = base64_decode($_GET['h'] ?? '', true);
if ($raw !== false && $raw !== '') {
    // Silence header()'s own warnings (e.g. embedded CR/LF); we only care
    // about what add_response_header does with whatever it accepts.
    @header($raw);
}

// Header values may legitimately contain non-UTF-8 bytes; substitute
// instead of letting json_encode() fail (and echo nothing) on those.
echo json_encode(frankenphp_response_headers(), JSON_INVALID_UTF8_SUBSTITUTE);
