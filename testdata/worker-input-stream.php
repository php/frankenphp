<?php

$kept = null;

$handler = static function () use (&$kept) {
    if (isset($_GET['close'])) {
        echo file_get_contents('php://input');
        foreach (get_resources('stream') as $stream) {
            if (get_resource_type($stream) === 'stream' && stream_get_meta_data($stream)['stream_type'] === 'TEMP') {
                fclose($stream);
            }
        }

        return;
    }

    if ($kept === null) {
        $kept = fopen('php://input', 'r');
        echo stream_get_contents($kept);

        return;
    }

    rewind($kept);
    echo stream_get_contents($kept);
};

while (frankenphp_handle_request($handler)) {
}
