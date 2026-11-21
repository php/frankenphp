<?php

$between = null;

while (frankenphp_handle_request(static function () use (&$between): void {
    if (isset($_GET['between'])) {
        $between = $_GET['between'];
    }

    if (isset($_GET['memory'])) {
        echo memory_get_usage();
    }

    if (isset($_GET['temp_streams'])) {
        echo count(array_filter(get_resources('stream'), static fn ($s): bool => stream_get_meta_data($s)['stream_type'] === 'TEMP'));
    }

    if (isset($_GET['rewrite'])) {
        output_add_rewrite_var('current', 'request');
        echo '<form action="/next"></form>';
    }

    if (isset($_GET['session'])) {
        session_start(['read_and_close' => true]);
        echo session_id();
    }
})) {
    // code running between two requests, like a framework's terminate()
    match ($between) {
        'header' => header('X-Between: ' . str_repeat('a', 4 << 20)),
        'status_line' => header('HTTP/1.1 200 ' . str_repeat('a', 4 << 20)),
        'content_type' => header('Content-Type: text/' . str_repeat('a', 4 << 20)),
        'input' => fclose(fopen('php://input', 'r')),
        'rewrite' => output_add_rewrite_var('between', 'requests'),
        'session' => session_id('between'),
        null => null,
    };
    $between = null;
}
