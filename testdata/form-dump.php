<?php
require_once __DIR__.'/_executor.php';

return function () {
    $post = $_POST;
    $files = $_FILES;

    if (isset($_GET['parse_body'])) {
        try {
            [$post, $files] = request_parse_body(json_decode($_GET['parse_body'], true));
        } catch (Throwable $e) {
            echo $e::class, ': ', $e->getMessage(), "\n";
        }
    }

    // tmp names are random: dump the content instead
    $read = function ($tmpName) use (&$read) {
        if (is_array($tmpName)) {
            return array_map($read, $tmpName);
        }

        return '' === $tmpName ? '' : [file_get_contents($tmpName), is_uploaded_file($tmpName)];
    };
    foreach ($files as &$file) {
        if (isset($file['tmp_name'])) {
            $file['tmp_name'] = $read($file['tmp_name']);
        }
    }

    var_export(['post' => $post, 'files' => $files, 'error' => error_get_last()['message'] ?? null]);
    error_clear_last();
};
