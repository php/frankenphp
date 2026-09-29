<?php

echo json_encode([
    'host' => $_SERVER['HTTP_HOST'],
    'tag' => $_SERVER['HTTP_X_TAG'],
    'uri' => $_SERVER['REQUEST_URI'],
    'body' => file_get_contents('php://input'),
]);
